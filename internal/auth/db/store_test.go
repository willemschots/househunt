package db_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/migrate/testdb"
)

func Test_Tx_CreateUser(t *testing.T) {
	t.Run("ok, create user", inTx(func(t *testing.T, tx auth.Tx) {
		user := testUser(t, nil)

		err := tx.CreateUser(&user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		want := testUser(t, func(u *auth.User) {
			// The store should set the following fields of user.
			u.ID = 1
			u.CreatedAt = now(t, 0)
			u.UpdatedAt = now(t, 0)
		})

		if !reflect.DeepEqual(user, want) {
			t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
		}

		assertFindUser(t, tx, want)
	}))

	t.Run("fail, email constraint violated", inTx(func(t *testing.T, tx auth.Tx) {
		user1 := testUser(t, nil)
		err := tx.CreateUser(&user1)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		user2 := testUser(t, nil)
		err = tx.CreateUser(&user2)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}

	}))

	t.Run("fail, non zero ID", inTx(func(t *testing.T, tx auth.Tx) {
		user := testUser(t, func(u *auth.User) {
			u.ID = 1
		})

		err := tx.CreateUser(&user)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))
}

func Test_Tx_UpdateUser(t *testing.T) {
	t.Run("ok, update user", inTx(func(t *testing.T, tx auth.Tx) {
		user := testUser(t, nil)

		err := tx.CreateUser(&user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		// The user has been saved succesfully.
		// Update all fields that can be modified.
		user.Email = "jacob@example.com"
		user.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
		user.IsActive = true

		err = tx.UpdateUser(&user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		want := testUser(t, func(u *auth.User) {
			u.ID = 1
			u.Email = "jacob@example.com"
			u.PasswordHash = argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")
			u.IsActive = true
			u.CreatedAt = now(t, 0)
			u.UpdatedAt = now(t, 1) // The store should update the UpdatedAt field.
		})

		if !reflect.DeepEqual(user, want) {
			t.Errorf("got\n%#v\nwant\n%#v\n", user, want)
		}

		assertFindUser(t, tx, want)
	}))

	t.Run("fail, not found", inTx(func(t *testing.T, tx auth.Tx) {
		user := testUser(t, func(u *auth.User) {
			u.ID = 1
		})

		err := tx.UpdateUser(&user)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrNotFound, err)
		}
	}))

	t.Run("fail, change email to an existing email", inTx(func(t *testing.T, tx auth.Tx) {
		user1 := testUser(t, nil)
		err := tx.CreateUser(&user1)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		user2 := testUser(t, func(u *auth.User) {
			u.Email = "jacob@example.com"
		})
		err = tx.CreateUser(&user2)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		// Attempt toc hange user1's email to user2's email.
		user1.Email = user2.Email
		err = tx.UpdateUser(&user1)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))
}

func Test_Tx_FindUserByEmail(t *testing.T) {
	// success cases already tested in Test_Tx_SaveUser.

	t.Run("fail, not found", func(t *testing.T) {
		store := storeForTest(t)

		tx, err := store.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}

		_, err = tx.FindUserByEmail("jacob@example.com")
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrNotFound, err)
		}
	})
}

func inTx(f func(*testing.T, auth.Tx)) func(*testing.T) {
	return func(t *testing.T) {
		store := storeForTest(t)

		tx, err := store.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("failed to begin tx: %v", err)
		}

		f(t, tx)

		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit tx: %v", err)
		}
	}
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

func testUser(t *testing.T, modFunc func(*auth.User)) auth.User {
	t.Helper()

	u := auth.User{
		ID:           0,
		Email:        "alice@example.com",
		PasswordHash: argon2Hash(t, "$argon2id$v=19$m=47104,t=1,p=1$vP9U4C5jsOzFQLj0gvUkYw$YLrSb2dGfcVohlm8syynqHs6/NHxXS9rt/t6TjL7pi0"),
		CreatedAt:    time.Time{},
		UpdatedAt:    time.Time{},
	}

	if modFunc != nil {
		modFunc(&u)
	}

	return u
}

func assertFindUser(t *testing.T, tx auth.Tx, want auth.User) {
	t.Helper()

	got, err := tx.FindUserByEmail(want.Email)
	if err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%#v\nwant\n%#v\n", got, want)
	}
}
