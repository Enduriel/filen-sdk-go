package filen

import (
	"context"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"io"
)

type SerializableFilen struct {
	APIKey         string
	AuthVersion    int
	Email          string
	MasterKeys     [][64]byte
	DEK            [32]byte
	KEK            [32]byte
	PrivateKey     []byte
	HMACKey        [32]byte
	BaseFolderUUID string
}

func (api *Filen) serialize() *SerializableFilen {
	masterKeys := make([][64]byte, len(api.MasterKeys))
	for i, masterKey := range api.MasterKeys {
		masterKeys[i] = masterKey.Bytes
	}
	return &SerializableFilen{
		APIKey:         api.Client.APIKey,
		AuthVersion:    api.AuthVersion,
		Email:          api.Email,
		MasterKeys:     masterKeys,
		DEK:            api.DEK.Bytes,
		KEK:            api.KEK.Bytes,
		PrivateKey:     x509.MarshalPKCS1PrivateKey(&api.PrivateKey),
		HMACKey:        api.HMACKey,
		BaseFolderUUID: api.BaseFolder.GetUUID(),
	}
}

func (s *SerializableFilen) deserialize() (*Filen, error) {
	masterKeys := make([]crypto.MasterKey, len(s.MasterKeys))
	for i, masterKey := range s.MasterKeys {
		masterKey, err := crypto.NewMasterKey(masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse master key: %w", err)
		}
		masterKeys[i] = *masterKey
	}
	var (
		dek crypto.EncryptionKey
		kek crypto.EncryptionKey
	)
	if s.AuthVersion >= 3 {
		dekPtr, err := crypto.MakeEncryptionKeyFromBytes(s.DEK)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DEK: %w", err)
		}
		dek = *dekPtr

		kekPtr, err := crypto.MakeEncryptionKeyFromBytes(s.KEK)
		if err != nil {
			return nil, fmt.Errorf("failed to parse KEK: %w", err)
		}
		kek = *kekPtr
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(s.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Filen{
		Client:      client.NewWithAPIKey(context.Background(), s.APIKey),
		AuthVersion: s.AuthVersion,
		Email:       s.Email,
		MasterKeys:  masterKeys,
		DEK:         dek,
		KEK:         kek,
		PrivateKey:  *privateKey,
		PublicKey:   privateKey.PublicKey,
		HMACKey:     s.HMACKey,
		BaseFolder:  types.NewRootDirectory(s.BaseFolderUUID),
	}, nil
}

func (api *Filen) SerializeTo(w io.Writer) error {
	s := api.serialize()
	encoder := gob.NewEncoder(w)
	return encoder.Encode(s)
}

func DeserializeFrom(r io.Reader) (*Filen, error) {
	var s SerializableFilen
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&s); err != nil {
		return nil, err
	}
	return s.deserialize()
}
