package auth

import (
	"time"

	"github.com/willemschots/househunt/internal/email"
)

// EmailToken contains the data for an email token.
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
