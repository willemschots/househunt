package web

import (
	"io"
	"log/slog"
	"net/http"
)

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

	return mux
}
