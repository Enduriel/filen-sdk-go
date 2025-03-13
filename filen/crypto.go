package filen

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"strings"
)

func (api *Filen) HashFileName(name string) string {
	name = strings.ToLower(name)
	switch api.AuthVersion {
	case 1, 2:
		outerHasher := sha1.New()
		innerHasher := sha512.New()
		innerHasher.Write([]byte(name))
		outerHasher.Write([]byte(hex.EncodeToString(innerHasher.Sum(nil))))
		return hex.EncodeToString(outerHasher.Sum(nil))
	default:
		return api.HMACKey.Hash([]byte(name))
	}
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
