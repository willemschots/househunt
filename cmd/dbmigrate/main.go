package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/willemschots/househunt/internal"
	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/db/migrate"
	"github.com/willemschots/househunt/migrations"
)

const helpText = `Usage: dbmigrate [sqlite_file]`

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, helpText)
		os.Exit(1)
	}

	dbFile := os.Args[1]

	sqlDB, err := db.OpenSQLite(dbFile, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	meta := migrate.Metadata{
		Revision:          internal.BuildRevision,
		RevisionTimestamp: internal.BuildRevisionTime,
	}

	migrations, err := migrate.RunFS(ctx, sqlDB, migrations.FS, meta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	for _, migration := range migrations {
		fmt.Printf("%d: %s\n", migration.Sequence, migration.Filename)
	}
}
