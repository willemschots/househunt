package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/schema"
	"github.com/gorilla/sessions"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
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
}

type Server struct {
	deps    *ServerDeps
	mux     *http.ServeMux
	decoder *schema.Decoder
	handler http.Handler
}

func NewServer(deps *ServerDeps) *Server {
	s := &Server{
		deps:    deps,
		mux:     http.NewServeMux(),
		decoder: schema.NewDecoder(),
	}

	// Most non-static endpoints below are created using the mapBoth, mapRequest or mapResponse functions.
	// These functions return handlers that automatically map between HTTP requests, target functions and HTTP responses.
	// The request mapping and response writing is customizable.

	s.public("GET /{$}", s.staticHandler("hello-world"))

	// Register user endpoints.
	s.publicOnly("GET /register", s.staticHandler("register-user"))
	s.publicOnly("POST /register", mapRequest(s, deps.AuthService.RegisterUser))

	// Activate user endpoints.
	forwardRawToken := func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
		return token, nil
	}

	s.publicOnly("GET /user-activations", mapBoth(s, forwardRawToken).response(func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
		return s.writeView(r.w, r.r, "activate-user", r.out)
	}))

	s.publicOnly("POST /user-activations", mapRequest(s, deps.AuthService.ActivateUser))

	// Login user endpoints
	s.publicOnly("GET /login", s.staticHandler("login-user"))
	s.publicOnly("POST /login", mapBoth(s, deps.AuthService.Authenticate).response(func(r result[auth.Credentials, auth.User]) error {
		// If we get here, the user has been authenticated.
		// TODO: Refresh CSRF token once added.
		err := s.writeAuthSession(r.w, r.r, r.out.ID)
		if err != nil {
			return err
		}
		http.Redirect(r.w, r.r, "/dashboard", http.StatusFound)
		return nil
	}))

	// Logout user endpoint
	s.loggedIn("POST /logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := s.stopAuthSession(w, r)
		if err != nil {
			s.handleError(w, err)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}))

	// Reset password endpoints
	s.publicOnly("GET /forgot-password", s.staticHandler("forgot-password"))

	type passwordReset struct {
		Email email.Address
	}

	s.publicOnly("POST /forgot-password", mapRequest(s, func(ctx context.Context, reset passwordReset) error {
		s.deps.AuthService.RequestPasswordReset(ctx, reset.Email)
		return nil
	}))

	s.publicOnly("GET /password-resets", mapBoth(s, forwardRawToken).response(func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
		return s.writeView(r.w, r.r, "reset-password", r.out)
	}))

	s.publicOnly("POST /password-resets", mapRequest(s, deps.AuthService.ResetPassword).response(func(r result[auth.NewPassword, struct{}]) error {
		http.Redirect(r.w, r.r, "/login", http.StatusFound)
		return nil
	}))

	// Dashboard endpoints
	s.loggedIn("GET /dashboard", s.staticHandler("dashboard"))

	// TODO: GET /reset-password - show password reset form
	//mux.Handle("POST /reset-password", HandleIn(s.AuthService.RequestPasswordReset))

	// TODO: GET /password-reset-requests
	//mux.Handle("POST /password-resets", HandleIn(s.AuthService.ResetPassword))

	// Wrap the mux with global middleware.
	s.handler = s.session(s.mux)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) staticHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.writeView(w, r, name, nil)
		if err != nil {
			s.handleError(w, err)
			return
		}
	}
}

func (s *Server) writeView(w http.ResponseWriter, r *http.Request, name string, data any) error {
	userID, ok := UserIDFromContext(r.Context())

	viewData := struct {
		Global any
		View   any
	}{
		Global: struct {
			IsLoggedIn bool
			UserID     uuid.UUID
		}{
			IsLoggedIn: ok,
			UserID:     userID,
		},
		View: data,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.deps.ViewRenderer.Render(w, name, viewData)
}

func (s *Server) handleError(w http.ResponseWriter, err error) {
	s.deps.Logger.Error("server error", "error", err)
	// TODO: Properly handle other errors.
	if errors.Is(err, errorz.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	panic(err)
}
