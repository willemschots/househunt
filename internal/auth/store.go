package auth

import (
	"context"

	"github.com/willemschots/househunt/internal/email"
)

// UserFilter is used filter users.
// Returned users must match all the provided fields.
// If a field is empty or nil, it's ignored.
type UserFilter struct {
	IDs      []int
	Emails   []email.Address
	IsActive *bool
}

// EmailTokenFilter is used to filter email tokens.
// Returned tokens must match all the provided fields.
// If a field is empty or nil, it's ignored.
type EmailTokenFilter struct {
	IDs        []int
	UserIDs    []int
	Purposes   []TokenPurpose
	IsConsumed *bool
}

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
	FindUsers(filter *UserFilter) ([]User, error)

	CreateEmailToken(t *EmailToken) error
	UpdateEmailToken(t *EmailToken) error
	FindEmailTokens(filter *EmailTokenFilter) ([]EmailToken, error)
}
