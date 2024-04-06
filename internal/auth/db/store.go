package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/krypto"
)

// NowFunc is a function that returns the current time.
type NowFunc func() time.Time

// Store is responsible for interacting with a database.
type Store struct {
	writeDB       *sql.DB
	readDB        *sql.DB
	encryptor     *krypto.Encryptor
	blindIndexKey krypto.Key
}

// New creates a new Store.
func New(writeDB, readDB *sql.DB, encryptor *krypto.Encryptor, blindIndexKey krypto.Key) *Store {
	return &Store{
		writeDB:       writeDB,
		readDB:        readDB,
		encryptor:     encryptor,
		blindIndexKey: blindIndexKey,
	}
}

func (s *Store) newQuery() db.Query {
	return db.Query{
		Encryptor:     s.encryptor,
		BlindIndexKey: s.blindIndexKey,
	}
}

// BeginTx starts a new transaction.
func (s *Store) BeginTx(ctx context.Context) (auth.Tx, error) {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{
		tx:    tx,
		store: s,
	}, nil
}

func (s *Store) FindUsers(ctx context.Context, filter *auth.UserFilter) ([]auth.User, error) {
	return selectUsers(s.newQuery(), func(query string, params ...any) (*sql.Rows, error) {
		return s.readDB.QueryContext(ctx, query, params...)
	}, filter)
}
