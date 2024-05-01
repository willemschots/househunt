package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
	"github.com/gorilla/sessions"
	"github.com/willemschots/househunt/internal"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/krypto"
)

// ViewRenderer renders named views with the given data.
type ViewRenderer interface {
	Render(w io.Writer, name string, data any) error
}

// ServerDeps are the dependencies for the server.
type ServerDeps struct {
	Logger       *slog.Logger
	ViewRenderer ViewRenderer
	AuthService  *auth.Service
	SessionStore sessions.Store
	DistFS       http.FileSystem
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	CSRFKey      krypto.Key
	SecureCookie bool
}

type Server struct {
	deps    *ServerDeps
	mux     *http.ServeMux
	decoder *schema.Decoder
	handler http.Handler
}

func NewServer(deps *ServerDeps, cfg ServerConfig) *Server {
	s := &Server{
		deps:    deps,
		mux:     http.NewServeMux(),
		decoder: schema.NewDecoder(),
	}

	// Most non-static endpoints below are created using the newHandler functions.
	// These functions return handlers that automatically map between HTTP requests, target functions and HTTP responses.
	// The request mapping and response writing is customizable.

	// Homepage endpoint.
	s.public("GET /{$}", s.staticHandler("hello-world"))

	// Register user endpoints.
	{
		s.publicOnly("GET /register", s.staticHandler("register-user"))
	}
	{
		const route = "POST /register"
		h := newInputHandler(s, deps.AuthService.RegisterUser)
		h.onSuccess(func(r result[auth.Credentials, struct{}]) error {
			r.sess.AddFlash("Thank you for your registration. Please follow the instructions that have arrived in your inbox.")
			err := r.s.deps.SessionStore.Save(r.r, r.w, r.sess)
			if err != nil {
				return err
			}

			http.Redirect(r.w, r.r, "/register", http.StatusFound)
			return nil
		})

		s.publicOnly(route, h)
	}

	// Activate user endpoints.
	{
		const route = "GET /user-activations"
		h := newHandler(s, func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
			// this target function ensures the input is validated before it's forwared to the view.
			return token, nil
		})
		h.onSuccess(func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
			return r.s.writeView(r.w, r.r, "activate-user", r.out)
		})

		s.publicOnly(route, h)
	}
	{
		const route = "POST /user-activations"
		h := newInputHandler(s, deps.AuthService.ActivateUser)
		h.onSuccess(func(r result[auth.EmailTokenRaw, struct{}]) error {
			r.sess.AddFlash("Your account has been activated, login below.")
			err := r.s.deps.SessionStore.Save(r.r, r.w, r.sess)
			if err != nil {
				return err
			}

			http.Redirect(r.w, r.r, "/login", http.StatusFound)
			return nil
		})

		s.publicOnly(route, h)
	}

	// Login user endpoints
	{
		s.publicOnly("GET /login", s.staticHandler("login-user"))
	}
	{
		const route = "POST /login"
		h := newHandler(s, deps.AuthService.Authenticate)
		h.onSuccess(func(r result[auth.Credentials, auth.User]) error {
			// If we get here, the user has been authenticated.

			// We clear the CSRF token to provide defense in depth against fixation attacks.
			// If an attacker somehow gains access to the CSRF token before the user logged in, it will
			// be worthless after the user logs in.
			// See this link for more information:
			// https://security.stackexchange.com/questions/209993/csrf-token-unique-per-user-session-why
			//
			// A new CSRF token will be generated on the next GET request after the redirect.
			http.SetCookie(r.w, &http.Cookie{
				Name:   csrfTokenCookieName,
				MaxAge: -1,
			})

			setSessionUserID(r.sess, r.out.ID)
			err := r.s.deps.SessionStore.Save(r.r, r.w, r.sess)
			if err != nil {
				return err
			}

			http.Redirect(r.w, r.r, "/dashboard", http.StatusFound)
			return nil
		})

		s.publicOnly(route, h)
	}

	// Logout user endpoint
	{
		const route = "POST /logout"
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := sessionFromCtx(r.Context())
			if err != nil {
				s.handleError(w, r, err)
				return
			}

			deleteSessionUserID(sess)
			err = s.deps.SessionStore.Save(r, w, sess)
			if err != nil {
				s.handleError(w, r, err)
				return
			}

			http.Redirect(w, r, "/", http.StatusFound)
		})

		s.loggedIn(route, h)
	}

	// Request password reset endpoints
	{
		s.publicOnly("GET /forgot-password", s.staticHandler("forgot-password"))
	}
	{
		const route = "POST /forgot-password"

		type passwordReset struct {
			Email email.Address
		}

		h := newInputHandler(s, func(ctx context.Context, reset passwordReset) error {
			// Need to adapt because RequestPasswordReset only accepts an email address, not a struct.
			s.deps.AuthService.RequestPasswordReset(ctx, reset.Email)
			return nil
		})
		h.onSuccess(func(r result[passwordReset, struct{}]) error {
			r.sess.AddFlash("Check your inbox for instructions to reset your password.")
			err := r.s.deps.SessionStore.Save(r.r, r.w, r.sess)
			if err != nil {
				return err
			}

			http.Redirect(r.w, r.r, "/forgot-password", http.StatusFound)
			return nil
		})

		s.publicOnly(route, h)
	}

	// Reset password endpoints
	{
		const route = "GET /password-resets"
		h := newHandler(s, func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
			// this target function ensures the input is validated before it's forwared to the view.
			return token, nil
		})
		h.onSuccess(func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
			return r.s.writeView(r.w, r.r, "reset-password", r.out)
		})

		s.publicOnly(route, h)
	}

	{
		const route = "POST /password-resets"
		h := newInputHandler(s, deps.AuthService.ResetPassword)
		h.onSuccess(func(r result[auth.NewPassword, struct{}]) error {
			r.sess.AddFlash("Your password was reset, login with your new password below")
			err := r.s.deps.SessionStore.Save(r.r, r.w, r.sess)
			if err != nil {
				return err
			}

			http.Redirect(r.w, r.r, "/login", http.StatusFound)
			return nil
		})

		s.publicOnly(route, h)
	}

	// Dashboard endpoints
	s.loggedIn("GET /dashboard", s.staticHandler("dashboard"))

	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(s.deps.DistFS)))

	// Wrap the mux with global middlewares.
	csrfMW := csrf.Protect(
		cfg.CSRFKey.SecretValue(),
		csrf.CookieName(csrfTokenCookieName),
		csrf.FieldName(csrfTokenField),
		csrf.Secure(cfg.SecureCookie),
	)

	middlewares := []func(http.Handler) http.Handler{
		csrfMW,
		s.session,
	}
	s.handler = s.mux
	for i := len(middlewares) - 1; i >= 0; i-- {
		s.handler = middlewares[i](s.handler)
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) staticHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.writeView(w, r, name, nil)
		if err != nil {
			s.handleError(w, r, err)
			return
		}
	}
}

func (s *Server) writeView(w http.ResponseWriter, r *http.Request, name string, data any) error {
	sess, err := sessionFromCtx(r.Context())
	if err != nil {
		return err
	}

	userID, ok := sessionUserID(sess)

	viewData := struct {
		Global any
		View   any
	}{
		Global: struct {
			Version    string
			CSRFToken  string
			IsLoggedIn bool
			UserID     uuid.UUID
			Flashes    []any
		}{
			Version:    internal.BuildRevision,
			CSRFToken:  csrf.Token(r),
			IsLoggedIn: ok,
			UserID:     userID,
			Flashes:    sess.Flashes(),
		},
		View: data,
	}

	err = s.deps.SessionStore.Save(r, w, sess)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.deps.ViewRenderer.Render(w, name, viewData)
}

func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	// TODO: Properly handle other errors.
	if errors.Is(err, errorz.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var invalidInput errorz.InvalidInput
	if errors.As(err, &invalidInput) {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	s.deps.Logger.Error("internal server error", "url", r.URL.String(), "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
