package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/krypto"
)

var (
	ErrDuplicateUser      = errors.New("duplicate user")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Emailer is used to send templated emails.
type Emailer interface {
	Send(ctx context.Context, template string, to email.Address, data interface{}) error
}

// ErrFunc is a function that handles errors.
type ErrFunc func(error)

// ServiceConfig is the configuration for the Service. Some methods run in seperate goroutines,
// it is up to the caller to wait for these methods to finish. This can be done by calling the
// Wait method.
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

// NewService creates a new Service.
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

	token, err := krypto.GenerateToken()
	if err != nil {
		return err
	}

	tokenHash, err := krypto.HashArgon2(token[:])
	if err != nil {
		return err
	}

	tokenID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	emailToken := EmailToken{
		ID:         tokenID,
		TokenHash:  tokenHash,
		UserID:     uuid.Nil, // set after inserting the user.
		Email:      addr,
		Purpose:    TokenPurposeActivate,
		CreatedAt:  now,
		ConsumedAt: nil,
	}

	err = s.inTx(ctx, func(tx Tx) error {
		// TODO: Limit nr of tokens per user.

		// Find user user with the same email.
		users, txErr := tx.FindUsers(UserFilter{
			Emails: []email.Address{addr},
		})
		if txErr != nil {
			return txErr
		}

		if len(users) == 0 {
			userID, txErr := uuid.NewRandom()
			if err != nil {
				return txErr
			}
			// No user with the this email found, create a new user.
			user := User{
				ID:           userID,
				Email:        addr,
				PasswordHash: pwdHash,
				IsActive:     false,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			txErr = tx.CreateUser(user)
			if txErr != nil {
				return txErr
			}

			emailToken.UserID = user.ID
		} else {
			// Check if we already have an inactive user with the same email,
			if users[0].IsActive {
				return ErrDuplicateUser
			}

			// Re-use the existing user for this email token.
			emailToken.UserID = users[0].ID
		}

		// Create the activation token.
		txErr = tx.CreateEmailToken(emailToken)
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
	err = s.emailer.Send(ctx, "user-activation", addr, EmailTokenRaw{
		ID:    emailToken.ID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
}

// ActivateUser attempts to activate the user for the provided token.
func (s *Service) ActivateUser(ctx context.Context, req EmailTokenRaw) error {
	now := s.NowFunc()

	return s.inTx(ctx, func(tx Tx) error {
		// Find an unconsumed activation token with the provided ID.
		token, err := findConsumableEmailToken(tx, req, TokenPurposeActivate, now, s.cfg.TokenExpiry)
		if err != nil {
			return err
		}

		// Activate the corresponding user.
		user, err := findUser(tx, UserFilter{
			IDs:      []uuid.UUID{token.UserID},
			IsActive: ptr(false),
		})
		if err != nil {
			return err
		}

		user.IsActive = true
		user.UpdatedAt = now

		err = tx.UpdateUser(user)
		if err != nil {
			return err
		}

		// Consume all unconsumed activation tokens for this user.
		return consumeAllTokensForUserID(tx, token.UserID, TokenPurposeActivate, now)
	})
}

// Authenticate checks if the provided credentials are valid.
func (s *Service) Authenticate(ctx context.Context, c Credentials) (User, error) {
	users, err := s.store.FindUsers(ctx, UserFilter{
		Emails:   []email.Address{c.Email},
		IsActive: ptr(true),
	})
	if err != nil {
		return User{}, err
	}

	if len(users) != 1 {
		// Even if no user is found we compare to a hash to prevent timing differences
		// that could result in user enumeration attacks.
		_ = c.Password.Match(s.comparisonHash)
		return User{}, errorz.InvalidInput{ErrInvalidCredentials}
	}

	match := c.Password.Match(users[0].PasswordHash)
	if !match {
		return User{}, errorz.InvalidInput{ErrInvalidCredentials}
	}

	return users[0], nil
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

	tokenID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	emailToken := EmailToken{
		ID:         tokenID,
		TokenHash:  tokenHash,
		UserID:     uuid.Nil, // set after the user is found.
		Email:      addr,
		Purpose:    TokenPurposePasswordReset,
		CreatedAt:  now,
		ConsumedAt: nil,
	}

	err = s.inTx(ctx, func(tx Tx) error {
		// Find the user with the provided email address.
		user, txErr := findUser(tx, UserFilter{
			Emails:   []email.Address{addr},
			IsActive: ptr(true),
		})
		if txErr != nil {
			return txErr
		}

		// Create the new password reset token.
		emailToken.UserID = user.ID

		txErr = tx.CreateEmailToken(emailToken)
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
	err = s.emailer.Send(ctx, "password-reset-request", addr, EmailTokenRaw{
		ID:    emailToken.ID,
		Token: token,
	})
	if err != nil {
		return err
	}

	return nil
}

// NewPassword contains the data required to reset a password.
type NewPassword struct {
	RawToken EmailTokenRaw
	Password Password
}

func (s *Service) ResetPassword(ctx context.Context, np NewPassword) error {
	now := s.NowFunc()

	// Hash the password.
	pwdHash, err := np.Password.Hash()
	if err != nil {
		return err
	}

	var recipient email.Address

	// finish the password reset:
	// - Find the token.
	// - Check if the token is still valid.
	// - Replace the password on the user.
	// - Consume all unconsumed activation tokens for the user.
	err = s.inTx(ctx, func(tx Tx) error {
		token, txErr := findConsumableEmailToken(tx, np.RawToken, TokenPurposePasswordReset, now, s.cfg.TokenExpiry)
		if txErr != nil {
			return txErr
		}

		user, txErr := findUser(tx, UserFilter{
			IDs:      []uuid.UUID{token.UserID},
			IsActive: ptr(true),
		})
		if txErr != nil {
			return txErr
		}

		// Update the user with the new password.
		user.PasswordHash = pwdHash
		user.UpdatedAt = now

		// We will send a confirmation email to the user after the transaction is done.
		recipient = user.Email

		txErr = tx.UpdateUser(user)
		if txErr != nil {
			return txErr
		}

		// Consume all unconsumed password reset tokens for this user.
		return consumeAllTokensForUserID(tx, token.UserID, TokenPurposePasswordReset, now)
	})
	if err != nil {
		return err
	}

	// Send the confirmation email asynchronously.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		emailCtx, cancel := context.WithTimeout(context.Background(), s.cfg.WorkerTimeout)
		defer cancel()
		err = s.emailer.Send(emailCtx, "password-reset-success", recipient, nil)
		if err != nil {
			s.errHandler(err)
			return
		}
	}()

	return nil
}

func findConsumableEmailToken(tx Tx, raw EmailTokenRaw, purpose TokenPurpose, now time.Time, maxAge time.Duration) (EmailToken, error) {
	tokens, err := tx.FindEmailTokens(EmailTokenFilter{
		IDs:        []uuid.UUID{raw.ID},
		Purposes:   []TokenPurpose{purpose},
		IsConsumed: ptr(false),
	})
	if err != nil {
		return EmailToken{}, err
	}

	if len(tokens) != 1 {
		return EmailToken{}, errorz.ErrNotFound
	}

	token := tokens[0]

	if now.Sub(token.CreatedAt) > maxAge {
		return EmailToken{}, errorz.ErrNotFound
	}

	// Check if the provided token matches the stored hash.
	if !token.TokenHash.MatchBytes(raw.Token[:]) {
		return EmailToken{}, errorz.ErrNotFound
	}

	return token, nil
}

func findUser(tx Tx, filter UserFilter) (User, error) {
	users, err := tx.FindUsers(filter)
	if err != nil {
		return User{}, err
	}

	if len(users) != 1 {
		return User{}, errorz.ErrNotFound
	}

	return users[0], nil
}

func consumeAllTokensForUserID(tx Tx, userID uuid.UUID, purpose TokenPurpose, consumedAt time.Time) error {
	tokens, err := tx.FindEmailTokens(EmailTokenFilter{
		UserIDs:  []uuid.UUID{userID},
		Purposes: []TokenPurpose{purpose},
		//IsConsumed: ptr(false),
	})
	if err != nil {
		return err
	}

	for _, t := range tokens {
		t.ConsumedAt = ptr(consumedAt)
		err = tx.UpdateEmailToken(t)
		if err != nil {
			return err
		}
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
