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
		st := newServiceTest(t)

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		st.svc.Wait()

		// Verify no errors were reported to the error handler.
		st.errList.assertNoError(t)

		// Assert that an email was send to the email address.
		if len(st.emailer.emails) != 1 || st.emailer.emails[0].recipient != credentials.Email {
			t.Fatalf("expected 1 email to %s, got %d", credentials.Email, len(st.emailer.emails))
		}
	})

	t.Run("ok, re-register non-activated user", func(t *testing.T) {
		st := newServiceTest(t)

		// Register an initial user, but don't activate it.
		credentials, _ := st.registerUser()

		// Register again.
		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		st.svc.Wait()

		// Verify no errors were reported to the error handler.
		st.errList.assertNoError(t)

		// Assert two emails were send.
		if len(st.emailer.emails) != 2 {
			t.Fatalf("expected 2 emails, got %d", len(st.emailer.emails))
		}
	})

	t.Run("fail async, re-register active user", func(t *testing.T) {
		st := newServiceTest(t)

		// Register an initial user and activate it.
		credentials, request := st.registerUser()
		st.activateUser(request)

		// Register again.
		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		st.svc.Wait()

		// Now we should have an error.
		st.errList.assertErrorIs(t, auth.ErrDuplicateUser)
	})

	// TODO: add case "fail async, too many registration requests"

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 5) {
		t.Run("fail async, store fails", func(t *testing.T) {
			st := newServiceTest(t)
			st.store.tracker = &tracker

			credentials := auth.Credentials{
				Email:    must(email.ParseAddress("info@example.com")),
				Password: must(auth.ParsePassword("reallyStrongPassword1")),
			}

			err := st.svc.RegisterUser(context.Background(), credentials)
			if err != nil {
				t.Fatalf("failed to register user: %v", err)
			}

			// Wait for service goroutine to finish registering.
			st.svc.Wait()

			st.errList.assertErrorIs(t, testerr.Err)

			// Assert no email was send.
			if len(st.emailer.emails) != 0 {
				t.Fatalf("expected 0 emails, got %d", len(st.emailer.emails))
			}
		})
	}

	t.Run("fail sync, emailer fails", func(t *testing.T) {
		st := newServiceTest(t)
		st.emailer.testErr = testerr.Err

		credentials := auth.Credentials{
			Email:    must(email.ParseAddress("info@example.com")),
			Password: must(auth.ParsePassword("reallyStrongPassword1")),
		}

		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		st.svc.Wait()

		st.errList.assertErrorIs(t, testerr.Err)
	})
}

func Test_Service_ActivateUser(t *testing.T) {
	t.Run("ok, activate non-activated user", func(t *testing.T) {
		st := newServiceTest(t)
		_, req := st.registerUser()

		err := st.svc.ActivateUser(context.Background(), req)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, non-matching token", func(t *testing.T) {
		st := newServiceTest(t)
		_, req := st.registerUser()

		req.Token = must(krypto.ParseToken("0102030405060708091011121314151617181920212223242526272829303132"))

		err := st.svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, non-existant token", func(t *testing.T) {
		st := newServiceTest(t)
		_, req := st.registerUser()

		req.ID = 2

		err := st.svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, token already consumed", func(t *testing.T) {
		st := newServiceTest(t)
		_, req := st.registerUser()

		// Consume token once.
		st.activateUser(req)

		// Consume token again.
		err := st.svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, other token used to activate user", func(t *testing.T) {
		st := newServiceTest(t)

		// register same user twice.
		_, req1 := st.registerUser()
		_, req2 := st.registerUser()

		// activate user with the second token.
		st.activateUser(req2)

		// now try with the first token.
		err := st.svc.ActivateUser(context.Background(), req1)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, expired token", func(t *testing.T) {
		st := newServiceTest(t)
		_, req := st.registerUser()

		// TokenExpiry is set to 1 hour.
		// Simulate the current time being an hour ahead.
		st.svc.NowFunc = func() time.Time {
			return time.Now().Add(time.Hour + time.Second)
		}

		err := st.svc.ActivateUser(context.Background(), req)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	//t.Run("fail, token for different purpose", func(t *testing.T) {
	//	// TODO: Implement this test.
	//})

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 6) {
		t.Run("fail, store fails", func(t *testing.T) {
			st := newServiceTest(t)
			_, req := st.registerUser()
			st.store.tracker = &tracker

			err := st.svc.ActivateUser(context.Background(), req)
			if !errors.Is(err, testerr.Err) {
				t.Fatalf("expected error %v, got %v (via errors.Is)", testerr.Err, err)
			}

			// assert no async errors were reported.
			st.svc.Wait()
			st.errList.assertNoError(t)
		})
	}
}

func Test_Service_Authenticate(t *testing.T) {
	t.Run("ok, right credentials", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, req := st.registerUser()
		st.activateUser(req)

		authenticated, err := st.svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if !authenticated {
			t.Fatalf("expected authentication to succeed")
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, wrong password", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, req := st.registerUser()
		st.activateUser(req)

		credentials.Password = must(auth.ParsePassword("wrongPassword"))

		authenticated, err := st.svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, non-existant user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, req := st.registerUser()
		st.activateUser(req)

		credentials.Email = must(email.ParseAddress("jacob@example.com"))

		authenticated, err := st.svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, inactive user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, _ := st.registerUser()

		authenticated, err := st.svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if authenticated {
			t.Fatalf("expected authentication to fail")
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, store fails", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, req := st.registerUser()
		st.activateUser(req)

		failingDeps := testerr.NewFailingDeps(testerr.Err, 1)
		st.store.tracker = &failingDeps[0]

		_, err := st.svc.Authenticate(context.Background(), credentials)
		if !errors.Is(err, testerr.Err) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", testerr.Err, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})
}

type svcTest struct {
	t       *testing.T
	svc     *auth.Service
	store   *testStore
	emailer *testEmailer
	errList *errList
	nowFunc func() time.Time
}

func newServiceTest(t *testing.T) *svcTest {
	encryptor := must(krypto.NewEncryptor([]krypto.Key{
		must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
	}))

	indexKey := must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf"))

	testDB := testdb.RunWhile(t, true)
	test := &svcTest{
		t: t,
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

	svc, err := auth.NewService(test.store, test.emailer, test.errList.AppendErr, cfg)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	test.svc = svc

	return test
}

func (st *svcTest) registerUser() (auth.Credentials, auth.ActivationRequest) {
	credentials := auth.Credentials{
		Email:    must(email.ParseAddress("info@example.com")),
		Password: must(auth.ParsePassword("reallyStrongPassword1")),
	}
	err := st.svc.RegisterUser(context.Background(), credentials)
	if err != nil {
		st.t.Fatalf("failed to register user: %v", err)
	}

	// wait for the service goroutine to finish registering.
	st.svc.Wait()
	st.errList.assertNoError(st.t)

	// Get the last email
	index := len(st.emailer.emails) - 1
	request, ok := st.emailer.emails[index].data.(auth.ActivationRequest)
	if !ok {
		st.t.Fatalf("unexpected data type: %T", st.emailer.emails[index].data)
	}

	return credentials, request
}

func (st *svcTest) activateUser(req auth.ActivationRequest) {
	err := st.svc.ActivateUser(context.Background(), req)
	if err != nil {
		st.t.Fatalf("failed to activate user: %v", err)
	}

	// wait for the service goroutine to finish activating.
	st.svc.Wait()
	st.errList.assertNoError(st.t)
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
