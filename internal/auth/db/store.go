package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/willemschots/househunt/internal/auth"
)

// NowFunc is a function that returns the current time.
type NowFunc func() time.Time

// Store is responsible for interacting with a database.
type Store struct {
	db *sql.DB
}

// New creates a new Store.
func New(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}

// BeginTx starts a new transaction.
func (s *Store) BeginTx(ctx context.Context) (auth.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{
		tx: tx,
	}, nil
}
