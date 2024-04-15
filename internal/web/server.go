package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/gorilla/schema"
	"github.com/willemschots/househunt/internal/auth"
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
		return s.view(r.w, "activate-user", r.out)
	}))

	s.mux.Handle("POST /user-activations", mapRequest(s, deps.AuthService.ActivateUser))

	// Login user endpoints
	s.mux.Handle("GET /login", s.staticHandler("login-user"))
	s.mux.Handle("POST /login", mapBoth(s, deps.AuthService.Authenticate).response(func(r result[auth.Credentials, bool]) error {
		if !r.out {
			// TODO: Handle failure to authenticate.
			return nil
		}

		// Authenticated.
		// TODO: Create session and redirect to home.
		return nil
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

func (s *Server) view(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.deps.ViewRenderer.Render(w, name, data)
}

func (s *Server) staticHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.view(w, name, nil)
		if err != nil {
			s.handleError(w, err)
			return
		}
	}
}

func (s *Server) handleError(w http.ResponseWriter, err error) {
	// TODO: Properly handle error.
	panic(err)
}
