package db

import (
	"database/sql"
	"fmt"
	"reflect"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/errorz"
)

type Tx struct {
	tx       *sql.Tx
	nowFunc  NowFunc
	badState bool
}

func (t *Tx) Commit() error {
	if t.badState {
		return errorz.ErrTxBadState
	}
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// CreateUser creates a user in the database.
// It updates the users ID, CreatedAt and UpdatedAt fields when successful.
func (t *Tx) CreateUser(u *auth.User) error {
	if u.ID != 0 {
		return fmt.Errorf("user already has an ID: %w", errorz.ErrConstraintViolated)
	}

	now := t.nowFunc()

	const q = `INSERT INTO users (email, password_hash, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	result, err := t.tx.Exec(q, u.Email, u.PasswordHash.String(), u.IsActive, now, now)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Only set the fields after the query was executed.
	u.ID = int(id)
	u.CreatedAt = now
	u.UpdatedAt = now

	return nil
}

// UpdateUser updates a user in the database.
// It updates the users UpdatedAt field when successful.
// It returns errorz.ErrNotFound if no user is found.
func (t *Tx) UpdateUser(user *auth.User) error {
	now := t.nowFunc()

	const q = `UPDATE users SET email = ?, password_hash = ?, is_active = ?, updated_at = ? WHERE id = ?`
	result, err := t.tx.Exec(q, user.Email, user.PasswordHash.String(), user.IsActive, now, user.ID)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if n != 1 {
		return fmt.Errorf("failed to update user %d: %w", user.ID, errorz.ErrNotFound)
	}

	// Only set the fields after the query was executed.
	user.UpdatedAt = now

	return nil
}

// FindUsers queries for users based on the provided filter.
// It returns an empty slice if no users are found.
func (t *Tx) FindUsers(filter *auth.UserFilter) ([]auth.User, error) {
	q, params := userFilterQuery(filter)
	rows, err := t.tx.Query(q, params...)
	if err != nil {
		return nil, errorz.MapDBErr(err)
	}

	defer rows.Close()

	out := make([]auth.User, 0)
	for rows.Next() {
		var u auth.User
		err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, errorz.MapDBErr(err)
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, errorz.MapDBErr(err)
	}

	return out, nil
}

func userFilterQuery(f *auth.UserFilter) (string, []any) {
	var q db.Query

	q.Query(`SELECT id, email, password_hash, is_active, created_at, updated_at FROM users WHERE 1=1 `)

	if len(f.IDs) > 0 {
		q.Query(`AND id IN (`)
		q.Params(anySlice(f.IDs)...)
		q.Query(`)`)
	}

	if len(f.Emails) > 0 {
		q.Query(`AND email IN (`)
		q.Params(anySlice(f.Emails)...)
		q.Query(`)`)
	}

	if f.IsActive != nil {
		q.Query("AND is_active = ")
		q.Param(f.IsActive)
	}

	q.Query(` ORDER BY id ASC`)

	return q.Get()
}

// CreateEmailToken creates an email token in the database.
// It updates the token ID and CreatedAt when successful.
func (t *Tx) CreateEmailToken(token *auth.EmailToken) error {
	if token.ID != 0 {
		return fmt.Errorf("email token already has an ID: %w", errorz.ErrConstraintViolated)
	}

	now := t.nowFunc()

	const q = `INSERT INTO email_tokens (token_hash, user_id, email, purpose, created_at, consumed_at) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := t.tx.Exec(q, token.TokenHash.String(), token.UserID, token.Email, token.Purpose, now, token.ConsumedAt)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	token.ID = int(id)
	token.CreatedAt = now

	return nil
}

// UpdateEmailToken updates an email token in the database.
// It returns errorz.ErrNotFound if no email token is found.
// It only allows updating the ConsumedAt field, attempting to
// update any other field will return errorz.ErrConstraintViolated.
func (t *Tx) UpdateEmailToken(token *auth.EmailToken) error {
	const q = `UPDATE email_tokens SET consumed_at = ? WHERE id = ? RETURNING token_hash, user_id, email, purpose`
	row := t.tx.QueryRow(q, token.ConsumedAt, token.ID)

	var out auth.EmailToken
	err := row.Scan(&out.TokenHash, &out.UserID, &out.Email, &out.Purpose)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	if !reflect.DeepEqual(out.TokenHash, token.TokenHash) || out.UserID != token.UserID || out.Email != token.Email || out.Purpose != token.Purpose {
		// We have already updated, the transaction is in a bad state.
		t.badState = true
		return fmt.Errorf("trying to update immutable field: %w", errorz.ErrConstraintViolated)
	}

	return nil
}

// FindEmailTokens queries for email tokens based on the provided filter.
func (t *Tx) FindEmailTokens(filter *auth.EmailTokenFilter) ([]auth.EmailToken, error) {
	q, params := emailTokenFilterQuery(filter)
	rows, err := t.tx.Query(q, params...)
	if err != nil {
		return nil, errorz.MapDBErr(err)
	}

	defer rows.Close()

	out := make([]auth.EmailToken, 0)
	for rows.Next() {
		var token auth.EmailToken
		err := rows.Scan(&token.ID, &token.TokenHash, &token.UserID, &token.Email, &token.Purpose, &token.CreatedAt, &token.ConsumedAt)
		if err != nil {
			return nil, errorz.MapDBErr(err)
		}

		out = append(out, token)
	}

	if err := rows.Err(); err != nil {
		return nil, errorz.MapDBErr(err)
	}

	return out, nil
}

func emailTokenFilterQuery(f *auth.EmailTokenFilter) (string, []any) {
	var q db.Query

	q.Query(`SELECT id, token_hash, user_id, email, purpose, created_at, consumed_at FROM email_tokens WHERE 1=1 `)

	if len(f.IDs) > 0 {
		q.Query(`AND id IN (`)
		q.Params(anySlice(f.IDs)...)
		q.Query(`) `)
	}

	if len(f.UserIDs) > 0 {
		q.Query(`AND user_id IN (`)
		q.Params(anySlice(f.UserIDs)...)
		q.Query(`) `)
	}

	if len(f.Purposes) > 0 {
		q.Query(`AND purpose IN (`)
		q.Params(anySlice(f.Purposes)...)
		q.Query(`) `)
	}

	if f.IsConsumed != nil {
		q.Query("AND consumed_at IS ")
		if *f.IsConsumed {
			q.Query("NOT ")
		}
		q.Query("NULL ")
	}

	return q.Get()
}

func anySlice[T any](s []T) []any {
	out := make([]any, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}
