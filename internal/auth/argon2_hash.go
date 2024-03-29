package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidArgon2Hash is returned when an invalid argon2 hash is parsed.
var ErrInvalidArgon2Hash = fmt.Errorf("invalid argon2 hash")

const (
	variant = "argon2id"
	// use the recommended parameters for argon2 password hashing according to OWASP:
	// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#argon2id
	memoryKiB   = 46 * 1024
	iterations  = 1
	parallelism = 1
)

// Argon2Hash is a hash generated by the Argon2 Hashing Algorithm.
type Argon2Hash struct {
	Variant     string
	Version     uint32
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	Salt        []byte
	Hash        []byte
}

// ParseArgon2Hash parses an argon2 hash from the string representation provided by the String method.
func ParseArgon2Hash(txt string) (Argon2Hash, error) {
	// Split the string into its components.
	vals := strings.Split(txt, "$")
	if len(vals) != 6 {
		return Argon2Hash{}, fmt.Errorf("wrong number of components: %w", ErrInvalidArgon2Hash)
	}

	h := Argon2Hash{}
	for i, v := range vals {
		if !parseComponent(i, v, &h) {
			return Argon2Hash{}, fmt.Errorf("failed to parse component %d: %w", i, ErrInvalidArgon2Hash)
		}
	}

	return h, nil
}

// parseComponent parses a single of a textual argon2 hash representation.
// i is the component index, v is the component value and h is the hash to populate.
// return false to indicate that the component was not parsed successfully.
func parseComponent(i int, v string, h *Argon2Hash) bool {
	switch i {
	case 0:
		return v == ""
	case 1:
		h.Variant = v
		return v == variant
	case 2:
		_, err := fmt.Sscanf(v, "v=%d", &h.Version)
		return err == nil && h.Version == argon2.Version
	case 3:
		_, err := fmt.Sscanf(v, "m=%d,t=%d,p=%d", &h.MemoryKiB, &h.Iterations, &h.Parallelism)
		return err == nil
	case 4:
		salt, err := base64.RawStdEncoding.DecodeString(v)
		h.Salt = salt
		return err == nil
	case 5:
		hash, err := base64.RawStdEncoding.DecodeString(v)
		h.Hash = hash
		return err == nil
	}

	return false
}

// String returns the string representation of the hash.
func (h Argon2Hash) String() string {
	b64Hash := base64.RawStdEncoding.EncodeToString(h.Hash)
	b64Salt := base64.RawStdEncoding.EncodeToString(h.Salt)

	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s", h.Variant, h.Version, h.MemoryKiB, h.Iterations, h.Parallelism, b64Salt, b64Hash)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (h Argon2Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (h *Argon2Hash) UnmarshalText(text []byte) error {
	parsed, err := ParseArgon2Hash(string(text))
	if err != nil {
		return err
	}
	*h = parsed
	return nil
}

// Scan implements the sql.Scanner interface.
func (h *Argon2Hash) Scan(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("can only scan strings, got %T", v)
	}

	parsed, err := ParseArgon2Hash(s)
	if err != nil {
		return err
	}

	*h = parsed

	return nil
}

func hashBytes(b []byte) (Argon2Hash, error) {
	// First we generate a salt.
	salt, err := genRandomBytes(saltLen)
	if err != nil {
		return Argon2Hash{}, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Then we hash the password.
	hash := argon2.IDKey(b, salt, iterations, memoryKiB, parallelism, keyLen)

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

func matchHash(h Argon2Hash, b []byte) bool {
	// Hash the plaintext password with the same parameters as the provided hash.
	other := argon2.IDKey(b, h.Salt, h.Iterations, h.MemoryKiB, h.Parallelism, uint32(len(h.Hash)))

	// compare the two hashes in constant time to avoid timing attacks.
	return subtle.ConstantTimeCompare(other, h.Hash) == 1
}

func genRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}

	return b, nil
}
