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
	"github.com/willemschots/househunt/internal/errorz"
)

const (
	AuthSession = "hh-auth"
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
}

func NewServer(deps *ServerDeps) *Server {
	s := &Server{
		deps:    deps,
		mux:     http.NewServeMux(),
		decoder: schema.NewDecoder(),
	}

	s.mux.Handle("GET /{$}", s.staticHandler("hello-world"))

	// Register user endpoints.
	s.mux.Handle("GET /register", s.staticHandler("register-user"))
	s.mux.Handle("POST /register", mapRequest(s, deps.AuthService.RegisterUser))

	// Activate user endpoints.
	forwardRawToken := func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
		return token, nil
	}

	s.mux.Handle("GET /user-activations", mapBoth(s, forwardRawToken).response(func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
		return s.writeView(r.w, "activate-user", r.out)
	}))

	s.mux.Handle("POST /user-activations", mapRequest(s, deps.AuthService.ActivateUser))

	// Login user endpoints
	s.mux.Handle("GET /login", s.staticHandler("login-user"))
	s.mux.Handle("POST /login", mapBoth(s, deps.AuthService.Authenticate).response(func(r result[auth.Credentials, auth.User]) error {
		// Authenticated.
		// TODO: Refresh CSRF token once implemented.
		// TODO: Redirect to dashboard.
		return s.writeAuthSession(r.w, r.r, r.out.ID)
	}))

	//s.mux.Handle("GET /login")

	//registerHandler := newInHandler(s, deps.AuthService.RegisterUser)
	//registerHandler.outFunc = func(w http.ResponseWriter, _ struct{}) {
	//}

	//s.mux.Handle("POST /register", registerHandler)

	// TODO: GET /user-activations
	//mux.Handle("POST /user-activations", HandleIn(s.AuthService.ActivateUser))

	// TODO: GET /login - show login form
	//mux.Handle("POST /login", HandleInOut(s.AuthService.Authenticate))

	// TODO: GET /reset-password - show password reset form
	//mux.Handle("POST /reset-password", HandleIn(s.AuthService.RequestPasswordReset))

	// TODO: GET /password-reset-requests
	//mux.Handle("POST /password-resets", HandleIn(s.AuthService.ResetPassword))

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) staticHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.writeView(w, name, nil)
		if err != nil {
			s.handleError(w, err)
			return
		}
	}
}

func (s *Server) writeView(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.deps.ViewRenderer.Render(w, name, data)
}

func (s *Server) writeAuthSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	session, err := s.deps.SessionStore.Get(r, AuthSession)
	if err != nil {
		return err
	}

	if !session.IsNew {
		return errors.New("non-new session")
	}

	session.Values["userID"] = userID
	return s.deps.SessionStore.Save(r, w, session)
}

func (s *Server) readAuthSession(r *http.Request) (uuid.UUID, error) {
	session, err := s.deps.SessionStore.Get(r, AuthSession)
	if err != nil {
		return uuid.Nil, err
	}

	if session.IsNew {
		return uuid.Nil, errorz.ErrNotFound
	}

	userID, ok := session.Values["userID"].(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user ID")
	}

	return userID, nil
}

func (s *Server) handleError(w http.ResponseWriter, err error) {
	// TODO: Properly handle error.
	panic(err)
}
