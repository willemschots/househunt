package auth

import (
	"time"

	"github.com/google/uuid"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/krypto"
)

// EmailToken contains the state of a token that was sent via email.
// Such tokens should be only used once and have a limited lifetime.
type EmailToken struct {
	ID uuid.UUID
	// TokenHash is the hash of the token. We hash the token to prevent someone with
	// access to the database from mis-using the tokens.
	TokenHash  krypto.Argon2Hash
	UserID     uuid.UUID
	Email      email.Address
	Purpose    TokenPurpose
	CreatedAt  time.Time
	ConsumedAt *time.Time
}

// TokenPurpose is the purpose of an email token.
type TokenPurpose string

const (
	// TokenPurposeActivate indicates a token should be used to activate an user.
	TokenPurposeActivate TokenPurpose = "activate"
	// TokenPurposePasswordReset indicates a token should be used to reset a password.
	TokenPurposePasswordReset TokenPurpose = "password_reset"
)

// EmailTokenRaw is the raw data that will be send to the user via email.
type EmailTokenRaw struct {
	ID    uuid.UUID
	Token krypto.Token
}
