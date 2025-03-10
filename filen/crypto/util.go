package crypto

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"math/big"
)

func RunSHA521(b []byte) []byte {
	hasher := sha512.New()
	hasher.Write(b)
	return hasher.Sum(nil)
}

// GenerateRandomString generates a cryptographically secure random string based on a selection of alphanumerical characters.
func GenerateRandomString(length int) string {
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	str := ""
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		if err != nil {
			panic(err)
		}
		str += string(runes[idx.Int64()])
	}
	return str
}

func GenerateRandomBytes(length int) []byte {
	b := make([]byte, length)
	// rand.Read fills b with random bytes and never errors according to doc
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Simplified EVP_BytesToKey implementation
func deriveKeyAndIV(key, salt []byte, keyLen, ivLen int) ([]byte, []byte) {
	keyAndIV := make([]byte, keyLen+ivLen)

	data := make([]byte, 0, 16+len(key))
	for offset := 0; offset < keyLen+ivLen; {
		hash := md5.New()
		hash.Write(data)
		hash.Write(key)
		hash.Write(salt)
		digest := hash.Sum(nil)

		copyLen := min(len(digest), keyLen+ivLen-offset)
		copy(keyAndIV[offset:], digest[:copyLen])
		offset += copyLen

		data = digest
	}

	return keyAndIV[:keyLen], keyAndIV[keyLen:]
}
