package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/krypto"
)

// NowFunc is a function that returns the current time.
type NowFunc func() time.Time

// Store is responsible for interacting with a database.
type Store struct {
	db            *sql.DB
	encryptor     *krypto.Encryptor
	blindIndexKey krypto.Key
}

// New creates a new Store.
func New(db *sql.DB, encryptor *krypto.Encryptor, blindIndexKey krypto.Key) *Store {
	return &Store{
		db:            db,
		encryptor:     encryptor,
		blindIndexKey: blindIndexKey,
	}
}

// BeginTx starts a new transaction.
func (s *Store) BeginTx(ctx context.Context) (auth.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{
		tx:    tx,
		store: s,
	}, nil
}
