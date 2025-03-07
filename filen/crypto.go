package filen

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func (api *Filen) HashFileName(name string) string {
	name = strings.ToLower(name)
	switch api.AuthVersion {
	case 1, 2:
		outerHasher := sha1.New()
		innerHasher := sha256.New()
		innerHasher.Write([]byte(name))
		outerHasher.Write(innerHasher.Sum(nil))
		return hex.EncodeToString(outerHasher.Sum(nil))
	default:
		hasher := sha256.New()
		hasher.Write(api.DEK.Bytes[:])
		hasher.Write([]byte(name))
		return hex.EncodeToString(hasher.Sum(nil))
	}
}
