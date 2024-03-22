package db_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/migrate/testdb"
)

func Test_Tx_SaveUser(t *testing.T) {
	t.Run("ok, in same tx", func(t *testing.T) {
		store := storeForTest(t)

		tx, err := store.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}

		user := newUser(t)

		// Save the user for a first time.
		t.Run("save new user", func(t *testing.T) {
			err := tx.SaveUser(&user)
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}

			want := newUser(t)
			// The store should set the following fields of user.
			want.ID = 1
			want.CreatedAt = now(t, 0)
			want.UpdatedAt = now(t, 0)

			if !reflect.DeepEqual(user, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
			}
		})

		// The user has been saved succesfully.
		// Update all fields that can be modified.
		user.Email = "jacob@example.com"
		user.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
		user.IsActive = true

		// And save the user again.
		t.Run("save user again", func(t *testing.T) {
			err := tx.SaveUser(&user)
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}

			want := newUser(t)
			want.ID = 1
			want.Email = "jacob@example.com"
			want.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
			want.IsActive = true
			want.CreatedAt = now(t, 0)
			want.UpdatedAt = now(t, 1) // The store should update the UpdatedAt field.

			if !reflect.DeepEqual(user, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
			}
		})

		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit tx: %v", err)
		}
	})

	t.Run("ok, in seperate tx", func(t *testing.T) {
		store := storeForTest(t)

		user := newUser(t)

		t.Run("save new user", func(t *testing.T) {
			tx, err := store.BeginTx(context.Background())
			if err != nil {
				t.Fatalf("failed to begin tx: %v", err)
			}

			err = tx.SaveUser(&user)
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}

			want := newUser(t)
			want.ID = 1
			want.CreatedAt = now(t, 0)
			want.UpdatedAt = now(t, 0)

			if !reflect.DeepEqual(user, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatalf("failed to commit tx: %v", err)
			}
		})

		// Update all fields that can be modified.
		user.Email = "jacob@example.com"
		user.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
		user.IsActive = true

		t.Run("save user again", func(t *testing.T) {
			tx, err := store.BeginTx(context.Background())
			if err != nil {
				t.Fatalf("failed to begin tx: %v", err)
			}

			err = tx.SaveUser(&user)
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}

			want := newUser(t)
			want.ID = 1
			want.Email = "jacob@example.com"
			want.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
			want.IsActive = true
			want.CreatedAt = now(t, 0)
			want.UpdatedAt = now(t, 1)

			if !reflect.DeepEqual(user, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatalf("failed to commit tx: %v", err)
			}
		})
	})

	t.Run("fail, need to have database create ID", func(t *testing.T) {
		store := storeForTest(t)

		tx, err := store.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}

		user := newUser(t)
		user.ID = 1 // The ID is already set, but this user has not been saved before.

		err = tx.SaveUser(&user)
		if err == nil {
			t.Fatalf("expected an error, but got nil")
		}
	})
}

func now(t *testing.T, i int) time.Time {
	t.Helper()
	if i > 9 {
		t.Fatalf("invalid time index: %d", i)
	}

	ts, err := time.Parse(time.RFC3339, fmt.Sprintf("2021-01-01T00:00:0%dZ", i))
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}

	return ts
}

func storeForTest(t *testing.T) *db.Store {
	t.Helper()

	testDB := testdb.RunTestDB(t)

	i := 0
	return db.New(testDB, func() time.Time {
		n := now(t, i)
		i++
		return n
	})
}

func argon2Hash(t *testing.T, raw string) auth.Argon2Hash {
	t.Helper()

	hash, err := auth.ParseArgon2Hash(raw)
	if err != nil {
		t.Fatalf("failed to parse hash: %v", err)
	}

	return hash
}

func newUser(t *testing.T) db.User {
	t.Helper()

	return db.User{
		ID:           0,
		Email:        "alice@example.com",
		PasswordHash: argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$vP9U4C5jsOzFQLj0gvUkYw$YLrSb2dGfcVohlm8syynqHs6/NHxXS9rt/t6TjL7pi0"),
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}
}
