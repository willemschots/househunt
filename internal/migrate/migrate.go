package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Migration is a migration that was ran.
type Migration struct {
	// Sequence is the number of the migration. Starts at 0.
	Sequence int
	Filename string
	Metadata Metadata
}

// Equal checks if two migrations are equal.
func (m Migration) Equal(other Migration) bool {
	return m.Sequence == other.Sequence &&
		m.Filename == other.Filename &&
		m.Metadata.AppVersion == other.Metadata.AppVersion &&
		m.Metadata.Timestamp.Equal(other.Metadata.Timestamp)
}

// Metadata contains metadata about a migration.
// If something does go wrong, this will help with debugging.
type Metadata struct {
	AppVersion string
	Timestamp  time.Time
}

const migrationsTableQuery = `CREATE TABLE IF NOT EXISTS migrations (
	sequence    INTEGER PRIMARY KEY,
	filename    TEXT NOT NULL,
	app_version TEXT NOT NULL,
	timestamp   TIMESTAMP NOT NULL
)
`

var (
	// ErrNoTable indicates the migrations table does not exist.
	ErrNoTable = errors.New("migrations table does not exist")
	// ErrMigrationsMismatch indicates a mismatch between migrations that ran before and the ones available now.
	ErrMigrationsMismatch = errors.New("migrations mismatch")
)

// MigrationError is an error that occurred while running a migration.
type MigrationError struct {
	Sequence int
	Filename string
	Err      error
}

func (m MigrationError) Error() string {
	return fmt.Sprintf("migration [%d] %q failed: %v", m.Sequence, m.Filename, m.Err)
}

// RunFS runs migrations from the provided fs.FS. It returns a slice of migrations that were
// run, if no migrations were run it returns an empty slice. RunFS assumes all migration files
// can be loaded into memory. RunFS only considers files with the .sql extension in the
// root of the FS.
func RunFS(ctx context.Context, db *sql.DB, fileSys fs.FS, meta Metadata) ([]Migration, error) {
	// Load all migration files from the filesystem.
	files, err := loadFiles(fileSys)
	if err != nil {
		return nil, err
	}

	// Begin a transaction.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create migrations table if it does not exist.
	_, err = tx.Exec(migrationsTableQuery)
	if err != nil {
		return nil, rollback(tx, fmt.Errorf("failed to create migrations table: %w", err))
	}

	// Query migrations that ran before.
	before, err := queryWith(func(q string) (*sql.Rows, error) {
		return tx.Query(q)
	})
	if err != nil {
		return nil, rollback(tx, err)
	}

	result, err := migrate(tx, before, files, meta)
	if err != nil {
		return nil, rollback(tx, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

func migrate(tx *sql.Tx, ranBefore []Migration, files []file, meta Metadata) ([]Migration, error) {
	// Check if no files were removed.
	if len(ranBefore) > len(files) {
		return nil, fmt.Errorf(
			"found %d existing migrations but only have %d files: %w",
			len(ranBefore), len(files), ErrMigrationsMismatch,
		)
	}

	// Verify the files that ran before.
	for i, before := range ranBefore {
		// Sanity check that sequence is as expected.
		if i != before.Sequence {
			return nil, fmt.Errorf(
				"migration sequence mismatch, wanted %d got %d", i, before.Sequence,
			)
		}

		// Check if the filename matches what we expect.
		if before.Filename != files[i].name {
			return nil, fmt.Errorf(
				"migration %d had filename %s, but now encountering %s: %w",
				i, before.Filename, files[i].name, ErrMigrationsMismatch,
			)
		}
	}

	// prepare the insert statement.
	stmt, err := tx.Prepare(`INSERT INTO migrations (sequence, filename, app_version, timestamp) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	// files now only contains the migrations that need to be ran.
	files = files[len(ranBefore):]

	ranNow := make([]Migration, 0)
	for i, f := range files {
		sequence := len(ranBefore) + i

		_, err := tx.Exec(f.content)
		if err != nil {
			return nil, MigrationError{
				Sequence: sequence,
				Filename: f.name,
				Err:      err,
			}
		}

		m := Migration{
			Sequence: sequence,
			Filename: f.name,
			Metadata: meta,
		}

		ranNow = append(ranNow, m)

		_, err = stmt.Exec(m.Sequence, m.Filename, m.Metadata.AppVersion, m.Metadata.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to insert migration: %w", err)
		}
	}

	return ranNow, nil
}

// QueryMigrations queries the given db for all migrations that ran.
// If the migration table does not exist yet, it returns the ErrNoTable error.
func QueryMigrations(ctx context.Context, db *sql.DB) ([]Migration, error) {
	return queryWith(func(q string) (*sql.Rows, error) {
		return db.QueryContext(ctx, q)
	})
}

func queryWith(rowsFunc func(q string) (*sql.Rows, error)) ([]Migration, error) {
	const q = `SELECT sequence, filename, app_version, timestamp FROM migrations ORDER BY sequence`
	rows, err := rowsFunc(q)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, ErrNoTable
		}
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	migrations := make([]Migration, 0)
	for rows.Next() {
		var m Migration
		err := rows.Scan(&m.Sequence, &m.Filename, &m.Metadata.AppVersion, &m.Metadata.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}

		migrations = append(migrations, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over rows: %w", err)
	}

	return migrations, nil
}

type file struct {
	name    string
	content string
}

func loadFiles(fileSys fs.FS) ([]file, error) {
	entries, err := fs.ReadDir(fileSys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	files := make([]file, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := fs.ReadFile(fileSys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %q: %w", entry.Name(), err)
		}

		files = append(files, file{
			name:    entry.Name(),
			content: string(content),
		})
	}

	return files, nil
}

func rollback(tx *sql.Tx, err error) error {
	rErr := tx.Rollback()
	if rErr != nil {
		return errors.Join(err, rErr)
	}

	return err
}
