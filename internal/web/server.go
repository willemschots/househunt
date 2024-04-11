package web

import (
	"io"
	"log/slog"
	"net/http"

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

func NewServer(s *ServerDeps) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		err := s.ViewRenderer.Render(w, "hello-world", "Hello World! (via a template)")
		if err != nil {
			s.Logger.Error("error rendering view", "error", err)
		}
	}))

	mux.Handle("GET /register", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Logger.Info("register page requested")
		w.WriteHeader(http.StatusOK)
		err := s.ViewRenderer.Render(w, "register-user", nil)
		if err != nil {
			s.Logger.Error("error rendering view", "error", err)
		}
	}))

	return mux
}
