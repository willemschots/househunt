package krypto

import (
	"encoding/hex"
	"errors"
	"log/slog"
)

const (
	tokenLen = 32
)

var ErrInvalidToken = errors.New("invalid token")

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

// LogValue implements the slog.Valuer interface.
func (t Token) LogValue() slog.Value {
	return slog.StringValue(SecretMarker)
}
