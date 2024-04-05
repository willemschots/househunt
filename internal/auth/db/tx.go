package db

import (
	"database/sql"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/db"
)

type Tx struct {
	tx    *sql.Tx
	store *Store
}

func (t *Tx) newQuery() db.Query {
	return db.Query{
		Encryptor:     t.store.encryptor,
		BlindIndexKey: t.store.blindIndexKey,
	}
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// CreateUser creates a user in the database.
// It updates the users ID, CreatedAt and UpdatedAt fields when successful.
func (t *Tx) CreateUser(u *auth.User) error {
	return insertUser(t.newQuery(), t.tx.Exec, u)
}

// UpdateUser updates a user in the database.
// It updates the users UpdatedAt field when successful.
// It returns errorz.ErrNotFound if no user is found.
func (t *Tx) UpdateUser(u *auth.User) error {
	return updateUser(t.newQuery(), t.tx.Exec, u)
}

// FindUsers queries for users based on the provided filter.
// It returns an empty slice if no users are found.
func (t *Tx) FindUsers(filter *auth.UserFilter) ([]auth.User, error) {
	return selectUsers(t.newQuery(), t.tx.Query, filter)
}

// CreateEmailToken creates an email token in the database.
// It updates the token ID and CreatedAt when successful.
func (t *Tx) CreateEmailToken(tok *auth.EmailToken) error {
	return insertEmailToken(t.newQuery(), t.tx.Exec, tok)
}

// UpdateEmailToken updates an email token in the database.
// It returns errorz.ErrNotFound if no email token is found.
// It only allows updating the ConsumedAt field, attempting to
// update any other field will return errorz.ErrConstraintViolated.
func (t *Tx) UpdateEmailToken(tok *auth.EmailToken) error {
	return updateEmailToken(t.newQuery(), t.tx.Exec, tok)
}

// FindEmailTokens queries for email tokens based on the provided filter.
func (t *Tx) FindEmailTokens(filter *auth.EmailTokenFilter) ([]auth.EmailToken, error) {
	return selectEmailTokens(t.newQuery(), t.tx.Query, filter)
}
