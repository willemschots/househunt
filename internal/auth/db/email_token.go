package db

import (
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
)

// EmailToken contains the data for an email token.
type EmailToken struct {
	TokenHash  auth.Argon2Hash
	UserID     int
	Email      email.Address
	Purpose    string // TODO: enum
	CreatedAt  time.Time
	ConsumedAt *time.Time
}
