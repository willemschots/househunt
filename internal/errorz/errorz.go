package errorz

import (
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrConstraintViolated = errors.New("constraint violated")
)

// MapDBErr maps database errors to appropriate errorz errors.
func MapDBErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	sErr := sqlite3.Error{}
	if errors.As(err, &sErr) {
		if sErr.Code == sqlite3.ErrConstraint {
			return ErrConstraintViolated
		}
	}

	return err
}
