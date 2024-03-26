package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/db/migrate"
	"github.com/willemschots/househunt/internal/db/testdb"
)

func Test_RunFS(t *testing.T) {
	t.Run("ok, empty dir", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		meta := migrate.Metadata{
			"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z"),
		}

		got, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/emptydir"), meta)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assertMigrations(t, got, []migrate.Migration{})
		assertTable(t, db, []migrate.Migration{})
	})

	t.Run("ok, subdir is skipped", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		meta := migrate.Metadata{
			"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z"),
		}

		got, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/skip_subdir"), meta)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []migrate.Migration{
			{
				Sequence: 0,
				Filename: "2_create_test_table.sql",
				Metadata: migrate.Metadata{
					"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z"),
				},
			},
		}
		assertMigrations(t, got, want)
		assertTable(t, db, want)
		assertNrOfRowsInTestTable(t, db, 0)
	})

	t.Run("ok, progression of migrations", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		metas := []migrate.Metadata{
			{"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z")},
			{"v2.0.0", timeRFC3339(t, "2024-04-20T14:56:00Z")},
			{"v3.0.0", timeRFC3339(t, "2024-05-20T14:56:00Z")},
		}

		migrations := []migrate.Migration{
			{
				Sequence: 0,
				Filename: "1_create_test_table.sql",
				Metadata: metas[0],
			},
			{
				Sequence: 1,
				Filename: "2_add_row_to_test_table.sql",
				Metadata: metas[1],
			},
			{
				Sequence: 2,
				Filename: "3_add_another_row.sql",
				Metadata: metas[2],
			},
			{
				Sequence: 3,
				Filename: "4_and_one_more.sql",
				Metadata: metas[2],
			},
		}

		t.Run("run_1", func(t *testing.T) {
			got, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/progression/run_1"), metas[0])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertMigrations(t, got, migrations[:1])
			assertTable(t, db, migrations[:1])
			assertNrOfRowsInTestTable(t, db, 0)
		})

		t.Run("run_2", func(t *testing.T) {
			got, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/progression/run_2"), metas[1])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertMigrations(t, got, migrations[1:2])
			assertTable(t, db, migrations[:2])
			assertNrOfRowsInTestTable(t, db, 1)
		})

		t.Run("run_3", func(t *testing.T) {
			got, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/progression/run_3"), metas[2])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertMigrations(t, got, migrations[2:4])
			assertTable(t, db, migrations[:4])
			assertNrOfRowsInTestTable(t, db, 3)
		})
	})

	t.Run("fail, error in migration", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		metas := []migrate.Metadata{
			{"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z")},
			{"v2.0.0", timeRFC3339(t, "2024-04-20T14:56:00Z")},
		}

		t.Run("run_1", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/error_in_migration/run_1"), metas[0])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertNrOfRowsInTestTable(t, db, 0)
		})

		t.Run("run_2", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/error_in_migration/run_2"), metas[1])

			var mErr migrate.MigrationError
			if !errors.As(err, &mErr) {
				t.Fatalf("got %T, want %T", err, mErr)
			}

			want := migrate.MigrationError{
				Sequence: 1,
				Filename: "2_insert_with_typo.sql",
			}

			if mErr.Sequence != want.Sequence || mErr.Filename != want.Filename {
				t.Errorf("got %v, want %v", mErr, want)
			}
		})
	})

	t.Run("fail, migration file that was executed was removed from disk", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		metas := []migrate.Metadata{
			{"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z")},
			{"v2.0.0", timeRFC3339(t, "2024-04-20T14:56:00Z")},
		}

		t.Run("run_1", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/removal_mismatch/run_1"), metas[0])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Just check if the migrations ran.
			assertNrOfRowsInTestTable(t, db, 3)
		})

		t.Run("run_2", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/removal_mismatch/run_2"), metas[1])
			if !errors.Is(err, migrate.ErrMigrationsMismatch) {
				t.Fatalf("got %v, want %v (via errors.Is)", err, migrate.ErrMigrationsMismatch)
			}

			assertNrOfRowsInTestTable(t, db, 3)
		})
	})

	t.Run("fail, migration file that was executed was renamed", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		metas := []migrate.Metadata{
			{"v1.0.0", timeRFC3339(t, "2024-03-20T14:56:00Z")},
			{"v2.0.0", timeRFC3339(t, "2024-04-20T14:56:00Z")},
		}

		t.Run("run_1", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/rename_mismatch/run_1"), metas[0])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Just check if the migrations ran.
			assertNrOfRowsInTestTable(t, db, 3)
		})

		t.Run("run_2", func(t *testing.T) {
			_, err := migrate.RunFS(context.Background(), db, os.DirFS("./testdata/rename_mismatch/run_2"), metas[1])
			if !errors.Is(err, migrate.ErrMigrationsMismatch) {
				t.Fatalf("got %v, want %v (via errors.Is)", err, migrate.ErrMigrationsMismatch)
			}

			assertNrOfRowsInTestTable(t, db, 3)
		})
	})
}

func Test_QueryMigrations(t *testing.T) {
	t.Run("fail, no table", func(t *testing.T) {
		db := testdb.RunUnmigratedWhile(t, true)

		_, err := migrate.QueryMigrations(context.Background(), db)
		if !errors.Is(err, migrate.ErrNoTable) {
			t.Fatalf("got %v, want %v (via errors.Is)", err, migrate.ErrNoTable)
		}
	})
}

func assertTable(t *testing.T, db *sql.DB, want []migrate.Migration) {
	t.Helper()

	got, err := migrate.QueryMigrations(context.Background(), db)
	if err != nil {
		t.Fatalf("failed to query migrations: %v", err)
	}

	assertMigrations(t, got, want)
}

func assertMigrations(t *testing.T, got, want []migrate.Migration) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("got\n%+v\nwant\n%+v\n", got, want)
	}

	if len(want) == 1 && got == nil {
		t.Errorf("got\n%+v\nwant\n%+v\n", got, want)
	}

	for i := range got {
		if !got[i].Equal(want[i]) {
			t.Errorf("got\n%+v\nwant\n%+v\n", got, want)
		}
	}
}

// assertNrOfRowsInTestTable checks the number of rows in the test_table.
// The test table is created by our testdata. Some migrations add rows to it,
// enabling us to test if migrations were executed.
func assertNrOfRowsInTestTable(t *testing.T, db *sql.DB, want int) {
	t.Helper()

	row := db.QueryRow("SELECT COUNT(*) FROM test_table")

	var got int
	err := row.Scan(&got)
	if err != nil {
		t.Fatalf("failed to scan test_table: %v", err)
	}

	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func timeRFC3339(t *testing.T, v string) time.Time {
	t.Helper()

	ts, err := time.Parse(time.RFC3339, v)
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}

	return ts
}
