package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
)

var (
	ErrDuplicateAccount = errors.New("duplicate account")
)

// Store provides access to the user store.
type Store interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx is a transaction. If an error occurs on any of the Create/Update/Find methods,
// the transaction is considered to have failed and should be rolled back.
// Tx is not safe for concurrent use.
type Tx interface {
	Commit() error
	Rollback() error

	CreateUser(u *User) error
	UpdateUser(u *User) error
	FindUserByEmail(v email.Address) (User, error)

	CreateEmailToken(t *EmailToken) error
	UpdateEmailToken(t *EmailToken) error
	FindEmailTokenByID(id int) (EmailToken, error)
}

// ErrFunc is a function that handles errors.
type ErrFunc func(error)

// Service is the type that provides the main rules for
// authentication.
type Service struct {
	store       Store
	wg          *sync.WaitGroup
	errHandler  ErrFunc
	workTimeout time.Duration // amount of time the "worker" goroutines are allowed to run.
}

func NewService(s Store, errHandler ErrFunc, workTimeout time.Duration) *Service {
	svc := &Service{
		store:       s,
		wg:          &sync.WaitGroup{},
		errHandler:  errHandler,
		workTimeout: workTimeout,
	}

	return svc
}

func (s *Service) Close() {
	s.wg.Wait()
}

// RegisterAccount registers a new account for the provided credentials.
func (s *Service) RegisterAccount(ctx context.Context, c Credentials) error {
	// Hash the password.
	pwdHash, err := c.Password.Hash()
	if err != nil {
		return err
	}

	user := User{
		Email:        c.Email,
		PasswordHash: pwdHash,
		IsActive:     false,
	}

	// The main logic to create a new user is run in a separate goroutine.
	// This is for two reaons:
	// - Waiting for the email to be send might slow down sending a response.
	// - Information leakage. Timing difference between existing/non-existing
	//   accounts could lead to account enumeration attacks.
	s.wg.Add(1)
	go func() {
		wCtx, cancel := context.WithTimeout(ctx, s.workTimeout)
		defer cancel()
		s.createNewUser(wCtx, user)
		s.wg.Done()
	}()

	// Note that we don't let the caller know if the account was created or not.
	// This is by design, again to prevent information leakage.
	return nil
}

func (s *Service) createNewUser(ctx context.Context, user User) {
	token, err := GenerateToken()
	if err != nil {
		s.errHandler(err)
		return
	}

	tokenHash, err := token.Hash()
	if err != nil {
		s.errHandler(err)
		return
	}

	s.inTx(ctx, func(tx Tx) error {
		_, err := tx.FindUserByEmail(user.Email)
		if err == nil {
			return ErrDuplicateAccount
		}

		if !errors.Is(err, errorz.ErrNotFound) {
			return err
		}

		err = tx.CreateUser(&user)
		if err != nil {
			return err
		}

		emailToken := EmailToken{
			TokenHash:  tokenHash,
			UserID:     user.ID,
			Email:      user.Email,
			Purpose:    TokenPurposeActivate,
			ConsumedAt: nil,
		}

		err = tx.CreateEmailToken(&emailToken)
		if err != nil {
			return err
		}

		return nil
	})

	// TODO: Send registration email.
}

func (s *Service) inTx(ctx context.Context, f func(tx Tx) error) {
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		s.errHandler(err)
		return
	}

	err = f(tx)
	if err != nil {
		rBackErr := tx.Rollback()
		if rBackErr != nil {
			err = errors.Join(err, rBackErr)
		}
		s.errHandler(err)
		return
	}

	err = tx.Commit()
	if err != nil {
		s.errHandler(err)
		return
	}
}
