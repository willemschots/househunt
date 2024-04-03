package krypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
)

var (
	// ErrUnknownKey indicates that the key used to encrypt the data is unknown.
	ErrUnknownKey = errors.New("unknown key")
	// ErrInvalidData indicates that the data is invalid.
	ErrInvalidData = errors.New("invalid data")
)

const indexBytes = 4

// Encryptor encrypts and decrypts data using AES-GCM.
//
// The encryptor uses an append only list of keys for encryption and decryption.
// The last key in the list is considered the latest key.
//
// To construct output data, the encryptor prefixes the encrypted data with the index
// of the used key. This allows the encryptor to work with multiple keys and to decrypt
// data encrypted with an older key.
//
// The index used is not considered secret.
type Encryptor struct {
	keys []Key
}

// NewEncryptor creates a new encryptor with the provided keys.
func NewEncryptor(keys []Key) (*Encryptor, error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one key is required")
	}

	return &Encryptor{
		keys: keys,
	}, nil
}

// Encrypt encrypts the data using the latest available key.
// It returns the encrypted data prefixed with the key identifier.
func (s *Encryptor) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidData
	}

	index := len(s.keys) - 1
	block, err := aes.NewCipher(s.keys[index].value)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, err := randBytes(gcm.NonceSize())
	if err != nil {
		return nil, err
	}

	buf := make([]byte, indexBytes)
	binary.BigEndian.PutUint32(buf, uint32(index))

	result := gcm.Seal(nil, nonce, data, buf)
	buf = append(buf, nonce...)
	buf = append(buf, result...)
	return buf, nil
}

// Decrypt decrypts the data using the key identified by the first 4 bytes in the data.
// It returns the decrypted data or an error.
func (s *Encryptor) Decrypt(message []byte) ([]byte, error) {
	if len(message) < indexBytes {
		return nil, ErrInvalidData
	}

	index := binary.BigEndian.Uint32(message[:indexBytes])
	if int(index) >= len(s.keys) {
		return nil, ErrUnknownKey
	}

	block, err := aes.NewCipher(s.keys[index].value)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	minLen := indexBytes + nonceSize
	if len(message) <= minLen {
		return nil, ErrInvalidData
	}

	nonce := message[indexBytes:minLen]
	ciphertext := message[minLen:]

	return gcm.Open(nil, nonce, ciphertext, message[:4])
}

func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}
