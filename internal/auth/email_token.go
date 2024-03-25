package auth

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/willemschots/househunt/internal/email"
)

const (
	tokenLen = 32
)

var ErrInvalidToken = errors.New("invalid token")

// EmailToken contains the state of a random token that is sent via email.
// Such tokens should be only used once and have a limited lifetime.
type EmailToken struct {
	ID int
	// TokenHash is the hash of the token. We hash the token to prevent someone with
	// access to the database from mis-using the tokens.
	TokenHash  Argon2Hash
	UserID     int
	Email      email.Address
	Purpose    EmailTokenPurpose
	CreatedAt  time.Time
	ConsumedAt *time.Time
}

// EmailTokenPurpose represents the purpose of an email token.
type EmailTokenPurpose string

const (
	// EmailTokenPurposeActivate indicates an email token is for activating an account.
	EmailTokenPurposeActivate EmailTokenPurpose = "activate"
)

// Token is a random token that is sent via email.
//
// The only time a token should be provided in plaintext is as part of
// the email to the user. Tokens are confidential and should never be
// exposed in logs or persisted in plaintext.
type Token [tokenLen]byte

// GenerateToken creates a new random token.
func GenerateToken() (Token, error) {
	b, err := genRandomBytes(tokenLen)
	if err != nil {
		return [tokenLen]byte{}, err
	}
	return [tokenLen]byte(b), nil
}

// ParseToken parses a token from a string.
func ParseToken(raw string) (Token, error) {
	if len(raw) != tokenLen*2 {
		return [tokenLen]byte{}, ErrInvalidToken
	}

	b, err := hex.DecodeString(raw)
	if err != nil {
		return [tokenLen]byte{}, ErrInvalidToken
	}

	return [tokenLen]byte(b), nil
}

// String returns the string representation of the token.
// As opposed to a Password this allowed, we need to embed the
// token in emails.
func (t Token) String() string {
	return hex.EncodeToString(t[:])
}

// Hash hashes the token using the argon2id algorithm.
func (t Token) Hash() (Argon2Hash, error) {
	return hashBytes(t[:])
}

// Match checks if the token matches the given hash.
func (t Token) Match(h Argon2Hash) bool {
	return matchHash(h, t[:])
}

// LogValue implements the slog.Valuer interface.
func (t Token) LogValue() slog.Value {
	return slog.StringValue(SecretMarker)
}
