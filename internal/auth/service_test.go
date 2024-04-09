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
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/errorz/testerr"
	"github.com/willemschots/househunt/internal/krypto"
)

func Test_Service_RegisterUser(t *testing.T) {
	t.Run("ok, register user", func(t *testing.T) {
		svc, deps := setupService(t)

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		err := svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		svc.Wait()

		// Verify no errors were reported to the error handler.
		deps.errList.assertNoError(t)

		// Assert that an email was send to the email address.
		if len(deps.emailer.emails) != 1 || deps.emailer.emails[0].recipient != credentials.Email {
			t.Fatalf("expected 1 email to %s, got %d", credentials.Email, len(deps.emailer.emails))
		}
	})

	t.Run("ok, re-register non-activated user", func(t *testing.T) {
		svc, deps := setupService(t)

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		// Register once.
		err := svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish.
		svc.Wait()

		// Register again.
		err = svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		svc.Wait()

		// Verify no errors were reported to the error handler.
		deps.errList.assertNoError(t)

		// Assert two single email were send.
		if len(deps.emailer.emails) != 2 {
			t.Fatalf("expected 2 emails, got %d", len(deps.emailer.emails))
		}
	})

	t.Run("fail async, re-register active user", func(t *testing.T) {
		svc, deps := setupService(t)

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		// Register once.
		err := svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		svc.Wait()

		// Activate the user.
		if len(deps.emailer.emails) != 1 {
			t.Fatalf("expected 1 email, got %d", len(deps.emailer.emails))
		}

		request, ok := deps.emailer.emails[0].data.(auth.ActivationRequest)
		if !ok {
			t.Fatalf("unexpected data type: %T", deps.emailer.emails[0].data)
		}

		err = svc.ActivateUser(context.Background(), request)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}

		// Wait for service goroutine to finish activating.
		svc.Wait()

		// Register again.
		err = svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		svc.Wait()

		// Now we should have an error.
		deps.errList.assertErrorIs(t, auth.ErrDuplicateUser)
	})

	// TODO: add case "fail async, too many registration requests"

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 5) {
		t.Run("fail async, store fails", func(t *testing.T) {
			svc, deps := setupService(t)
			deps.store.tracker = &tracker

			credentials := auth.Credentials{
				Email:    must(email.ParseAddress("info@example.com")),
				Password: must(auth.ParsePassword("reallyStrongPassword1")),
			}

			err := svc.RegisterUser(context.Background(), credentials)
			if err != nil {
				t.Fatalf("failed to register user: %v", err)
			}

			// Wait for service goroutine to finish registering.
			svc.Wait()

			deps.errList.assertErrorIs(t, testerr.Err)

			// Assert no email was send.
			if len(deps.emailer.emails) != 0 {
				t.Fatalf("expected 0 emails, got %d", len(deps.emailer.emails))
			}
		})
	}

	t.Run("fail sync, emailer fails", func(t *testing.T) {
		svc, deps := setupService(t)
		deps.emailer.testErr = testerr.Err

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		err := svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		svc.Wait()

		deps.errList.assertErrorIs(t, testerr.Err)
	})
}

func Test_Service_ActivateUser(t *testing.T) {
	registerAndGetRequest := func(t *testing.T, svc *auth.Service, deps *svcDeps) auth.ActivationRequest {
		err := svc.RegisterUser(context.Background(), auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		})
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// wait for the service goroutine to finish registering.
		svc.Wait()

		// Get the last email
		index := len(deps.emailer.emails) - 1
		request, ok := deps.emailer.emails[index].data.(auth.ActivationRequest)
		if !ok {
			t.Fatalf("unexpected data type: %T", deps.emailer.emails[index].data)
		}

		return request
	}

	// setup the test by registering an user and getting the activation request.
	setup := func(t *testing.T) (*auth.Service, *svcDeps, auth.ActivationRequest) {
		svc, deps := setupService(t)

		request := registerAndGetRequest(t, svc, deps)

		return svc, deps, request
	}

	t.Run("ok, activate non-activated user", func(t *testing.T) {
		svc, deps, req := setup(t)

		err := svc.ActivateUser(context.Background(), req)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, non-matching token", func(t *testing.T) {
		svc, deps, req := setup(t)

		req.Token = must(krypto.ParseToken("0102030405060708091011121314151617181920212223242526272829303132"))

		err := svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, non-existant token", func(t *testing.T) {
		svc, deps, req := setup(t)

		req.ID = 2

		err := svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, token already consumed", func(t *testing.T) {
		svc, deps, req := setup(t)

		err := svc.ActivateUser(context.Background(), req)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}
		// wait for the service goroutine to finish activating.
		svc.Wait()

		err = svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, other token used to activate user", func(t *testing.T) {
		svc, deps, req1 := setup(t)

		req2 := registerAndGetRequest(t, svc, deps)

		err := svc.ActivateUser(context.Background(), req2)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}

		// wait for the service goroutine to finish activating.
		svc.Wait()

		// now try with the first token.
		err = svc.ActivateUser(context.Background(), req1)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, expired token", func(t *testing.T) {
		svc, deps, req := setup(t)

		// TokenExpiry is set to 1 hour.
		// Simulate the current time being an hour ahead.
		svc.NowFunc = func() time.Time {
			return time.Now().Add(time.Hour + time.Second)
		}

		err := svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	//t.Run("fail, token for different purpose", func(t *testing.T) {
	//	// TODO: Implement this test.
	//})

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 6) {
		t.Run("fail, store fails", func(t *testing.T) {
			svc, deps, req := setup(t)
			deps.store.tracker = &tracker

			err := svc.ActivateUser(context.Background(), req)
			if !errors.Is(err, testerr.Err) {
				t.Fatalf("expected error %v, got %v (via errors.Is)", testerr.Err, err)
			}

			// assert no async errors were reported.
			svc.Wait()
			deps.errList.assertNoError(t)
		})
	}
}

func Test_Service_Authenticate(t *testing.T) {
	setup := func(t *testing.T, activate bool) (*auth.Service, *svcDeps, auth.Credentials) {
		svc, deps := setupService(t)

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		err := svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		svc.Wait()

		if activate {
			// Get the last email that was sent to retrieve the activation request.
			index := len(deps.emailer.emails) - 1
			request, ok := deps.emailer.emails[index].data.(auth.ActivationRequest)
			if !ok {
				t.Fatalf("unexpected data type: %T", deps.emailer.emails[index].data)
			}

			err = svc.ActivateUser(context.Background(), request)
			if err != nil {
				t.Fatalf("failed to activate user: %v", err)
			}
		}

		return svc, deps, credentials
	}

	t.Run("ok, right credentials", func(t *testing.T) {
		svc, deps, credentials := setup(t, true)

		authenticated, err := svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if !authenticated {
			t.Fatalf("expected authentication to succeed")
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, wrong password", func(t *testing.T) {
		svc, deps, credentials := setup(t, true)

		credentials.Password = must(auth.ParsePassword("wrongPassword"))

		authenticated, err := svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, non-existant user", func(t *testing.T) {
		svc, deps, credentials := setup(t, true)

		credentials.Email = must(email.ParseAddress("jacob@example.com"))

		authenticated, err := svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, inactive user", func(t *testing.T) {
		svc, deps, credentials := setup(t, false)

		authenticated, err := svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
	})

	t.Run("fail, store fails", func(t *testing.T) {
		svc, deps, credentials := setup(t, true)
		failingDeps := testerr.NewFailingDeps(testerr.Err, 1)
		deps.store.tracker = &failingDeps[0]

		_, err := svc.Authenticate(context.Background(), credentials)
		if !errors.Is(err, testerr.Err) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", testerr.Err, err)
		}

		// assert no async errors were reported.
		svc.Wait()
		deps.errList.assertNoError(t)
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

type svcDeps struct {
	store   *testStore
	emailer *testEmailer
	errList *errList
	nowFunc func() time.Time
}

func setupService(t *testing.T) (*auth.Service, *svcDeps) {
	encryptor := must(krypto.NewEncryptor([]krypto.Key{
		must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
	}))

	indexKey := must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf"))

	testDB := testdb.RunWhile(t, true)
	deps := &svcDeps{
		store: &testStore{
			store:   db.New(testDB, testDB, encryptor, indexKey),
			tracker: &testerr.Calltracker{}, // empty call trackers never fail.
		},
		errList: &errList{
			mutex: &sync.Mutex{},
			errs:  make([]error, 0),
		},
		emailer: &testEmailer{},
		nowFunc: func() time.Time {
			return time.Now().Round(0)
		},
	}

	cfg := auth.ServiceConfig{
		WorkerTimeout: time.Second,
		TokenExpiry:   time.Hour,
	}

	svc, err := auth.NewService(deps.store, deps.emailer, deps.errList.AppendErr, cfg)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	return svc, deps
}

// testStore wraps a real store but uses a testerr.Calltracker to
// possibly fail on certain method calls.
type testStore struct {
	store   auth.Store
	tracker *testerr.Calltracker
}

func (f *testStore) BeginTx(ctx context.Context) (auth.Tx, error) {
	return testerr.MaybeFail(f.tracker, func() (auth.Tx, error) {
		realTx, err := f.store.BeginTx(ctx)
		return &testTx{
			store: f,
			tx:    realTx,
		}, err
	})
}

func (f *testStore) FindUsers(ctx context.Context, filter *auth.UserFilter) ([]auth.User, error) {
	return testerr.MaybeFail(f.tracker, func() ([]auth.User, error) {
		return f.store.FindUsers(ctx, filter)
	})
}

type testTx struct {
	store *testStore
	tx    auth.Tx
}

func (tx *testTx) Commit() error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.Commit()
	})
}

func (tx *testTx) Rollback() error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.Rollback()
	})
}

func (tx *testTx) CreateUser(u *auth.User) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.CreateUser(u)
	})
}

func (tx *testTx) UpdateUser(u *auth.User) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.UpdateUser(u)
	})
}

func (tx *testTx) FindUsers(filter *auth.UserFilter) ([]auth.User, error) {
	return testerr.MaybeFail(tx.store.tracker, func() ([]auth.User, error) {
		return tx.tx.FindUsers(filter)
	})
}

func (tx *testTx) CreateEmailToken(t *auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.CreateEmailToken(t)
	})
}

func (tx *testTx) UpdateEmailToken(t *auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.UpdateEmailToken(t)
	})
}

func (tx *testTx) FindEmailTokens(filter *auth.EmailTokenFilter) ([]auth.EmailToken, error) {
	return testerr.MaybeFail(tx.store.tracker, func() ([]auth.EmailToken, error) {
		return tx.tx.FindEmailTokens(filter)
	})
}

type sendEmail struct {
	template  string
	recipient email.Address
	data      interface{}
}

type testEmailer struct {
	emails  []sendEmail
	testErr error
}

func (e *testEmailer) Send(_ context.Context, template string, to email.Address, data interface{}) error {
	e.emails = append(e.emails, sendEmail{
		template:  template,
		recipient: to,
		data:      data,
	})

	return e.testErr
}
