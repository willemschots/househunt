package auth

import (
	"context"

	"github.com/willemschots/househunt/internal/email"
)

// Store provides access to the user store.
type Store interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx is a transaction.
type Tx interface {
	Commit() error
	Rollback() error
	SaveUser(u *User) error
	FindUserByEmail(v email.Address) (User, error)
}
