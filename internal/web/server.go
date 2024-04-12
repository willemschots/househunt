package web

import (
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

	s.mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		err := deps.ViewRenderer.Render(w, "hello-world", "Hello World! (via a template)")
		if err != nil {
			deps.Logger.Error("error rendering view", "error", err)
		}
	}))

	s.mux.Handle("GET /register", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deps.Logger.Info("register page requested")
		w.WriteHeader(http.StatusOK)
		err := deps.ViewRenderer.Render(w, "register-user", nil)
		if err != nil {
			deps.Logger.Error("error rendering view", "error", err)
		}
	}))

	s.mux.Handle("POST /register", mapRequest(s, deps.AuthService.RegisterUser).response(func(w http.ResponseWriter, _ struct{}) error {
		return nil
	}))

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
