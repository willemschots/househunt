package db

import (
	"database/sql"
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
func (t *Tx) SaveUser(u *User) error {
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
			return fmt.Errorf("tried to update user with id %d that does not exist", u.ID)
		}
	}

	u.UpdatedAt = now

	return nil
}

func (t *Tx) FindUserByEmail(v email.Address) (User, error) {
	return User{}, nil
}

func (t *Tx) SaveEmailToken(v EmailToken) error {
	return nil
}

func (t *Tx) FindEmailToken(v auth.Argon2Hash) (EmailToken, error) {
	return EmailToken{}, nil
}
