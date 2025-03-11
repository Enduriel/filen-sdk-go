// Package filen provides an SDK interface to interact with the cloud drive.
package filen

import (
	"context"
	"crypto/rsa"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
)

// Filen provides the SDK interface. Needs to be initialized via [New].
type Filen struct {
	Client      *client.Client
	AuthVersion int

	Email string

	// MasterKeys contains the crypto master keys for the current user. When the user changes
	// their password, a new master key is appended. For decryption, all master keys are tried
	// until one works; for encryption, always use the latest master key.
	MasterKeys crypto.MasterKeys
	DEK        crypto.EncryptionKey
	KEK        crypto.EncryptionKey

	PrivateKey rsa.PrivateKey
	PublicKey  rsa.PublicKey

	// BaseFolderUUID is the UUID of the cloud drive's root directory
	BaseFolder types.RootDirectory
}

// New creates a new Filen and initializes it with the given email and password
// by logging in with the API and preparing the API key and master keys.
func New(ctx context.Context, email, password string) (*Filen, error) {
	unauthorizedClient := client.New(ctx)

	// fetch salt
	authInfo, err := unauthorizedClient.PostV3AuthInfo(ctx, email)
	if err != nil {
		return nil, err
	}

	switch authInfo.AuthVersion {
	case 1:
		panic("unimplemented")
	case 2:
		return newV2(ctx, email, password, *authInfo, unauthorizedClient)
	case 3:
		return newV3(ctx, email, password, *authInfo, unauthorizedClient)
	default:
		panic("unimplemented")
	}
}

// NewWithAPIKey creates a new Filen and initializes it with the given email, password, and API key
func NewWithAPIKey(ctx context.Context, email, password, apiKey string) (*Filen, error) {
	c := client.NewWithAPIKey(ctx, apiKey)

	authInfo, err := c.PostV3AuthInfo(ctx, email)
	if err != nil {
		return nil, err
	}

	switch authInfo.AuthVersion {
	case 1:
		panic("unimplemented")
	case 2:
		return newV2WithAPIKey(ctx, email, password, *authInfo, c)
	case 3:
		return newV3WithAPIKey(ctx, email, password, *authInfo, c)
	default:
		panic("unimplemented")
	}
}

func getKeyPair(ctx context.Context, metaCrypter crypto.MetaCrypter, c *client.Client) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	response, err := c.GetV3UserKeyPairInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get keypair info: %w", err)
	}
	privateKeyStr, err := metaCrypter.DecryptMeta(response.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	privateKey, publicKey, err := crypto.RSAKeyPairFromStrings(privateKeyStr, response.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse rsa keys: %w", err)
	}
	return privateKey, publicKey, nil
}

func getMasterKeys(ctx context.Context, masterKey crypto.MasterKey, c *client.Client) (crypto.MasterKeys, error) {
	encryptedMasterKey := masterKey.EncryptMeta(string(masterKey.Bytes[:]))
	mkResponse, err := c.PostV3UserMasterKeys(ctx, encryptedMasterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get master keys: %w", err)
	}
	masterKeysStr, err := masterKey.DecryptMetaV2(mkResponse.Keys)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt master keys meta: %w", err)
	}

	masterKeys, err := crypto.NewMasterKeys(masterKey, masterKeysStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse master keys: %w", err)
	}

	return masterKeys, nil
}

func getDEK(ctx context.Context, kek crypto.EncryptionKey, c *client.Client) (*crypto.EncryptionKey, error) {
	encryptedDEK, err := c.GetV3UserDEK(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get DEK: %w", err)
	}
	decryptedDEKStr, err := kek.DecryptMeta(encryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt DEK: %w", err)
	}
	dek, err := crypto.MakeEncryptionKeyFromStr(decryptedDEKStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DEK: %w", err)
	}
	return dek, nil
}

func loginV2(ctx context.Context, email, password string, info client.V3AuthInfoResponse, uc *client.UnauthorizedClient) (*client.Client, *crypto.MasterKey, error) {
	masterKey, derivedPass, err := crypto.DeriveMKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, nil, fmt.Errorf("DeriveMKAndAuthFromPassword: %w", err)
	}
	// for simplicity, I'm going to ignore the fact that response here contains the RSAKeypair
	response, err := uc.PostV3Login(ctx, email, derivedPass, info.AuthVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to log in: %w", err)
	}
	c := uc.Authorize(response.APIKey)
	return c, masterKey, nil
}

func loginV3(ctx context.Context, email, password string, info client.V3AuthInfoResponse, uc *client.UnauthorizedClient) (*client.Client, *crypto.EncryptionKey, error) {
	kek, derivedPass, err := crypto.DeriveKEKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, nil, fmt.Errorf("DeriveKEKAndAuthFromPassword: %w", err)
	}
	// for simplicity, I'm going to ignore the fact that response here contains the RSAKeypair
	response, err := uc.PostV3Login(ctx, email, derivedPass, info.AuthVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to log in: %w", err)
	}
	c := uc.Authorize(response.APIKey)

	return c, kek, nil
}

func newV2Authed(ctx context.Context, email string, info client.V3AuthInfoResponse, c *client.Client, masterKey crypto.MasterKey) (*Filen, error) {
	masterKeys, err := getMasterKeys(ctx, masterKey, c)
	if err != nil {
		return nil, fmt.Errorf("getMasterKeys: %w", err)
	}

	privateKey, publicKey, err := getKeyPair(ctx, &masterKeys, c)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt rsa keys: %w", err)
	}

	baseFolderResponse, err := c.GetV3UserBaseFolder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get base folder: %w", err)
	}

	return &Filen{
		Client:      c,
		Email:       email,
		MasterKeys:  masterKeys,
		PrivateKey:  *privateKey,
		PublicKey:   *publicKey,
		BaseFolder:  types.NewRootDirectory(baseFolderResponse.UUID),
		AuthVersion: info.AuthVersion,
	}, nil
}

func newV2(ctx context.Context, email, password string, info client.V3AuthInfoResponse, uc *client.UnauthorizedClient) (*Filen, error) {
	c, masterKey, err := loginV2(ctx, email, password, info, uc)
	if err != nil {
		return nil, fmt.Errorf("loginV2: %w", err)
	}

	return newV2Authed(ctx, email, info, c, *masterKey)
}

func newV2WithAPIKey(ctx context.Context, email, password string, info client.V3AuthInfoResponse, c *client.Client) (*Filen, error) {
	masterKey, _, err := crypto.DeriveMKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, fmt.Errorf("DeriveMKAndAuthFromPassword: %w", err)
	}

	return newV2Authed(ctx, email, info, c, *masterKey)
}

func newV3Authed(ctx context.Context, email string, info client.V3AuthInfoResponse, c *client.Client, kek crypto.EncryptionKey) (*Filen, error) {
	dek, err := getDEK(ctx, kek, c)
	if err != nil {
		return nil, fmt.Errorf("getDEK: %w", err)
	}

	privateKey, publicKey, err := getKeyPair(ctx, dek, c)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt rsa keys: %w", err)
	}

	baseFolderResponse, err := c.GetV3UserBaseFolder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get base folder: %w", err)
	}
	return &Filen{
		Client:      c,
		Email:       email,
		MasterKeys:  make(crypto.MasterKeys, 0),
		KEK:         kek,
		DEK:         *dek,
		PrivateKey:  *privateKey,
		PublicKey:   *publicKey,
		BaseFolder:  types.NewRootDirectory(baseFolderResponse.UUID),
		AuthVersion: info.AuthVersion,
	}, nil
}

func newV3(ctx context.Context, email, password string, info client.V3AuthInfoResponse, uc *client.UnauthorizedClient) (*Filen, error) {
	c, kek, err := loginV3(ctx, email, password, info, uc)
	if err != nil {
		return nil, fmt.Errorf("loginV3: %w", err)
	}

	return newV3Authed(ctx, email, info, c, *kek)
}

func newV3WithAPIKey(ctx context.Context, email, password string, info client.V3AuthInfoResponse, c *client.Client) (*Filen, error) {
	kek, _, err := crypto.DeriveKEKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, fmt.Errorf("DeriveKEKAndAuthFromPassword: %w", err)
	}

	return newV3Authed(ctx, email, info, c, *kek)
}
