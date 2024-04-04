package auth

import (
	"time"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/krypto"
)

// User contains the data for a user.
type User struct {
	ID           int
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
