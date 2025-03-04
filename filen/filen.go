// Package filen provides an SDK interface to interact with the cloud drive.
package filen

import (
	"encoding/hex"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

// Filen provides the SDK interface. Needs to be initialized via [New].
type Filen struct {
	client      *client.Client
	AuthVersion int

	Email string

	// MasterKeys contains the crypto master keys for the current user. When the user changes
	// their password, a new master key is appended. For decryption, all master keys are tried
	// until one works; for encryption, always use the latest master key.
	MasterKeys crypto.MasterKeys
	DEK        crypto.V3EncryptionKey
	KEK        crypto.V3EncryptionKey

	// BaseFolderUUID is the UUID of the cloud drive's root directory
	BaseFolderUUID string
}

func newV2(email, password string, info *client.AuthInfo, unauthorizedClient *client.UnauthorizedClient) (*Filen, error) {
	// login
	masterKey, derivedPass, err := crypto.DeriveMKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, fmt.Errorf("DeriveMKAndAuthFromPassword: %w", err)
	}
	response, err := unauthorizedClient.Login(email, derivedPass)
	if err != nil {
		return nil, fmt.Errorf("failed to log in: %w", err)
	}
	c := unauthorizedClient.Authorize(response.APIKey)

	// master keys decryption
	encryptedMasterKey := masterKey.EncryptMeta(string(masterKey.Bytes[:]))
	mkResponse, err := c.GetUserMasterKeys(encryptedMasterKey)
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

	// set up base folder
	baseFolderResponse, err := c.GetUserBaseFolder()
	if err != nil {
		return nil, fmt.Errorf("failed to get base folder: %w", err)
	}

	return &Filen{
		client:         c,
		Email:          email,
		MasterKeys:     masterKeys,
		BaseFolderUUID: baseFolderResponse.UUID,
		AuthVersion:    info.AuthVersion,
	}, nil
}

func newV3(email, password string, info *client.AuthInfo, unauthorizedClient *client.UnauthorizedClient) (*Filen, error) {
	// a lot of this is the same above which isn't very DRY,
	// but is annoying to do nicely with interfaces

	// login
	kek, derivedPass, err := crypto.DeriveKEKAndAuthFromPassword(password, info.Salt)
	if err != nil {
		return nil, fmt.Errorf("DeriveKEKAndAuthFromPassword: %w", err)
	}
	response, err := unauthorizedClient.Login(email, derivedPass)
	if err != nil {
		return nil, fmt.Errorf("failed to log in: %w", err)
	}
	c := unauthorizedClient.Authorize(response.APIKey)

	// master keys decryption
	encryptedKEK := kek.EncryptMeta(hex.EncodeToString(kek.Bytes[:]))
	mkResponse, err := c.GetUserMasterKeys(encryptedKEK)
	if err != nil {
		return nil, fmt.Errorf("failed to get master keys using kek: %w", err)
	}
	masterKeysStr, err := kek.DecryptMeta(mkResponse.Keys)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt master keys meta using kek: %w", err)
	}

	masterKeys, err := crypto.NewMasterKeys(nil, masterKeysStr)

	if err != nil {
		return nil, fmt.Errorf("failed to parse master keys using kek: %w", err)
	}

	// dek decryption
	encryptedDEK, err := c.GetV3UserDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to get DEK: %w", err)
	}
	decryptedDEKStr, err := kek.DecryptMeta(encryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt DEK: %w", err)
	}
	dek, err := crypto.NewV3EncryptionKeyFromStr(decryptedDEKStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DEK: %w", err)
	}

	// set up base folder
	baseFolderResponse, err := c.GetUserBaseFolder()
	if err != nil {
		return nil, fmt.Errorf("failed to get base folder: %w", err)
	}

	return &Filen{
		client:         c,
		Email:          email,
		MasterKeys:     masterKeys,
		KEK:            *kek,
		DEK:            *dek,
		BaseFolderUUID: baseFolderResponse.UUID,
		AuthVersion:    info.AuthVersion,
	}, nil
}

func (api *Filen) EncryptMeta(metadata string) crypto.EncryptedString {
	switch api.AuthVersion {
	case 1:
		panic("todo")
	case 2:
		return api.MasterKeys.EncryptMeta(metadata)
	case 3:
		return api.DEK.EncryptMeta(metadata)
	default:
		panic("unsupported version")
	}
}

func (api *Filen) DecryptMeta(encrypted crypto.EncryptedString) (string, error) {
	if encrypted[0:8] == "U2FsdGVk" {
		return api.MasterKeys.DecryptMetaV1(encrypted)
	}
	switch encrypted[0:3] {
	case "002":
		return api.MasterKeys.DecryptMetaV2(encrypted)
	case "003":
		return api.DEK.DecryptMeta(encrypted)
	default:
		panic("unsupported version")
	}
}

// New creates a new Filen and initializes it with the given email and password
// by logging in with the API and preparing the API key and master keys.
func New(email, password string) (*Filen, error) {
	unauthorizedClient := client.New()

	// fetch salt
	authInfo, err := unauthorizedClient.GetAuthInfo(email)
	if err != nil {
		return nil, err
	}

	switch authInfo.AuthVersion {
	case 1:
		panic("unimplemented")
	case 2:
		return newV2(email, password, authInfo, unauthorizedClient)
		// todo implement v3 upgrade?
	case 3:
		return newV3(email, password, authInfo, unauthorizedClient)
	default:
		panic("unimplemented")
	}

}
