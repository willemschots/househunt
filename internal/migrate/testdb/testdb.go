package testdb

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/willemschots/househunt/internal/migrate"
	"github.com/willemschots/househunt/migrations"
)

// RunTestDB runs a database while the provided test is executing.
// It returns an empty database with all migrations applied.
func RunTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = migrate.RunFS(ctx, db, migrations.FS, migrate.Metadata{})
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}