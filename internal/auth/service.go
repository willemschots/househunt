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
	ErrTokenExpired     = errors.New("token expired")
)

// Emailer is used to send templated emails.
type Emailer interface {
	Send(ctx context.Context, template string, to email.Address, data interface{}) error
}

// ErrFunc is a function that handles errors.
type ErrFunc func(error)

// ServiceConfig is the configuration for the Service.
type ServiceConfig struct {
	// WorkerTimeout is the max duration worker goroutines are allowed
	// to take befor they are cancelled.
	WorkerTimeout time.Duration
	// TokenExpirty is the duration a token is valid.
	TokenExpiry time.Duration
}

// Service is the type that provides the main rules for
// authentication.
type Service struct {
	store      Store
	emailer    Emailer
	wg         *sync.WaitGroup
	errHandler ErrFunc
	cfg        ServiceConfig

	// NowFunc is used to get the current time.
	// Exposed for testing purposes.
	NowFunc func() time.Time
}

func NewService(s Store, emailer Emailer, errHandler ErrFunc, cfg ServiceConfig) *Service {
	svc := &Service{
		store:      s,
		emailer:    emailer,
		wg:         &sync.WaitGroup{},
		errHandler: errHandler,
		cfg:        cfg,
		NowFunc:    time.Now,
	}

	return svc
}

// Wait waits for all open workers to finish.
func (s *Service) Wait() {
	s.wg.Wait()
}

// RegisterAccount registers a new account for the provided credentials.
func (s *Service) RegisterAccount(_ context.Context, c Credentials) error {
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

	// The actual work is done in a separate goroutine to prevent:
	// - Waiting for the email to be send might slow down sending a response.
	// - Information leakage. Timing difference between existing/non-existing
	//   accounts could lead to account enumeration attacks.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		wCtx, cancel := context.WithTimeout(context.Background(), s.cfg.WorkerTimeout)
		defer cancel()

		err := s.startActivation(wCtx, user)
		if err != nil {
			s.errHandler(err)
		}
	}()

	// Note that we don't let the caller know if the account was created or not.
	// This is by design, again to prevent information leakage.
	return nil
}

// startActivation begins the activation process of a new account:
// - Create a new user if necessary.
// - Create a new email token.
// - Send an email to address with an activation link.
//
// If an active user with the same email address exists, ErrDuplicateAccount is returned.
func (s *Service) startActivation(ctx context.Context, user User) error {
	token, err := GenerateToken()
	if err != nil {
		return err
	}

	tokenHash, err := token.Hash()
	if err != nil {
		return err
	}

	emailToken := EmailToken{
		TokenHash:  tokenHash,
		UserID:     0, // set after inserting the user.
		Email:      user.Email,
		Purpose:    TokenPurposeActivate,
		ConsumedAt: nil,
	}

	err = s.inTx(ctx, func(tx Tx) error {
		// TODO: Limit nr of tokens per user.

		// Find user user with the same email.
		users, txErr := tx.FindUsers(&UserFilter{
			Emails: []email.Address{user.Email},
		})
		if txErr != nil {
			return txErr
		}

		// Check if we already have an inactive user with the same email,
		// otherwise create a new user.
		if len(users) > 0 {
			if users[0].IsActive {
				return ErrDuplicateAccount
			}

			emailToken.UserID = users[0].ID
		} else {
			txErr = tx.CreateUser(&user)
			if txErr != nil {
				return txErr
			}

			emailToken.UserID = user.ID
		}

		// Create the activation token.
		txErr = tx.CreateEmailToken(&emailToken)
		if txErr != nil {
			return txErr
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Send the email.
	// This could fail independently of the transaction. This is an acceptable
	// risk for now. If the user has not received the email, they can always try to register again.
	//
	// If at some point this becomes unacceptable, we need to consider some kind of outbox pattern.
	err = s.emailer.Send(ctx, "account-activation", user.Email, ActivationRequest{
		ID:    emailToken.ID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
}

// ActivationRequest is a request to activate an account.
type ActivationRequest struct {
	ID    int
	Token Token
}

// ActivateAccount attempts to activate the requested account.
func (s *Service) ActivateAccount(ctx context.Context, req ActivationRequest) error {
	return s.inTx(ctx, func(tx Tx) error {
		// Find the token for this id.
		token, err := tx.FindEmailTokenByID(req.ID)
		if err != nil {
			return err
		}

		if token.Purpose != TokenPurposeActivate {
			return errorz.ErrNotFound
		}

		now := s.NowFunc()

		duration := now.Sub(token.CreatedAt)
		if token.ConsumedAt != nil || duration > s.cfg.TokenExpiry {
			return ErrTokenExpired
		}

		// Check if the provided token matches the stored hash.
		if !req.Token.Match(token.TokenHash) {
			return errorz.ErrNotFound
		}

		// Consume the token.
		token.ConsumedAt = &now

		err = tx.UpdateEmailToken(&token)
		if err != nil {
			return err
		}

		// Activate the corresponding user account.
		users, err := tx.FindUsers(&UserFilter{
			IDs:      []int{token.UserID},
			IsActive: ptr(false),
		})
		if err != nil {
			return err
		}

		if len(users) != 1 {
			return errorz.ErrNotFound
		}

		users[0].IsActive = true

		err = tx.UpdateUser(&users[0])
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) inTx(ctx context.Context, f func(tx Tx) error) error {
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return err
	}

	err = f(tx)
	if err != nil {
		rBackErr := tx.Rollback()
		if rBackErr != nil {
			err = errors.Join(err, rBackErr)
		}
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func ptr[T any](v T) *T {
	return &v
}
