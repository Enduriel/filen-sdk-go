// Package crypto provides the cryptographic functions required within the SDK.
//
// There are two kinds of decrypted data:
//   - Metadata means any small string data, typically file metadata, but also e.g. directory names.
//   - Data means file content.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
	"slices"
	"strings"
)

// EncryptedString denotes that a string is encrypted and can't be used meaningfully before being decrypted.
type EncryptedString string

func NewEncryptedStringV2(encrypted []byte, nonce [12]byte) EncryptedString {
	return EncryptedString("002" + string(nonce[:]) + base64.StdEncoding.EncodeToString(encrypted))
}

func NewEncryptedStringV3(encrypted []byte, nonce [12]byte) EncryptedString {
	return EncryptedString("003" + hex.EncodeToString(nonce[:]) + base64.StdEncoding.EncodeToString(encrypted))
}

// other

// v1 and v2
type MasterKeys []MasterKey

func NewMasterKeys(encryptionKey *MasterKey, stringKeys string) (MasterKeys, error) {
	keys := make([]MasterKey, 0)
	for _, key := range strings.Split(stringKeys, "|") {
		if len(key) != 64 {
			return nil, fmt.Errorf("key length wrong %s")
		}
		keyBytes := []byte(key)
		keySized := [64]byte(keyBytes)
		mk, err := NewMasterKey(keySized)
		if err != nil {
			return nil, fmt.Errorf("NewMasterKey: %w", err)
		}
		if encryptionKey != nil && encryptionKey.DerivedBytes == mk.DerivedBytes {
			continue
		}
		keys = append(keys, *mk)
	}
	if encryptionKey != nil {
		keys = slices.Insert(keys, 0, *encryptionKey)
	}
	return keys, nil
}

func getCipherForKey(key [32]byte) (cipher.AEAD, error) {
	c, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("getCipherForKey: %v", err)
	}
	derivedGcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("getCipherForKey: %v", err)
	}
	return derivedGcm, nil
}

type MasterKey struct {
	Bytes        [64]byte
	DerivedBytes [32]byte
	cipher       cipher.AEAD
}

func NewMasterKey(key [64]byte) (*MasterKey, error) {
	derivedKey := pbkdf2.Key(key[:], key[:], 1, 32, sha512.New)
	derivedBytes := [32]byte{}
	copy(derivedBytes[:], derivedKey[:32])
	c, err := getCipherForKey(derivedBytes)
	if err != nil {
		return nil, fmt.Errorf("NewMasterKey: %v", err)
	}
	return &MasterKey{
		Bytes:        key,
		DerivedBytes: derivedBytes,
		cipher:       c,
	}, nil
}

func (m *MasterKey) EncryptMeta(metadata string) EncryptedString {
	nonce := [12]byte([]byte(GenerateRandomString(12)))
	encrypted := m.cipher.Seal(nil, nonce[:], []byte(metadata), nil)
	return NewEncryptedStringV2(encrypted, nonce)
}

func (m *MasterKey) DecryptMetaV1(metadata EncryptedString) (string, error) {
	panic("unimplemented")
}

func (m *MasterKey) DecryptMetaV2(metadata EncryptedString) (string, error) {
	nonce := metadata[3:15]
	decoded, err := base64.StdEncoding.DecodeString(string(metadata[15:]))
	if err != nil {
		return "", fmt.Errorf("DecryptMetadataV2: %v", err)
	}
	decoded, err = m.cipher.Open(decoded[:0], []byte(nonce), decoded, nil)
	if err != nil {
		return "", fmt.Errorf("DecryptMetadataV2: %v", err)
	}
	return string(decoded), nil
}

func (m *MasterKey) DecryptMeta(metadata EncryptedString) (string, error) {
	if metadata[0:8] == "U2FsdGVk" {
		return m.DecryptMetaV1(metadata)
	}
	switch metadata[0:3] {
	case "002":
		return m.DecryptMetaV2(metadata)
	default:
		return "", fmt.Errorf("unknown metadata format")
	}
}

// AllKeysFailedError denotes that no key passed to [DecryptMetadataAllKeys] worked.
type AllKeysFailedError struct {
	Errors []error // errors thrown in the process
}

func (e *AllKeysFailedError) Error() string {
	return fmt.Sprintf("all keys failed: %v", e.Errors)
}

func (ms *MasterKeys) decryptMeta(metadata EncryptedString, decryptFunc func(m *MasterKey, encryptedString EncryptedString) (string, error)) (string, error) {
	errs := make([]error, 0)
	for _, masterKey := range *ms {
		var decrypted string
		decrypted, err := decryptFunc(&masterKey, metadata)
		if err == nil {
			return decrypted, nil
		}
		errs = append(errs, err)
	}
	return "", &AllKeysFailedError{Errors: errs}
}

func (ms *MasterKeys) DecryptMeta(metadata EncryptedString) (string, error) {
	if metadata[0:8] == "U2FsdGVk" {
		return ms.decryptMeta(metadata, (*MasterKey).DecryptMetaV1)
	}
	switch metadata[0:3] {
	case "002":
		return ms.decryptMeta(metadata, (*MasterKey).DecryptMetaV2)
	default:
		return "", fmt.Errorf("unknown metadata format")
	}
}

func (ms *MasterKeys) EncryptMeta(metadata string) EncryptedString {
	// potential null dereference which makes me uncomfortable
	// this function should only ever be called on non-empty MasterKeys
	// which should be safe since in v2 there must be at least 1 master key,
	// and in v3 we won't be using this function
	return (*ms)[0].EncryptMeta(metadata)
}

type DerivedPassword string

func DeriveMKAndAuthFromPassword(password string, salt string) (*MasterKey, DerivedPassword, error) {
	// makes a 128 byte string
	derived := hex.EncodeToString(pbkdf2.Key([]byte(password), []byte(salt), 200000, 64, sha512.New))
	var (
		rawMasterKey [64]byte
	)
	copy(rawMasterKey[:], derived[:64])

	hasher := sha512.New()
	hasher.Write([]byte(derived[64:])) // write password
	derivedPass := DerivedPassword(hex.EncodeToString(hasher.Sum(nil)))

	masterKey, err := NewMasterKey(rawMasterKey)
	if err != nil {
		return nil, "", fmt.Errorf("NewMasterKey: %v\n", err)
	}
	return masterKey, derivedPass, nil
}

// v3

type DataEncryptionKey struct {
	Bytes  [32]byte
	cipher cipher.AEAD
}

func NewDataEncryptionKey(key [32]byte) (*DataEncryptionKey, error) {
	c, err := getCipherForKey(key)
	if err != nil {
		return nil, fmt.Errorf("NewDataEncryptionKey: %v", err)
	}
	return &DataEncryptionKey{
		Bytes:  key,
		cipher: c,
	}, nil
}

func DEKFromDecryptedString(decrypted string) (*DataEncryptionKey, error) {
	decoded, err := hex.DecodeString(decrypted)
	if err != nil {
		return nil, fmt.Errorf("decoding DEK: %w", err)
	}
	dek, err := NewDataEncryptionKey([32]byte(decoded))
	if err != nil {
		return nil, fmt.Errorf("initializing DEK: %w", err)
	}
	return dek, nil
}

func (dek *DataEncryptionKey) ToString() string {
	return hex.EncodeToString(dek.Bytes[:])
}

func (dek *DataEncryptionKey) EncryptMeta(metadata string) EncryptedString {
	panic("unimplemented")
}

func (dek *DataEncryptionKey) DecryptMeta(metadata EncryptedString) (string, error) {
	panic("unimplemented")
}

type KeyEncryptionKey struct {
	Bytes  [32]byte
	cipher cipher.AEAD
}

func NewKeyEncryptionKey(key [32]byte) (*KeyEncryptionKey, error) {
	c, err := getCipherForKey(key)
	if err != nil {
		return nil, fmt.Errorf("NewKeyEncryptionKey: %v", err)
	}
	return &KeyEncryptionKey{
		Bytes:  key,
		cipher: c,
	}, nil
}

func (kek *KeyEncryptionKey) EncryptMeta(metadata string) EncryptedString {
	nonce := [12]byte(GenerateRandomBytes(12))
	encrypted := kek.cipher.Seal(nil, nonce[:], []byte(metadata), nil)
	return NewEncryptedStringV3(encrypted, nonce)
}

func (kek *KeyEncryptionKey) DecryptMeta(metadata EncryptedString) (string, error) {
	if metadata[0:3] != "003" {
		return "", fmt.Errorf("unknown metadata format")
	}
	nonce, err := hex.DecodeString(string(metadata[3:27]))
	if err != nil {
		return "", fmt.Errorf("decoding nonce: %v", err)
	}
	decrypted, err := kek.cipher.Open(nil, nonce[:], []byte(metadata[27:]), nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %v", err)
	}
	return string(decrypted), nil
}

func DeriveKEKAndAuthFromPassword(password string, salt string) (*KeyEncryptionKey, DerivedPassword, error) {
	derived := argon2.IDKey([]byte(password), []byte(salt), 3, 65536, 4, 64)
	var rawKEK [32]byte
	copy(rawKEK[:], derived[:32])

	kek, err := NewKeyEncryptionKey(rawKEK)
	if err != nil {
		return nil, "", fmt.Errorf("NewKeyEncryptionKey: %v", err)
	}
	return kek, DerivedPassword(hex.EncodeToString(derived[32:])), nil
}

// file
type FileKey struct {
	Bytes  [32]byte
	cipher cipher.AEAD
}

func NewFileKey(key [32]byte) (*FileKey, error) {
	c, err := getCipherForKey(key)
	if err != nil {
		return nil, fmt.Errorf("NewFileKey: %v", err)
	}

	return &FileKey{
		Bytes:  key,
		cipher: c,
	}, nil
}

func NewFileKeyFromStr(key string) (*FileKey, error) {
	switch len(key) {
	case 32: // v1 & v2
		return NewFileKey([32]byte([]byte(key)))
	case 64: // v3
		decoded, err := hex.DecodeString(key)
		if err != nil {
			return nil, fmt.Errorf("decoding file key: %v", err)
		}
		return NewFileKey([32]byte(decoded))
	default:
		return nil, fmt.Errorf("key length wrong")
	}
}

func (fk *FileKey) encrypt(nonce []byte, data []byte) []byte {
	return fk.cipher.Seal(data[:0], nonce, data, nil)
}

func (fk *FileKey) EncryptData(data []byte) []byte {
	nonce := GenerateRandomBytes(12)
	data = fk.encrypt(nonce[:], data)
	return append(nonce, data...)
}

func (fk *FileKey) decrypt(nonce []byte, data []byte) error {
	data, err := fk.cipher.Open(data[:0], nonce, data, nil)
	if err != nil {
		return fmt.Errorf("open: %v", err)
	}
	return nil
}

func (fk *FileKey) DecryptData(data []byte) ([]byte, error) {
	nonce := data[:12]
	err := fk.decrypt(nonce, data[12:])
	if err != nil {
		return nil, err
	}
	return data[12:], nil
}

func (fk *FileKey) ToString(authVersion int) string {
	if authVersion == 3 {
		return hex.EncodeToString(fk.Bytes[:])
	}
	return string(fk.Bytes[:])
}
