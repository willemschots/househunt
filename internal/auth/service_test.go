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
)

func Test_Service_RegisterAccount(t *testing.T) {
	setup := func(t *testing.T) (*auth.Service, *errList) {
		testDB := db.New(testdb.RunWhile(t, true), func() time.Time {
			return time.Now().Round(0)
		})

		errs := &errList{
			mutex: &sync.Mutex{},
			errs:  make([]error, 0),
		}
		svc := auth.NewService(testDB, errs.AppendErr, time.Second)

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
