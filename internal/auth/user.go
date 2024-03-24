package auth

import (
	"time"

	"github.com/willemschots/househunt/internal/email"
)

// User contains the data for a user.
type User struct {
	ID           int
	Email        email.Address
	PasswordHash Argon2Hash
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
