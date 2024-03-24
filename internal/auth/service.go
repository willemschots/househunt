package auth

import (
	"context"

	"github.com/willemschots/househunt/internal/email"
)

// Store provides access to the user store.
type Store interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx is a transaction. If an error occurs on any of the Create/Update/Find methods,
// the transaction is considered to have failed and should be rolled back.
// Tx is not safe for concurrent use.
type Tx interface {
	Commit() error
	Rollback() error

	CreateUser(u *User) error
	UpdateUser(u *User) error
	FindUserByEmail(v email.Address) (User, error)

	CreateEmailToken(t *EmailToken) error
	UpdateEmailToken(t *EmailToken) error
	FindEmailTokenByID(id int) (EmailToken, error)
}
