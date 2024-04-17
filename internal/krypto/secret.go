package krypto

import "fmt"

// SecretMarker is arbitrary sensitive data that needs to be passed
// around but not exposed. Things like API keys or other credentials.
type Secret struct {
	value []byte
}

// NewSecret creates a new secret.
func NewSecret(raw string) Secret {
	return Secret{
		value: []byte(raw),
	}
}

func (k Secret) Format(f fmt.State, verb rune) {
	f.Write([]byte(SecretMarker))
}

func (k Secret) MarshalText() ([]byte, error) {
	return []byte(SecretMarker), nil
}

// SecretValue returns the secret as a byte slice. This is provided
// as an escape hatch for cases where the key needs to be provided
// to third party packages or libraries.
func (k Secret) SecretValue() []byte {
	return k.value
}
