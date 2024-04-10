package db_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/db/testdb"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/krypto"
)

func Test_Tx_CreateUser(t *testing.T) {
	t.Run("ok, create user", inTx(func(t *testing.T, tx auth.Tx) {
		user := newUser(t, nil)

		err := tx.CreateUser(user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		assertFindUser(t, tx, user)
	}))

	t.Run("fail, email constraint violated", inTx(func(t *testing.T, tx auth.Tx) {
		user1 := newUser(t, nil)
		err := tx.CreateUser(user1)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		user2 := newUser(t, func(u *auth.User) {
			u.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
		})
		err = tx.CreateUser(user2)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))

	t.Run("fail, zero ID", inTx(func(t *testing.T, tx auth.Tx) {
		user := newUser(t, func(u *auth.User) {
			u.ID = uuid.Nil
		})

		err := tx.CreateUser(user)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))
}

func Test_Tx_UpdateUser(t *testing.T) {
	setup := func(t *testing.T, tx auth.Tx) auth.User {
		user := newUser(t, nil)
		err := tx.CreateUser(user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		return user
	}

	t.Run("ok, update user", inTx(func(t *testing.T, tx auth.Tx) {
		user := setup(t, tx)

		// Update all fields that can be modified.
		user.Email = must(email.ParseAddress("jacob@example.com"))
		user.PasswordHash = must(krypto.ParseArgon2Hash("$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU"))
		user.IsActive = true
		user.CreatedAt = now(t, 1)
		user.UpdatedAt = now(t, 2)

		err := tx.UpdateUser(user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		assertFindUser(t, tx, user)
	}))

	t.Run("fail, not found", inTx(func(t *testing.T, tx auth.Tx) {
		setup(t, tx)

		user2 := newUser(t, func(u *auth.User) {
			u.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
		})

		err := tx.UpdateUser(user2)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrNotFound, err)
		}
	}))

	t.Run("fail, change email to an existing email", inTx(func(t *testing.T, tx auth.Tx) {
		user1 := setup(t, tx)

		user2 := newUser(t, func(u *auth.User) {
			u.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
			u.Email = must(email.ParseAddress("jacob@example.com"))
		})

		err := tx.CreateUser(user2)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		// Attempt to change user1's email to user2's email.
		user1.Email = user2.Email
		err = tx.UpdateUser(user1)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))
}

func Test_Tx_FindUser(t *testing.T) {
	setupUsers := func(t *testing.T, tx auth.Tx) []auth.User {
		users := []auth.User{
			newUser(t, nil),
			newUser(t, func(u *auth.User) {
				u.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
				u.Email = must(email.ParseAddress("jacob@example.com"))
				u.IsActive = true
			}),
			newUser(t, func(u *auth.User) {
				u.ID = must(uuid.Parse("d622d0b0-465c-4c4d-b084-028c9787e1de"))
				u.Email = must(email.ParseAddress("eva@example.com"))
			}),
		}

		for i := range users {
			err := tx.CreateUser(users[i])
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}
		}

		return users
	}

	tests := map[string]struct {
		filter   auth.UserFilter
		wantFunc func([]auth.User) []auth.User
	}{
		"ok, all users, empty slices": {
			filter: auth.UserFilter{
				IDs:      []uuid.UUID{},
				Emails:   []email.Address{},
				IsActive: nil,
			},
			wantFunc: func(users []auth.User) []auth.User {
				return users
			},
		},
		"ok, active users": {
			filter: auth.UserFilter{
				IsActive: ptr(true),
			},
			wantFunc: func(users []auth.User) []auth.User {
				return users[1:2]
			},
		},
		"ok, one by id": {
			filter: auth.UserFilter{
				IDs: []uuid.UUID{must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))},
			},
			wantFunc: func(users []auth.User) []auth.User {
				return []auth.User{users[1]}
			},
		},
		"ok, several by id": {
			filter: auth.UserFilter{
				IDs: []uuid.UUID{
					must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
					must(uuid.Parse("d622d0b0-465c-4c4d-b084-028c9787e1de")),
				},
			},
			wantFunc: func(users []auth.User) []auth.User {
				return []auth.User{
					users[0], users[2],
				}
			},
		},
		"ok, one by email": {
			filter: auth.UserFilter{
				Emails: []email.Address{
					must(email.ParseAddress("jacob@example.com")),
				},
			},
			wantFunc: func(users []auth.User) []auth.User {
				return []auth.User{users[1]}
			},
		},
		"ok, several by email": {
			filter: auth.UserFilter{
				Emails: []email.Address{
					must(email.ParseAddress("jacob@example.com")),
					must(email.ParseAddress("eva@example.com")),
				},
			},
			wantFunc: func(users []auth.User) []auth.User {
				return []auth.User{
					users[1], users[2],
				}
			},
		},
		"ok, combine filters": {
			filter: auth.UserFilter{
				IDs: []uuid.UUID{
					must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
					must(uuid.Parse("d622d0b0-465c-4c4d-b084-028c9787e1de")),
				},
				Emails: []email.Address{
					must(email.ParseAddress("alice@example.com")),
				},
				IsActive: ptr(false),
			},
			wantFunc: func(users []auth.User) []auth.User {
				return users[0:1]
			},
		},
		"ok, no results": {
			filter: auth.UserFilter{
				IDs: []uuid.UUID{uuid.Nil},
			},
			wantFunc: func(users []auth.User) []auth.User {
				return []auth.User{}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store := storeForTest(t)

			tx, err := store.BeginTx(context.Background())
			if err != nil {
				t.Fatalf("failed to begin tx: %v", err)
			}

			users := setupUsers(t, tx)
			want := tc.wantFunc(users)

			// first check if FindUsers works on the tx
			got, err := tx.FindUsers(tc.filter)
			if err != nil {
				t.Fatalf("failed to find users: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, want)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatalf("failed to commit tx: %v", err)
			}

			// then, check if FindUsers works on the store itself.
			got, err = store.FindUsers(context.Background(), tc.filter)
			if err != nil {
				t.Fatalf("failed to find users: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, want)
			}
		})
	}
}

func Test_Tx_CreateEmailToken(t *testing.T) {
	setup := func(t *testing.T, tx auth.Tx) auth.User {
		user := newUser(t, nil)
		err := tx.CreateUser(user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		return user
	}

	t.Run("ok, create email token", inTx(func(t *testing.T, tx auth.Tx) {
		_ = setup(t, tx)

		token := newEmailToken(t, nil)

		err := tx.CreateEmailToken(token)
		if err != nil {
			t.Fatalf("failed to save email token: %v", err)
		}

		assertFindEmailToken(t, tx, token)
	}))

	t.Run("fail, user foreign key does not exist", inTx(func(t *testing.T, tx auth.Tx) {
		setup(t, tx)

		token := newEmailToken(t, func(tok *auth.EmailToken) {
			tok.UserID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
		})

		err := tx.CreateEmailToken(token)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))

	t.Run("fail, zero ID", inTx(func(t *testing.T, tx auth.Tx) {
		setup(t, tx)

		token := newEmailToken(t, func(tok *auth.EmailToken) {
			tok.ID = uuid.Nil
		})

		err := tx.CreateEmailToken(token)
		if !errors.Is(err, errorz.ErrConstraintViolated) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrConstraintViolated, err)
		}
	}))
}

func Test_Tx_UpdateEmailToken(t *testing.T) {
	setup := func(t *testing.T, tx auth.Tx) (auth.User, auth.EmailToken) {
		user := newUser(t, nil)
		err := tx.CreateUser(user)
		if err != nil {
			t.Fatalf("failed to save user: %v", err)
		}

		token := newEmailToken(t, nil)
		err = tx.CreateEmailToken(token)
		if err != nil {
			t.Fatalf("failed to save email token: %v", err)
		}

		return user, token
	}

	t.Run("ok, update email token", inTx(func(t *testing.T, tx auth.Tx) {
		_, token := setup(t, tx)

		// TODO: Change other fields.
		consumedAt := now(t, 9)
		token.ConsumedAt = &consumedAt

		err := tx.UpdateEmailToken(token)
		if err != nil {
			t.Fatalf("failed to save email token: %v", err)
		}

		assertFindEmailToken(t, tx, token)
	}))

	t.Run("fail, not found", inTx(func(t *testing.T, tx auth.Tx) {
		_, token := setup(t, tx)

		token.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
		err := tx.UpdateEmailToken(token)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected errors to be %v got %v (via errors.Is)", errorz.ErrNotFound, err)
		}
	}))
}

func Test_Tx_FinderEmailTokens(t *testing.T) {
	setupEmailTokens := func(t *testing.T, tx auth.Tx) []auth.EmailToken {
		users := []auth.User{
			newUser(t, nil),
			newUser(t, func(u *auth.User) {
				u.ID = must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930"))
				u.Email = must(email.ParseAddress("jacob@example.com"))
			}),
		}

		for i := range users {
			err := tx.CreateUser(users[i])
			if err != nil {
				t.Fatalf("failed to save user: %v", err)
			}
		}

		tokens := []auth.EmailToken{
			newEmailToken(t, nil),
			newEmailToken(t, func(tok *auth.EmailToken) {
				tok.ID = must(uuid.Parse("4516a1c0-efc3-4561-9e97-e749e008aa3f"))
			}),
			newEmailToken(t, func(tok *auth.EmailToken) {
				tok.ID = must(uuid.Parse("b7d2b72f-e20b-4f6d-abae-ec90ff36553a"))
				tok.UserID = users[1].ID
				tok.Purpose = auth.TokenPurposePasswordReset
				now := now(t, 9)
				tok.ConsumedAt = &now
			}),
		}

		for i := range tokens {
			err := tx.CreateEmailToken(tokens[i])
			if err != nil {
				t.Fatalf("failed to save email token: %v", err)
			}
		}

		return tokens
	}

	tests := map[string]struct {
		filter   auth.EmailTokenFilter
		wantFunc func([]auth.EmailToken) []auth.EmailToken
	}{
		"ok, all email tokens, empty slices": {
			filter: auth.EmailTokenFilter{
				IDs:        []uuid.UUID{},
				UserIDs:    []uuid.UUID{},
				Purposes:   []auth.TokenPurpose{},
				IsConsumed: nil,
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens
			},
		},
		"ok, unconsumed": {
			filter: auth.EmailTokenFilter{
				IsConsumed: ptr(false),
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[0:2]
			},
		},
		"ok, consumed": {
			filter: auth.EmailTokenFilter{
				IsConsumed: ptr(true),
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[2:3]
			},
		},
		"ok, one by id": {
			filter: auth.EmailTokenFilter{
				IDs: []uuid.UUID{must(uuid.Parse("4516a1c0-efc3-4561-9e97-e749e008aa3f"))},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return []auth.EmailToken{tokens[1]}
			},
		},
		"ok, several by id": {
			filter: auth.EmailTokenFilter{
				IDs: []uuid.UUID{
					must(uuid.Parse("42bf8943-2ffc-43d9-8682-ca8fc4d7cb8e")),
					must(uuid.Parse("b7d2b72f-e20b-4f6d-abae-ec90ff36553a")),
				},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return []auth.EmailToken{
					tokens[0], tokens[2],
				}
			},
		},
		"ok, one by user id": {
			filter: auth.EmailTokenFilter{
				UserIDs: []uuid.UUID{
					must(uuid.Parse("597228ee-afde-4991-b13c-0161325e3930")),
				},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[2:3]
			},
		},
		"ok, several by user id": {
			filter: auth.EmailTokenFilter{
				UserIDs: []uuid.UUID{
					must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
				},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[0:2]
			},
		},
		"ok, one by purpose": {
			filter: auth.EmailTokenFilter{
				Purposes: []auth.TokenPurpose{auth.TokenPurposePasswordReset},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[2:3]
			},
		},
		"ok, several by purpose": {
			filter: auth.EmailTokenFilter{
				Purposes: []auth.TokenPurpose{auth.TokenPurposeActivate},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[0:2]
			},
		},
		"ok, combine filters": {
			filter: auth.EmailTokenFilter{
				IDs: []uuid.UUID{
					must(uuid.Parse("4516a1c0-efc3-4561-9e97-e749e008aa3f")),
					must(uuid.Parse("b7d2b72f-e20b-4f6d-abae-ec90ff36553a")),
				},
				UserIDs: []uuid.UUID{
					must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
				},
				Purposes: []auth.TokenPurpose{auth.TokenPurposeActivate},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return tokens[1:2]
			},
		},
		"ok, no results": {
			filter: auth.EmailTokenFilter{
				IDs: []uuid.UUID{uuid.Nil},
			},
			wantFunc: func(tokens []auth.EmailToken) []auth.EmailToken {
				return []auth.EmailToken{}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store := storeForTest(t)

			tx, err := store.BeginTx(context.Background())
			if err != nil {
				t.Fatalf("failed to begin tx: %v", err)
			}

			tokens := setupEmailTokens(t, tx)
			want := tc.wantFunc(tokens)

			got, err := tx.FindEmailTokens(tc.filter)
			if err != nil {
				t.Fatalf("failed to find email tokens: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, want)
			}
		})
	}
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

	encryptor := must(krypto.NewEncryptor([]krypto.Key{
		must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
	}))
	indexKey := must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf"))

	testDB := testdb.RunWhile(t, true)
	return db.New(testDB, testDB, encryptor, indexKey)
}

func newUser(t *testing.T, modFunc func(*auth.User)) auth.User {
	t.Helper()

	u := auth.User{
		ID:           must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
		Email:        must(email.ParseAddress("alice@example.com")),
		PasswordHash: must(krypto.ParseArgon2Hash("$argon2id$v=19$m=47104,t=1,p=1$vP9U4C5jsOzFQLj0gvUkYw$YLrSb2dGfcVohlm8syynqHs6/NHxXS9rt/t6TjL7pi0")),
		CreatedAt:    now(t, 0),
		UpdatedAt:    now(t, 0),
	}

	if modFunc != nil {
		modFunc(&u)
	}

	return u
}

func newEmailToken(t *testing.T, modFunc func(*auth.EmailToken)) auth.EmailToken {
	t.Helper()

	tok := auth.EmailToken{
		ID:         must(uuid.Parse("42bf8943-2ffc-43d9-8682-ca8fc4d7cb8e")),
		TokenHash:  must(krypto.ParseArgon2Hash("$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU")),
		UserID:     must(uuid.Parse("0e61a06e-bbf6-4b87-aaaa-75fee0f38cca")),
		Email:      must(email.ParseAddress("alice@example.com")),
		Purpose:    auth.TokenPurposeActivate,
		CreatedAt:  now(t, 1),
		ConsumedAt: nil,
	}

	if modFunc != nil {
		modFunc(&tok)
	}

	return tok
}

func assertFindUser(t *testing.T, tx auth.Tx, want auth.User) {
	t.Helper()

	got, err := tx.FindUsers(auth.UserFilter{IDs: []uuid.UUID{want.ID}})
	if err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 user, got %d", len(got))
	}

	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got\n%#v\nwant\n%#v\n", got[0], want)
	}
}

func assertFindEmailToken(t *testing.T, tx auth.Tx, want auth.EmailToken) {
	t.Helper()

	got, err := tx.FindEmailTokens(auth.EmailTokenFilter{IDs: []uuid.UUID{want.ID}})
	if err != nil {
		t.Fatalf("failed to find email token: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 email token, got %d", len(got))
	}

	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got\n%#v\nwant\n%#v\n", got[0], want)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
