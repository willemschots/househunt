package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/db/testdb"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz/testerr"
)

func Test_Service_RegisterAccount(t *testing.T) {
	setup := func(t *testing.T) (*auth.Service, *errList) {
		store := db.New(testdb.RunWhile(t, true), func() time.Time {
			return time.Now().Round(0)
		})

		errs := &errList{
			mutex: &sync.Mutex{},
			errs:  make([]error, 0),
		}

		svc := auth.NewService(store, errs.AppendErr, time.Second)

		return svc, errs
	}

	setupFailingStore := func(t *testing.T, dep *testerr.FailingDep) (*auth.Service, *errList) {
		store := &failingStore{
			store: db.New(testdb.RunWhile(t, true), func() time.Time {
				return time.Now().Round(0)
			}),
			dep: dep,
		}

		errs := &errList{
			mutex: &sync.Mutex{},
			errs:  make([]error, 0),
		}

		svc := auth.NewService(store, errs.AppendErr, time.Second)

		return svc, errs

	}

	t.Run("ok, register account", func(t *testing.T) {
		svc, errs := setup(t)

		credentials := auth.Credentials{
			Email:    emailAddress(t, "test@example.com"),
			Password: password(t, "reallyStrongPassword1"),
		}
		err := svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Wait for service goroutines to finish.
		svc.Close()

		errs.assertNoError(t)

		// TODO: Assert that an email was send to the email address.
	})

	t.Run("fail async, register account with same email", func(t *testing.T) {
		svc, errs := setup(t)

		credentials := auth.Credentials{
			Email:    emailAddress(t, "test@example.com"),
			Password: password(t, "reallyStrongPassword1"),
		}
		err := svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Register again, notice this outputs no error.
		err = svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Wait for service goroutines to finish.
		svc.Close()

		// However, it does output an error to the err handler.
		errs.assertErrorIs(t, auth.ErrDuplicateAccount)

		// TODO: Assert only a single email was send.
	})

	for _, dep := range testerr.NewFailingDeps(testerr.Err, 5) {
		t.Run("fail, dependency fails", func(t *testing.T) {
			svc, errs := setupFailingStore(t, &dep)

			credentials := auth.Credentials{
				Email:    emailAddress(t, "test@example.com"),
				Password: password(t, "reallyStrongPassword1"),
			}
			err := svc.RegisterAccount(context.Background(), credentials)
			if err != nil {
				t.Fatalf("failed to register account: %v", err)
			}

			// Wait for service goroutines to finish.
			svc.Close()

			errs.assertErrorIs(t, testerr.Err)

			// TODO: Assert no email was sent.
		})
	}
}

type errList struct {
	mutex *sync.Mutex
	errs  []error
}

func (e *errList) AppendErr(err error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.errs == nil {
		e.errs = make([]error, 0)
	}
	e.errs = append(e.errs, err)
}

func (e *errList) assertNoError(t *testing.T) {
	t.Helper()

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if len(e.errs) > 0 {
		t.Fatalf("unexpected errors: %v", e.errs)
	}
}

func (e *errList) assertErrorIs(t *testing.T, err error) {
	t.Helper()

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if len(e.errs) != 1 || !errors.Is(e.errs[0], err) {
		t.Fatalf("expected error %v, got %v via errors.Is()", err, e.errs)
	}
}

func emailAddress(t *testing.T, raw string) email.Address {
	t.Helper()

	e, err := email.ParseAddress(raw)
	if err != nil {
		t.Fatalf("failed to parse email: %v", err)
	}

	return e
}

func password(t *testing.T, raw string) auth.Password {
	t.Helper()

	p, err := auth.ParsePassword(raw)
	if err != nil {
		t.Fatalf("failed to parse password: %v", err)
	}

	return p
}

// failingStore wraps a real store but fails on specific calls.
//
// The failure is controlled by a FailingDep. They either fail
// on the xth call, or on the xth call and all calls after that.
type failingStore struct {
	store auth.Store
	dep   *testerr.FailingDep
}

func (f *failingStore) BeginTx(ctx context.Context) (auth.Tx, error) {
	return testerr.MaybeFail(f.dep, func() (auth.Tx, error) {
		realTx, err := f.store.BeginTx(ctx)
		return &failingTx{
			store: f,
			tx:    realTx,
		}, err
	})
}

type failingTx struct {
	store *failingStore
	tx    auth.Tx
}

func (f *failingTx) Commit() error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.Commit()
	})
}

func (f *failingTx) Rollback() error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.Rollback()
	})
}

func (f *failingTx) CreateUser(u *auth.User) error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.CreateUser(u)
	})
}

func (f *failingTx) UpdateUser(u *auth.User) error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.UpdateUser(u)
	})
}

func (f *failingTx) FindUserByEmail(v email.Address) (auth.User, error) {
	return testerr.MaybeFail(f.store.dep, func() (auth.User, error) {
		return f.tx.FindUserByEmail(v)
	})
}

func (f *failingTx) CreateEmailToken(t *auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.CreateEmailToken(t)
	})
}

func (f *failingTx) UpdateEmailToken(t *auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(f.store.dep, func() error {
		return f.tx.UpdateEmailToken(t)
	})
}

func (f *failingTx) FindEmailTokenByID(id int) (auth.EmailToken, error) {
	return testerr.MaybeFail(f.store.dep, func() (auth.EmailToken, error) {
		return f.tx.FindEmailTokenByID(id)
	})
}
