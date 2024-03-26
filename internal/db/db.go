package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const (
	// both options use wal mode, foreign keys, and a busy timeout of 5 seconds.
	// the writeOptions also use immediate transactions to prevent locking issues.
	// To run SQLite so that it works well with out app, we need a few options
	// We need to configure a few options to make sure SQLite works well with our app:
	// - WAL Mode so that reads and writes don't block eachother.
	// - A busy timeout, specifying the duration a connection will wait for a lock.
	// - Foreign keys are enforced.
	writeOptions = "?mode=rw_&_foreign_keys=on&_journal_mode=wal&_busy_timeout=5000&_txlock=immediate"
	readOptions  = "?mode=ro_&_foreign_keys=on&_journal_mode=wal&_busy_timeout=5000"
)

// OpenSQLite3 opens a pool of SQLite3 connections. Different settings
// are appropriate for reading and writing, so this function needs to know
// what the sql.DB will be used for.
//
// See this comment for more information:
// https://github.com/mattn/go-sqlite3/issues/1179#issuecomment-1638083995
func OpenSQLite(dbFile string, write bool) (*sql.DB, error) {
	optsPostfix := readOptions
	if write {
		optsPostfix = writeOptions
	}

	// Open the database file with the correct options.
	db, err := sql.Open("sqlite3", dbFile+optsPostfix)
	if err != nil {
		return nil, err
	}

	if write {
		// use only a single connection for writing.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// don't close this connection.
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(0)
	}

	return db, nil
}
