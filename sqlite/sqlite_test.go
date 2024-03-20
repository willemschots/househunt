package sqlite

import (
	"database/sql"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

const rows = 500

type row struct {
	id       int
	text     string
	moreText string
	nr1      int
	nr2      int
	nr3      float64
}

var data = []row{}

func init() {
	data = make([]row, 0, rows)
	for i := 0; i < rows; i++ {
		data = append(data, row{
			text:     randString(128),
			moreText: randString(512),
			nr1:      rand.Intn(math.MaxInt64),
			nr2:      rand.Intn(math.MaxInt64),
			nr3:      rand.Float64(),
		})
	}
}

func Test_ModernC(t *testing.T) {
	runTests(t, "sqlite", "./modernc.db")
}

func Test_CGO(t *testing.T) {
	runTests(t, "sqlite3", "./cgo.db")
}

func runTests(t *testing.T, driver, file string) {
	db := setupDB(t, driver, file)

	start := time.Now()

	for i := 0; i < rows; i++ {
		r := data[i]
		_, err := db.Exec("INSERT INTO test_data (text, more_text, nr1, nr2, nr3) VALUES (?, ?, ?, ?, ?)", r.text, r.moreText, r.nr1, r.nr2, r.nr3)
		if err != nil {
			t.Fatalf("failed to insert data: %v", err)
		}
	}

	t.Logf("%s writes: %v", driver, time.Since(start))

	start = time.Now()

	for i := 0; i < rows; i++ {
		var where string
		var param any
		switch i % 3 {
		case 0:
			where = "id = ?"
			param = i + 1
		case 1:
			where = "text = ?"
			param = data[i].text
		case 2:
			where = "nr1 = ?"
			param = data[i].nr1
		}

		stmt, err := db.Prepare("SELECT id, text, more_text, nr1, nr2, nr3 FROM test_data WHERE " + where)
		if err != nil {
			t.Fatalf("failed to prepare query: %v", err)
		}

		rows, err := stmt.Query(param)
		if err != nil {
			t.Fatalf("failed to query data: %v", err)
		}

		for rows.Next() {
			r := row{}
			err := rows.Scan(&r.id, &r.text, &r.moreText, &r.nr1, &r.nr2, &r.nr3)
			if err != nil {
				t.Fatalf("failed to scan data: %v", err)
			}
		}

		if err = rows.Err(); err != nil {
			t.Fatalf("failed to read data: %v", err)
		}
	}

	t.Logf("%s reads: %v", driver, time.Since(start))
}

func setupDB(t *testing.T, driver, file string) *sql.DB {
	t.Helper()

	db, err := sql.Open(driver, file)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	t.Cleanup(func() {
		err := db.Close()
		if err != nil {
			t.Errorf("failed to close db: %v", err)
		}

		err = os.Remove(file)
		if err != nil {
			t.Errorf("failed to remove db: %v", err)
		}
	})

	createTestTable(t, db)

	return db
}

func createTestTable(t *testing.T, db *sql.DB) {
	t.Helper()

	const q = `CREATE TABLE test_data (
	id INTEGER PRIMARY KEY,
	text TEXT,
	more_text TEXT,
	nr1 INTEGER,
	nr2 INTEGER,
	nr3 REAL
)`

	_, err := db.Exec(q)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

func randString(nr int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	out := make([]byte, nr)
	for i := 0; i < nr; i++ {
		out[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(out)
}
