package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/krypto"
)

var (
	ErrDuplicateUser = errors.New("duplicate user")
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

	// comparisonHash is used to compare passwords when no user was found.
	comparisonHash krypto.Argon2Hash

	// NowFunc is used to get the current time.
	// Exposed for testing purposes.
	NowFunc func() time.Time
}

func NewService(s Store, emailer Emailer, errHandler ErrFunc, cfg ServiceConfig) (*Service, error) {
	tok, err := krypto.GenerateToken()
	if err != nil {
		return nil, err
	}

	hash, err := krypto.HashArgon2(tok[:])
	if err != nil {
		return nil, err
	}

	svc := &Service{
		store:          s,
		emailer:        emailer,
		wg:             &sync.WaitGroup{},
		errHandler:     errHandler,
		cfg:            cfg,
		comparisonHash: hash,
		NowFunc:        time.Now,
	}

	return svc, nil
}

// Wait waits for all open workers to finish.
func (s *Service) Wait() {
	s.wg.Wait()
}

// RegisterUser registers a new user with the provided credentials.
// The main work of this method is done in a separate goroutine. The returned
// error does not indicate whether a user was actually registered or not. This
// is by design to prevent information leakage.
func (s *Service) RegisterUser(_ context.Context, c Credentials) error {
	// Hash the password.
	pwdHash, err := c.Password.Hash()
	if err != nil {
		return err
	}

	// The actual work is done in a separate goroutine to prevent:
	// - Waiting for the email to be send might slow down sending a response.
	// - Information leakage. Timing difference between existing/non-existing
	//   user could lead to user enumeration attacks.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		wCtx, cancel := context.WithTimeout(context.Background(), s.cfg.WorkerTimeout)
		defer cancel()

		err := s.startActivation(wCtx, c.Email, pwdHash)
		if err != nil {
			s.errHandler(err)
			return
		}
	}()

	// Note that we don't let the caller know if the user was created or not.
	// This is by design, again to prevent information leakage.
	return nil
}

// startActivation begins the activation process for a new user:
// - Create a new auth.User if necessary.
// - Create a new email token.
// - Send an email to the email address with an activation link.
//
// If an active user with the same email address exists, ErrDuplicateUser is returned.
func (s *Service) startActivation(ctx context.Context, addr email.Address, pwdHash krypto.Argon2Hash) error {
	now := s.NowFunc()

	user := User{
		Email:        addr,
		PasswordHash: pwdHash,
		IsActive:     false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	token, err := krypto.GenerateToken()
	if err != nil {
		return err
	}

	tokenHash, err := krypto.HashArgon2(token[:])
	if err != nil {
		return err
	}

	emailToken := EmailToken{
		TokenHash:  tokenHash,
		UserID:     0, // set after inserting the user.
		Email:      user.Email,
		Purpose:    TokenPurposeActivate,
		CreatedAt:  now,
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
				return ErrDuplicateUser
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
	err = s.emailer.Send(ctx, "user-activation", user.Email, ActivationRequest{
		ID:    emailToken.ID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
}

// ActivationRequest is a request to activate an user.
type ActivationRequest struct {
	ID    int
	Token krypto.Token
}

// ActivateUser attempts to activate the user identified by the activation request.
func (s *Service) ActivateUser(ctx context.Context, req ActivationRequest) error {
	// finish the activation:
	// - Find the token.
	// - Check if the token is still valid.
	// - Activate the user.
	// - Consume all unconsumed activation tokens for the user.
	return s.inTx(ctx, func(tx Tx) error {
		// Find an unconsumed activation token with the provided ID.
		tokens, err := tx.FindEmailTokens(&EmailTokenFilter{
			IDs:        []int{req.ID},
			Purposes:   []TokenPurpose{TokenPurposeActivate},
			IsConsumed: ptr(false),
		})
		if err != nil {
			return err
		}

		if len(tokens) != 1 {
			return errorz.ErrNotFound
		}

		token := tokens[0]
		now := s.NowFunc()

		if now.Sub(token.CreatedAt) > s.cfg.TokenExpiry {
			return errorz.ErrNotFound
		}

		// Check if the provided token matches the stored hash.
		if !token.TokenHash.MatchBytes(req.Token[:]) {
			return errorz.ErrNotFound
		}

		// Activate the corresponding user.
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
		users[0].UpdatedAt = now

		err = tx.UpdateUser(&users[0])
		if err != nil {
			return err
		}

		// Consume all unconsumed activation tokens for the user.
		tokens, err = tx.FindEmailTokens(&EmailTokenFilter{
			UserIDs:  []int{token.UserID},
			Purposes: []TokenPurpose{TokenPurposeActivate},
		})
		if err != nil {
			return err
		}

		for _, t := range tokens {
			t.ConsumedAt = ptr(now)
			err = tx.UpdateEmailToken(&t)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Authenticate checks if the provided credentials are valid.
func (s *Service) Authenticate(ctx context.Context, c Credentials) (bool, error) {
	users, err := s.store.FindUsers(ctx, &UserFilter{
		Emails:   []email.Address{c.Email},
		IsActive: ptr(true),
	})
	if err != nil {
		return false, err
	}

	if len(users) != 1 {
		// Even if no user is found we compare to a hash to prevent timing differences
		// that could result in user enumeration attacks.
		_ = c.Password.Match(s.comparisonHash)
		return false, nil
	}

	return c.Password.Match(users[0].PasswordHash), nil
}

// RequestPasswordReset requests a password reset for the user with the provided email address.
// Similary to RegisterUser, the main work is done in a separate goroutine and no output is
// returned to indicate if the request was successful.
func (s *Service) RequestPasswordReset(ctx context.Context, addr email.Address) {
	// The actual work is done in a separate goroutine to prevent:
	// - Waiting for the email to be send might slow down sending a response.
	// - Information leakage. Timing difference between existing/non-existing
	//   user could lead to user enumeration attacks.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		wCtx, cancel := context.WithTimeout(context.Background(), s.cfg.WorkerTimeout)
		defer cancel()

		err := s.startPasswordReset(wCtx, addr)
		if err != nil {
			s.errHandler(err)
			return
		}
	}()
}

func (s *Service) startPasswordReset(ctx context.Context, addr email.Address) error {
	now := s.NowFunc()

	token, err := krypto.GenerateToken()
	if err != nil {
		return err
	}

	tokenHash, err := krypto.HashArgon2(token[:])
	if err != nil {
		return err
	}

	emailToken := EmailToken{
		TokenHash:  tokenHash,
		UserID:     0, // set after the user was found.
		Email:      addr,
		Purpose:    TokenPurposePasswordReset,
		CreatedAt:  now,
		ConsumedAt: nil,
	}

	err = s.inTx(ctx, func(tx Tx) error {
		// Find the user with the provided email address.
		users, txErr := tx.FindUsers(&UserFilter{
			Emails:   []email.Address{addr},
			IsActive: ptr(true),
		})

		if txErr != nil {
			return txErr
		}

		if len(users) != 1 {
			return errorz.ErrNotFound
		}

		user := users[0]

		// Create the new password reset token.
		emailToken.UserID = user.ID

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
	// risk for now. If the user has not received the email, they can always try to request a new password again.
	//
	// If at some point this becomes unacceptable, we need to consider some kind of outbox pattern.
	err = s.emailer.Send(ctx, "password-reset-request", addr, ActivationRequest{
		ID:    emailToken.ID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
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
