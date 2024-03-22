package db

import (
	"context"
	"database/sql"
	"time"
)

// NowFunc is a function that returns the current time.
type NowFunc func() time.Time

// Store is responsible for interacting with a database.
type Store struct {
	db      *sql.DB
	nowFunc NowFunc
}

// New creates a new Store.
func New(db *sql.DB, nowFunc NowFunc) *Store {
	return &Store{
		db:      db,
		nowFunc: nowFunc,
	}
}

// BeginTx starts a new transaction.
func (s *Store) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{
		tx:      tx,
		nowFunc: s.nowFunc,
	}, nil
}
