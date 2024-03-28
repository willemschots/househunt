package auth

import (
	"fmt"
)

const (
	saltLen = 16
	keyLen  = 32

	minPasswordBytes = 8
	// We put a generous upper cap on password length, so people can use
	// passphrases but we don't allow MBs of data as a password.
	maxPasswordBytes = 512

	// SecretMarker is a string we can look for in logs to see if the app
	// is accidentally exposing secrets.
	SecretMarker = "<!SECRET_REDACTED!>"
)

var ErrInvalidPassword = fmt.Errorf("invalid password")

// Password is a plaintext password.
//
// It should never be persisted, logged or exposed in any other way. To
// protect ourselves from accidentally doing so, the type implements
// several common interfaces that would allow it to be used inappropriately.
//
// There are only two operations allowed on a Password:
// - Converting it to a hash.
// - Comparing it with an existing hash to see if they match.
type Password struct {
	plain []byte
}

// ParsePassword creates a new Password from a plaintext string.
// It errors if the password is too short or too long.
func ParsePassword(pwd string) (Password, error) {
	if len(pwd) < minPasswordBytes || len(pwd) > maxPasswordBytes {
		return Password{}, ErrInvalidPassword
	}

	return Password{
		plain: []byte(pwd),
	}, nil
}

// Match checks if the plaintext password matches the given hash.
func (p Password) Match(h Argon2Hash) bool {
	return matchHash(h, p.plain)
}

// Hash hashes the plaintext password using the argon2id algorithm.
func (p Password) Hash() (Argon2Hash, error) {
	return hashBytes(p.plain)
}

func (p Password) Format(f fmt.State, verb rune) {
	f.Write([]byte(SecretMarker))
}

func (p Password) MarshalText() ([]byte, error) {
	return []byte(SecretMarker), nil
}
