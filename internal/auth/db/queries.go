package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
)

type execFunc func(query string, params ...any) (sql.Result, error)
type queryFunc func(query string, params ...any) (*sql.Rows, error)

func insertUser(q db.Query, ef execFunc, u auth.User) error {
	if u.ID == uuid.Nil {
		return fmt.Errorf("zero uuid provided: %w", errorz.ErrConstraintViolated)
	}

	q.Unsafe(`INSERT INTO users (id, email_encrypted, email_blind_index, password_hash, is_active, created_at, updated_at) VALUES (`)
	q.Param(u.ID)
	q.Unsafe(`, `)
	q.ParamEncrypted([]byte(u.Email))
	q.Unsafe(`, `)
	q.ParamBlindIndex([]byte(u.Email))
	q.Unsafe(`, `)
	q.Params(u.PasswordHash.String(), u.IsActive, u.CreatedAt, u.UpdatedAt)
	q.Unsafe(`)`)

	s, params, err := q.Get()
	if err != nil {
		return err
	}

	_, err = ef(s, params...)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	return nil
}

func updateUser(q db.Query, ef execFunc, u auth.User) error {
	q.Unsafe(`UPDATE users SET `)

	q.Unsafe(`email_encrypted = `)
	q.ParamEncrypted([]byte(u.Email))

	q.Unsafe(`, email_blind_index = `)
	q.ParamBlindIndex([]byte(u.Email))

	q.Unsafe(`, password_hash = `)
	q.Param(u.PasswordHash.String())

	q.Unsafe(`, is_active = `)
	q.Param(u.IsActive)

	q.Unsafe(`, created_at = `)
	q.Param(u.CreatedAt)

	q.Unsafe(`, updated_at = `)
	q.Param(u.UpdatedAt)

	q.Unsafe(` WHERE id = `)
	q.Params(u.ID)

	s, params, err := q.Get()
	if err != nil {
		return err
	}

	result, err := ef(s, params...)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errorz.MapDBErr(err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found: %w", errorz.ErrNotFound)
	}

	return nil
}

func selectUsers(q db.Query, qf queryFunc, f auth.UserFilter) ([]auth.User, error) {
	q.Unsafe(`SELECT id, email_encrypted, password_hash, is_active, created_at, updated_at FROM users WHERE 1=1 `)

	if len(f.IDs) > 0 {
		q.Unsafe(`AND id IN (`)
		q.Params(anySlice(f.IDs)...)
		q.Unsafe(`)`)
	}

	if len(f.Emails) > 0 {
		q.Unsafe(`AND email_blind_index IN (`)
		for i, email := range f.Emails {
			if i > 0 {
				q.Unsafe(`, `)
			}
			q.ParamBlindIndex([]byte(email))
		}
		q.Unsafe(`)`)
	}

	if f.IsActive != nil {
		q.Unsafe("AND is_active = ")
		q.Param(f.IsActive)
	}

	q.Unsafe(` ORDER BY id ASC`)

	s, params, err := q.Get()
	if err != nil {
		return nil, err
	}

	rows, err := qf(s, params...)
	if err != nil {
		return nil, errorz.MapDBErr(err)
	}

	defer rows.Close()

	out := make([]auth.User, 0)
	for rows.Next() {
		var u auth.User
		emailBytes := q.DecryptionTarget()
		err := rows.Scan(&u.ID, emailBytes, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, errorz.MapDBErr(err)
		}

		u.Email, err = email.ParseAddress(string(emailBytes.Data))
		if err != nil {
			return nil, err
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, errorz.MapDBErr(err)
	}

	return out, nil
}

func insertEmailToken(q db.Query, ef execFunc, tok auth.EmailToken) error {
	if tok.ID == uuid.Nil {
		return fmt.Errorf("zero uuid provided: %w", errorz.ErrConstraintViolated)
	}

	q.Unsafe(`INSERT INTO email_tokens (id, token_hash, user_id, email_encrypted, purpose, created_at, consumed_at) VALUES (`)
	q.Params(tok.ID, tok.TokenHash.String(), tok.UserID)
	q.Unsafe(`, `)
	q.ParamEncrypted([]byte(tok.Email))
	q.Unsafe(`, `)
	q.Params(tok.Purpose, tok.CreatedAt, tok.ConsumedAt)
	q.Unsafe(`)`)

	s, params, err := q.Get()
	if err != nil {
		return err
	}

	_, err = ef(s, params...)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	return nil
}

func updateEmailToken(q db.Query, ef execFunc, tok auth.EmailToken) error {
	q.Unsafe(`UPDATE email_tokens SET `)

	q.Unsafe(`token_hash = `)
	q.Param(tok.TokenHash.String())

	q.Unsafe(`, user_id = `)
	q.Param(tok.UserID)

	q.Unsafe(`, email_encrypted = `)
	q.ParamEncrypted([]byte(tok.Email))

	q.Unsafe(`, purpose = `)
	q.Param(tok.Purpose)

	q.Unsafe(`, created_at = `)
	q.Param(tok.CreatedAt)

	q.Unsafe(`, consumed_at = `)
	q.Param(tok.ConsumedAt)

	q.Unsafe(` WHERE id = `)
	q.Params(tok.ID)

	s, params, err := q.Get()
	if err != nil {
		return err
	}

	result, err := ef(s, params...)
	if err != nil {
		return errorz.MapDBErr(err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errorz.MapDBErr(err)
	}

	if rows == 0 {
		return fmt.Errorf("email token not found: %w", errorz.ErrNotFound)
	}

	return nil
}

func selectEmailTokens(q db.Query, qf queryFunc, f auth.EmailTokenFilter) ([]auth.EmailToken, error) {
	q.Unsafe(`SELECT id, token_hash, user_id, email_encrypted, purpose, created_at, consumed_at FROM email_tokens WHERE 1=1 `)

	if len(f.IDs) > 0 {
		q.Unsafe(`AND id IN (`)
		q.Params(anySlice(f.IDs)...)
		q.Unsafe(`) `)
	}

	if len(f.UserIDs) > 0 {
		q.Unsafe(`AND user_id IN (`)
		q.Params(anySlice(f.UserIDs)...)
		q.Unsafe(`) `)
	}

	if len(f.Purposes) > 0 {
		q.Unsafe(`AND purpose IN (`)
		q.Params(anySlice(f.Purposes)...)
		q.Unsafe(`) `)
	}

	if f.IsConsumed != nil {
		q.Unsafe("AND consumed_at IS ")
		if *f.IsConsumed {
			q.Unsafe("NOT ")
		}
		q.Unsafe("NULL ")
	}

	s, params, err := q.Get()
	if err != nil {
		return nil, err
	}

	rows, err := qf(s, params...)
	if err != nil {
		return nil, errorz.MapDBErr(err)
	}

	defer rows.Close()

	out := make([]auth.EmailToken, 0)
	for rows.Next() {
		var token auth.EmailToken
		emailBytes := q.DecryptionTarget()
		err := rows.Scan(&token.ID, &token.TokenHash, &token.UserID, emailBytes, &token.Purpose, &token.CreatedAt, &token.ConsumedAt)
		if err != nil {
			return nil, errorz.MapDBErr(err)
		}

		token.Email, err = email.ParseAddress(string(emailBytes.Data))
		if err != nil {
			return nil, err
		}

		out = append(out, token)
	}

	if err := rows.Err(); err != nil {
		return nil, errorz.MapDBErr(err)
	}

	return out, nil
}

func anySlice[T any](s []T) []any {
	out := make([]any, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}
