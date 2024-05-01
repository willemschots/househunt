package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
		st.emailer.assertLastEmail(t, "user-activation", credentials.Email, func(t *testing.T, data any) {
			req, ok := data.(auth.EmailTokenRaw)
			if !ok {
				t.Fatalf("unexpected data type: %T", data)
			}
			if req.ID == uuid.Nil {
				t.Fatalf("expected ID to be set")
			}
			if len(req.Token) == 0 {
				t.Fatalf("expected token to be set")
			}
		})
	})

	t.Run("ok, re-register non-activated user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, _ := st.registerUser()
		st.emailer.clearEmails()

		// Register again.
		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for service goroutine to finish registering.
		st.svc.Wait()

		// Verify no errors were reported to the error handler.
		st.errList.assertNoError(t)

		// Assert that an email was send to the email address again.
		st.emailer.assertLastEmail(t, "user-activation", credentials.Email, func(t *testing.T, data any) {
			req, ok := data.(auth.EmailTokenRaw)
			if !ok {
				t.Fatalf("unexpected data type: %T", data)
			}
			if req.ID == uuid.Nil {
				t.Fatalf("expected ID to be set")
			}
			if len(req.Token) == 0 {
				t.Fatalf("expected token to be set")
			}
		})
	})

	t.Run("fail async, re-register active user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, activationTok := st.registerUser()
		st.activateUser(activationTok)
		st.emailer.clearEmails()

		// Register again.
		err := st.svc.RegisterUser(context.Background(), credentials)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		st.svc.Wait()

		// Now we should have an error.
		st.errList.assertErrorIs(t, auth.ErrDuplicateUser)
		st.emailer.assertNoEmails(t)
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
			st.emailer.assertNoEmails(t)
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
		_, tok := st.registerUser()

		err := st.svc.ActivateUser(context.Background(), tok)
		if err != nil {
			t.Fatalf("failed to activate user: %v", err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, non-matching token", func(t *testing.T) {
		st := newServiceTest(t)
		_, tok := st.registerUser()

		tok.Token = must(krypto.ParseToken("0102030405060708091011121314151617181920212223242526272829303132"))

		err := st.svc.ActivateUser(context.Background(), tok)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, non-existant token", func(t *testing.T) {
		st := newServiceTest(t)
		_, tok := st.registerUser()

		tok.ID = uuid.New()

		err := st.svc.ActivateUser(context.Background(), tok)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, token already consumed", func(t *testing.T) {
		st := newServiceTest(t)
		_, tok := st.registerUser()

		// Consume token once.
		st.activateUser(tok)

		// Consume token again.
		err := st.svc.ActivateUser(context.Background(), tok)
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
		_, tok1 := st.registerUser()
		_, tok2 := st.registerUser()

		// activate user with the second token.
		st.activateUser(tok2)

		// now try with the first token.
		err := st.svc.ActivateUser(context.Background(), tok1)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, expired token", func(t *testing.T) {
		st := newServiceTest(t)
		_, tok := st.registerUser()

		// TokenExpiry is set to 1 hour.
		// Simulate the current time being an hour ahead.
		st.svc.NowFunc = func() time.Time {
			return time.Now().Add(time.Hour + time.Second)
		}

		err := st.svc.ActivateUser(context.Background(), tok)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, token for different purpose", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(credentials.Email)
		st.emailer.clearEmails()

		// Try to activate the user with the reset token.
		err := st.svc.ActivateUser(context.Background(), resetTok)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 6) {
		t.Run("fail, store fails", func(t *testing.T) {
			st := newServiceTest(t)
			_, tok := st.registerUser()
			st.store.tracker = &tracker

			err := st.svc.ActivateUser(context.Background(), tok)
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
		credentials, tok := st.registerUser()
		st.activateUser(tok)

		user, err := st.svc.Authenticate(context.Background(), credentials)
		if err != nil {
			t.Fatalf("failed to authenticate: %v", err)
		}

		if user.ID == uuid.Nil || user.Email != credentials.Email {
			t.Fatalf("unexpected user: %v", user)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, wrong password", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, tok := st.registerUser()
		st.activateUser(tok)

		credentials.Password = must(auth.ParsePassword("wrongPassword"))

		_, err := st.svc.Authenticate(context.Background(), credentials)

		var invalidInput errorz.InvalidInput
		if !errors.As(err, &invalidInput) {
			t.Fatalf("expected error to be of type %T, got %T (via errors.As)", invalidInput, err)
		}

		if !errors.Is(invalidInput, auth.ErrInvalidCredentials) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", auth.ErrInvalidCredentials, invalidInput)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, non-existant user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, tok := st.registerUser()
		st.activateUser(tok)

		credentials.Email = must(email.ParseAddress("jacob@example.com"))

		_, err := st.svc.Authenticate(context.Background(), credentials)

		var invalidInput errorz.InvalidInput
		if !errors.As(err, &invalidInput) {
			t.Fatalf("expected error to be of type %T, got %T (via errors.As)", invalidInput, err)
		}

		if !errors.Is(invalidInput, auth.ErrInvalidCredentials) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", auth.ErrInvalidCredentials, invalidInput)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("ok, failed to authenticate, inactive user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, _ := st.registerUser()

		_, err := st.svc.Authenticate(context.Background(), credentials)

		var invalidInput errorz.InvalidInput
		if !errors.As(err, &invalidInput) {
			t.Fatalf("expected error to be of type %T, got %T (via errors.As)", invalidInput, err)
		}

		if !errors.Is(invalidInput, auth.ErrInvalidCredentials) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", auth.ErrInvalidCredentials, invalidInput)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
	})

	t.Run("fail, store fails", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, tok := st.registerUser()
		st.activateUser(tok)

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

func Test_Service_RequestPasswordReset(t *testing.T) {
	t.Run("ok, active user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, aTok := st.registerUser()
		st.activateUser(aTok)
		st.emailer.clearEmails()

		st.svc.RequestPasswordReset(context.Background(), credentials.Email)

		// Wait for service goroutine to finish.
		st.svc.Wait()

		// Verify no errors were reported to the error handler.
		st.errList.assertNoError(t)

		st.emailer.assertLastEmail(t, "password-reset-request", credentials.Email, func(t *testing.T, data any) {
			resetTok, ok := data.(auth.EmailTokenRaw)
			if !ok {
				t.Fatalf("unexpected data type: %T", data)
			}
			if resetTok.ID == uuid.Nil {
				t.Fatalf("expected ID to be set")
			}
			if len(resetTok.Token) == 0 {
				t.Fatalf("expected token to be set")
			}
		})
	})

	t.Run("fail async, non-existant user", func(t *testing.T) {
		st := newServiceTest(t)
		_, aTok := st.registerUser()
		st.activateUser(aTok)
		st.emailer.clearEmails()

		email := must(email.ParseAddress("jacob@example.com"))
		st.svc.RequestPasswordReset(context.Background(), email)

		// Wait for service goroutine to finish.
		st.svc.Wait()
		st.errList.assertErrorIs(t, errorz.ErrNotFound)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail async, inactive user", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, _ := st.registerUser()
		st.emailer.clearEmails()

		st.svc.RequestPasswordReset(context.Background(), credentials.Email)

		// Wait for service goroutine to finish.
		st.svc.Wait()
		st.errList.assertErrorIs(t, errorz.ErrNotFound)
		st.emailer.assertNoEmails(t)
	})

	// TODO: add case "fail async, too many reset password requests"

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 4) {
		t.Run("fail async, store fails", func(t *testing.T) {
			st := newServiceTest(t)
			credentials, aTok := st.registerUser()
			st.activateUser(aTok)
			st.emailer.clearEmails()

			st.store.tracker = &tracker

			st.svc.RequestPasswordReset(context.Background(), credentials.Email)

			// Wait for service goroutine to finish registering.
			st.svc.Wait()
			st.errList.assertErrorIs(t, testerr.Err)
			st.emailer.assertNoEmails(t)
		})
	}

	t.Run("fail async, emailer fails", func(t *testing.T) {
		st := newServiceTest(t)
		credentials, aTok := st.registerUser()
		st.activateUser(aTok)
		st.emailer.clearEmails()
		st.emailer.testErr = testerr.Err

		st.svc.RequestPasswordReset(context.Background(), credentials.Email)

		// Wait for service goroutine to finish registering.
		st.svc.Wait()
		st.errList.assertErrorIs(t, testerr.Err)
	})
}

func Test_Service_ResetPassword(t *testing.T) {
	t.Run("ok, reset password", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)
		st.emailer.clearEmails()

		// Reset the password by providing the reset token and a new password.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}
		err := st.svc.ResetPassword(context.Background(), newPass)
		if err != nil {
			t.Fatalf("failed to reset password: %v", err)
		}

		// Wait for service goroutine to finish.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertLastEmail(t, "password-reset-success", oldCreds.Email, nil)

		// Check that the old password no longer works.
		if st.authenticate(oldCreds) {
			t.Fatalf("expected authentication to fail")
		}

		// Check that the new password works.
		newCreds := auth.Credentials{
			Email:    oldCreds.Email,
			Password: newPass.Password,
		}
		if !st.authenticate(newCreds) {
			t.Fatalf("expected authentication to succeed")
		}
	})

	t.Run("fail, non-matching token", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)
		st.emailer.clearEmails()

		// Reset the password by providing the reset token and a new password.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}

		newPass.RawToken.Token = must(krypto.ParseToken("0102030405060708091011121314151617181920212223242526272829303132"))

		err := st.svc.ResetPassword(context.Background(), newPass)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail, non-existant token", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)
		st.emailer.clearEmails()

		// Reset the password by providing the reset token and a new password.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}

		newPass.RawToken.ID = uuid.New()

		err := st.svc.ResetPassword(context.Background(), newPass)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail, token already consumed", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)

		newPassword := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}
		// Reset the password once.
		st.resetPassword(newPassword)
		st.emailer.clearEmails()

		// Try resetting the password again.
		err := st.svc.ResetPassword(context.Background(), newPassword)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail, other token used to reset password", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok1 := st.requestPasswordReset(oldCreds.Email)
		resetTok2 := st.requestPasswordReset(oldCreds.Email)

		newPass1 := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok1,
		}

		newPass2 := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok2,
		}

		// Reset the password with the second token.
		st.resetPassword(newPass2)
		st.emailer.clearEmails()

		// Now try with the first token.
		err := st.svc.ResetPassword(context.Background(), newPass1)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail, expired token", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)
		st.emailer.clearEmails()

		// TokenExpiry is set to 1 hour.
		// Simulate the current time being an hour ahead.
		st.svc.NowFunc = func() time.Time {
			return time.Now().Add(time.Hour + time.Second)
		}

		// Reset the password by providing the reset token and a new password.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}

		err := st.svc.ResetPassword(context.Background(), newPass)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	t.Run("fail, token for different purpose", func(t *testing.T) {
		st := newServiceTest(t)
		_, aTok := st.registerUser()
		st.emailer.clearEmails()

		// Try to reset the password with the activation token.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: aTok,
		}

		err := st.svc.ResetPassword(context.Background(), newPass)
		if !errors.Is(err, errorz.ErrNotFound) {
			t.Fatalf("expected error %v, got %v (via errors.Is)", errorz.ErrNotFound, err)
		}

		// assert no async errors were reported.
		st.svc.Wait()
		st.errList.assertNoError(t)
		st.emailer.assertNoEmails(t)
	})

	for _, tracker := range testerr.NewFailingDeps(testerr.Err, 7) {
		t.Run("fail, store fails", func(t *testing.T) {
			st := newServiceTest(t)
			oldCreds, aTok := st.registerUser()
			st.activateUser(aTok)
			resetTok := st.requestPasswordReset(oldCreds.Email)
			st.emailer.clearEmails()

			st.store.tracker = &tracker

			// Reset the password by providing the reset token and a new password.
			newPass := auth.NewPassword{
				Password: must(auth.ParsePassword("otherPassword")),
				RawToken: resetTok,
			}
			err := st.svc.ResetPassword(context.Background(), newPass)
			if !errors.Is(err, testerr.Err) {
				t.Fatalf("expected error %v, got %v (via errors.Is)", testerr.Err, err)
			}

			// Wait for service goroutine to finish.
			st.svc.Wait()
			st.errList.assertNoError(t)
			st.emailer.assertNoEmails(t)
		})
	}

	t.Run("fail, emailer fails", func(t *testing.T) {
		st := newServiceTest(t)
		oldCreds, aTok := st.registerUser()
		st.activateUser(aTok)
		resetTok := st.requestPasswordReset(oldCreds.Email)
		st.emailer.clearEmails()

		st.emailer.testErr = testerr.Err

		// Reset the password by providing the reset token and a new password.
		newPass := auth.NewPassword{
			Password: must(auth.ParsePassword("otherPassword")),
			RawToken: resetTok,
		}
		err := st.svc.ResetPassword(context.Background(), newPass)
		if err != nil {
			t.Fatalf("failed to reset password: %v", err)
		}

		// Wait for service goroutine to finish.
		st.svc.Wait()
		st.errList.assertErrorIs(t, testerr.Err)
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

func (st *svcTest) registerUser() (auth.Credentials, auth.EmailTokenRaw) {
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

	// Get the raw email token.
	index := len(st.emailer.emails) - 1
	raw, ok := st.emailer.emails[index].data.(auth.EmailTokenRaw)
	if !ok {
		st.t.Fatalf("unexpected data type: %T", st.emailer.emails[index].data)
	}

	return credentials, raw
}

func (st *svcTest) activateUser(req auth.EmailTokenRaw) {
	err := st.svc.ActivateUser(context.Background(), req)
	if err != nil {
		st.t.Fatalf("failed to activate user: %v", err)
	}

	// wait for the service goroutine to finish activating.
	st.svc.Wait()
	st.errList.assertNoError(st.t)
}

func (st *svcTest) requestPasswordReset(email email.Address) auth.EmailTokenRaw {
	st.svc.RequestPasswordReset(context.Background(), email)

	// wait for the service goroutine to finish requesting.
	st.svc.Wait()
	st.errList.assertNoError(st.t)

	// Get the raw email token
	index := len(st.emailer.emails) - 1
	raw, ok := st.emailer.emails[index].data.(auth.EmailTokenRaw)
	if !ok {
		st.t.Fatalf("unexpected data type: %T", st.emailer.emails[index].data)
	}

	return raw
}

func (st *svcTest) resetPassword(np auth.NewPassword) {
	err := st.svc.ResetPassword(context.Background(), np)
	if err != nil {
		st.t.Fatalf("failed to reset password: %v", err)
	}

	// wait for the service goroutine to finish resetting.
	st.svc.Wait()
	st.errList.assertNoError(st.t)
}

func (st *svcTest) authenticate(credentials auth.Credentials) bool {
	_, err := st.svc.Authenticate(context.Background(), credentials)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return false
		}
		st.t.Fatalf("failed to authenticate: %v", err)
	}

	return true
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

func (f *testStore) FindUsers(ctx context.Context, filter auth.UserFilter) ([]auth.User, error) {
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

func (tx *testTx) CreateUser(u auth.User) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.CreateUser(u)
	})
}

func (tx *testTx) UpdateUser(u auth.User) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.UpdateUser(u)
	})
}

func (tx *testTx) FindUsers(filter auth.UserFilter) ([]auth.User, error) {
	return testerr.MaybeFail(tx.store.tracker, func() ([]auth.User, error) {
		return tx.tx.FindUsers(filter)
	})
}

func (tx *testTx) CreateEmailToken(t auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.CreateEmailToken(t)
	})
}

func (tx *testTx) UpdateEmailToken(t auth.EmailToken) error {
	return testerr.MaybeFailErrFunc(tx.store.tracker, func() error {
		return tx.tx.UpdateEmailToken(t)
	})
}

func (tx *testTx) FindEmailTokens(filter auth.EmailTokenFilter) ([]auth.EmailToken, error) {
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

func (e *testEmailer) clearEmails() {
	e.emails = nil
}

func (e *testEmailer) Send(_ context.Context, template string, to email.Address, data interface{}) error {
	e.emails = append(e.emails, sendEmail{
		template:  template,
		recipient: to,
		data:      data,
	})

	return e.testErr
}

func (e *testEmailer) assertLastEmail(t *testing.T, template string, recipient email.Address, dataFunc func(t *testing.T, data any)) {
	t.Helper()

	if len(e.emails) == 0 {
		t.Fatalf("expected an email, got none")
	}

	last := e.emails[len(e.emails)-1]
	if last.template != template {
		t.Fatalf("wanted last email to use template %s, got %s", template, last.template)
	}

	if last.recipient != recipient {
		t.Fatalf("wanted last email to be to %s, got %s", recipient, last.recipient)
	}

	if dataFunc != nil {
		dataFunc(t, last.data)
	}
}

func (e *testEmailer) assertNoEmails(t *testing.T) {
	t.Helper()

	if len(e.emails) != 0 {
		t.Fatalf("expected no emails, got %d", len(e.emails))
	}
}
