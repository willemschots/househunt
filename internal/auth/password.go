package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen = 16
	keyLen  = 32

	minPasswordBytes = 8
	// We put a generous upper cap on password length, so people can use
	// passphrases but we don't allow MBs of data as a password.
	maxPasswordBytes = 512

	// SecretMarker is a string we can look for in logs to see if the app
	// is accidentally exposing passwords.
	SecretMarker = "<!PASSWORD_REDACTED!>"
)

var InvalidPasswordErr = fmt.Errorf("invalid password")

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
		return Password{}, InvalidPasswordErr
	}

	return Password{
		plain: []byte(pwd),
	}, nil
}

// Match checks if the plaintext password matches the given hash.
func (p Password) Match(h Argon2Hash) bool {
	// Hash the plaintext password with the same parameters as the provided hash.
	other := argon2.IDKey(p.plain, h.Salt, h.Iterations, h.MemoryKiB, h.Parallelism, uint32(len(h.Hash)))

	// use subtle to compare the two hashes in constant time to avoid timing attacks.
	return subtle.ConstantTimeCompare(other, h.Hash) == 1
}

// Hash hashes the plaintext password using the argon2id algorithm.
func (p Password) Hash() (Argon2Hash, error) {
	// First we generate a salt.
	salt, err := generateSalt()
	if err != nil {
		return Argon2Hash{}, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Then we hash the password.
	hash := argon2.IDKey(p.plain, salt, iterations, memoryKiB, parallelism, keyLen)

	return Argon2Hash{
		Variant:     variant,
		Version:     argon2.Version,
		MemoryKiB:   memoryKiB,
		Iterations:  iterations,
		Parallelism: parallelism,
		Salt:        salt,
		Hash:        hash,
	}, nil
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}

	return salt, nil
}

func (p Password) String() string {
	return SecretMarker
}

func (p Password) GoString() string {
	return SecretMarker
}

func (p Password) MarshalText() ([]byte, error) {
	return []byte(SecretMarker), nil
}
