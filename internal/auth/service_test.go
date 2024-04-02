package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/willemschots/househunt/assets"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/db/testdb"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/email/view"
	"github.com/willemschots/househunt/internal/errorz/testerr"
)

func Test_Service_RegisterAccount(t *testing.T) {
	setup := func(t *testing.T) (*auth.Service, *email.MemorySender, *errList) {
		store := db.New(testdb.RunWhile(t, true), func() time.Time {
			return time.Now().Round(0)
		})

		errs := &errList{
			mutex: &sync.Mutex{},
			errs:  make([]error, 0),
		}

		emailSvc, sender := emailSvc()
		svc := auth.NewService(store, emailSvc, errs.AppendErr, 100*time.Second)

		return svc, sender, errs
	}

	setupFailingStore := func(t *testing.T, dep *testerr.FailingDep) (*auth.Service, *email.MemorySender, *errList) {
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

		emailSvc, sender := emailSvc()
		svc := auth.NewService(store, emailSvc, errs.AppendErr, time.Second)

		return svc, sender, errs
	}

	t.Run("ok, register account", func(t *testing.T) {
		svc, sender, errs := setup(t)

		credentials := auth.Credentials{
			Email:    emailAddress(t, "test@example.com"),
			Password: password(t, "reallyStrongPassword1"),
		}
		err := svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Wait for service goroutine to finish.
		svc.Wait()

		// Verify no errors were reported to the error handler.
		errs.assertNoError(t)

		// Assert that an email was send to the email address.
		if len(sender.Emails) != 1 || sender.Emails[0].Recipient != credentials.Email {
			t.Fatalf("expected 1 email to %s, got %d", credentials.Email, len(sender.Emails))
		}
	})

	t.Run("ok, re-register non-activated account", func(t *testing.T) {
		svc, sender, errs := setup(t)

		credentials := auth.Credentials{
			Email:    emailAddress(t, "test@example.com"),
			Password: password(t, "reallyStrongPassword1"),
		}

		// Register once.
		err := svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Wait for service goroutine to finish.
		svc.Wait()

		// Register again.
		err = svc.RegisterAccount(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register account: %v", err)
		}

		// Wait for service goroutine to finish.
		svc.Wait()

		// Verify no errors were reported to the error handler.
		errs.assertNoError(t)

		// Assert two single email were send.
		if len(sender.Emails) != 2 {
			t.Fatalf("expected 2 emails, got %d", len(sender.Emails))
		}
	})

	// TODO: Add test "fail, async re-register activated account"
	// TODO: add case "fail, async, too many registration requests"

	for _, dep := range testerr.NewFailingDeps(testerr.Err, 5) {
		t.Run("fail, store fails", func(t *testing.T) {
			svc, sender, errs := setupFailingStore(t, &dep)

			credentials := auth.Credentials{
				Email:    emailAddress(t, "test@example.com"),
				Password: password(t, "reallyStrongPassword1"),
			}
			err := svc.RegisterAccount(context.Background(), credentials)
			if err != nil {
				t.Fatalf("failed to register account: %v", err)
			}

			// Wait for service goroutine to finish.
			svc.Wait()

			errs.assertErrorIs(t, testerr.Err)

			// Assert no email was send.
			if len(sender.Emails) != 0 {
				t.Fatalf("expected 0 emails, got %d", len(sender.Emails))
			}
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

func emailSvc() (*email.Service, *email.MemorySender) {
	sender := &email.MemorySender{}
	renderer := view.NewFSRenderer(assets.EmailFS)
	emailSvc := email.NewService(email.Address("sender@example.com"), renderer, sender)
	return emailSvc, sender
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

func (f *failingTx) FindUsers(filter *auth.UserFilter) ([]auth.User, error) {
	return testerr.MaybeFail(f.store.dep, func() ([]auth.User, error) {
		return f.tx.FindUsers(filter)
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
