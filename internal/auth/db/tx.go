package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
)

type Tx struct {
	tx      *sql.Tx
	nowFunc NowFunc
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// SaveUser saves a user to the database.
// The provided user might have its ID, CreatedAt and UpdatedAt modified.
func (t *Tx) SaveUser(u *auth.User) error {
	now := t.nowFunc()

	if u.ID == 0 {
		const q = `INSERT INTO users (email, password_hash, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
		result, err := t.tx.Exec(q, u.Email, u.PasswordHash.String(), u.IsActive, now, now)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}

		u.ID = int(id)
		u.CreatedAt = now
	} else {
		const q = `UPDATE users SET email = ?, password_hash = ?, is_active = ?, updated_at = ? WHERE id = ?`
		result, err := t.tx.Exec(q, u.Email, u.PasswordHash.String(), u.IsActive, now, u.ID)
		if err != nil {
			return err
		}

		n, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if n != 1 {
			return fmt.Errorf("tried to update user with id %d: %w", u.ID, ErrNotFound)
		}
	}

	u.UpdatedAt = now

	return nil
}

func (t *Tx) FindUserByEmail(v email.Address) (auth.User, error) {
	const q = `SELECT id, email, password_hash, is_active, created_at, updated_at FROM users WHERE email = ?`
	row := t.tx.QueryRow(q, v)

	var u auth.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, ErrNotFound
		}
		return auth.User{}, err
	}

	return u, nil
}

// TODO:
//func (t *Tx) SaveEmailToken(v auth.EmailToken) error {
//	return nil
//}
//
//func (t *Tx) FindEmailToken(v auth.Argon2Hash) (auth.EmailToken, error) {
//	return auth.EmailToken{}, nil
//}
