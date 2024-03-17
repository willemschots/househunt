package web

import (
	"io"
	"log/slog"
	"net/http"
)

// ViewRenderer renders named views with the given data.
type ViewRenderer interface {
	Render(w io.Writer, name string, data any) error
}

func NewServer(logger *slog.Logger, viewRenderer ViewRenderer) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		err := viewRenderer.Render(w, "hello-world", "Hello World! (via a template)")
		if err != nil {
			logger.Error("error rendering view", "error", err)
		}
	}))

	mux.Handle("GET /register", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("register page requested")
		w.WriteHeader(http.StatusOK)
		err := viewRenderer.Render(w, "register-account", nil)
		if err != nil {
			logger.Error("error rendering view", "error", err)
		}
	}))

	return mux
}
