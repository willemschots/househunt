package db

import (
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
)

// User contains the data for a user.
type User struct {
	ID           int
	Email        email.Address
	PasswordHash auth.Argon2Hash
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
