package testdb

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/db/migrate"
	"github.com/willemschots/househunt/migrations"
)

// RunWhile runs a database while the provided test is executing.
// It returns an empty database with all migrations applied.
func RunWhile(t *testing.T, write bool) *sql.DB {
	t.Helper()

	db := RunUnmigratedWhile(t, write)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := migrate.RunFS(ctx, db, migrations.FS, migrate.Metadata{})
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

// RunUnmigratedWhile runs a database while the provided test is executing.
// It returns an empty database without any migrations applied.
func RunUnmigratedWhile(t *testing.T, write bool) *sql.DB {
	t.Helper()

	db, err := db.OpenSQLite(":memory:", write)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		err := db.Close()
		if err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	})

	return db
}
