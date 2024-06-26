package auth

import (
	"time"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/krypto"

	"github.com/google/uuid"
)

// User contains the data for a user.
type User struct {
	ID           uuid.UUID
	Email        email.Address
	PasswordHash krypto.Argon2Hash
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Credentials are used to authenticate users.
type Credentials struct {
	Password Password
	Email    email.Address
}
