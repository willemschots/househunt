package web

import (
	"io"
	"log"
	"log/slog"
	"net/http"
)

type ViewRenderer interface {
	Render(w io.Writer, name string, data any) error
}

type Server struct {
	mux          *http.ServeMux
	logger       *log.Logger
	ViewRenderer ViewRenderer
}

func NewServer(logger *slog.Logger, viewRenderer ViewRenderer) *Server {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		err := viewRenderer.Render(w, "hello-world", "Hello World! (via a template)")
		if err != nil {
			logger.Error("error rendering view", "error", err)
		}
	}))

	return &Server{
		mux: mux,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
