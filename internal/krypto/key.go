package krypto

import (
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	keyLen = 32

	// SecretMarker is a string we can look for in logs to see if the app
	// is accidentally exposing secrets.
	SecretMarker = "<!SECRET_REDACTED!>"
)

var (
	ErrInvalidKey = errors.New("invalid key")
)

type Key struct {
	value []byte
}

// ParseKey expects a hex encoded key of 32 bytes (64 bytes as hex).
func ParseKey(raw string) (Key, error) {
	if len(raw) != keyLen*2 {
		return Key{}, ErrInvalidKey
	}

	k := make([]byte, keyLen)
	_, err := hex.Decode(k, []byte(raw))
	if err != nil {
		return Key{}, ErrInvalidKey
	}

	return Key{
		value: k,
	}, nil
}

func (k Key) Format(f fmt.State, verb rune) {
	f.Write([]byte(SecretMarker))
}

func (k Key) MarshalText() ([]byte, error) {
	return []byte(SecretMarker), nil
}

// SecretValue returns the key as a byte slice. This is provided
// as an escape hatch for cases where the key needs to be provided
// to third party packages or libraries.
func (k Key) SecretValue() []byte {
	return k.value
}
