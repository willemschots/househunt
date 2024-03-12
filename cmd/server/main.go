package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Stderr))
}

func run(ctx context.Context, w io.Writer) int {
	logger := slog.New(slog.NewTextHandler(w, nil))

	cfg, err := configFromEnv()
	if err != nil {
		logger.Error("failed to get config from environment", "error", err)
		return 1
	}

	srv := &http.Server{
		Addr:         cfg.http.addr,
		ReadTimeout:  cfg.http.readTimeout,
		WriteTimeout: cfg.http.writeTimeout,
		IdleTimeout:  cfg.http.idleTimeout,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("Hello, world!"))
			if err != nil {
				logger.Error("failed to write response", "error", err)
			}
		}),
	}

	// We need to run two tasks concurrently:
	// - Listen and serving of the HTTP server.
	// - Waiting for a signal to stop the server.

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("starting http server", "addr", cfg.http.addr)
		// ListenAndServe always returns a non-nil error,
		// g will cancel gCtx when an error is returned, so
		// this will also stop the other goroutine.
		return srv.ListenAndServe()
	})

	g.Go(func() error {
		<-gCtx.Done()
		logger.Info("stopping http server")

		shutCtx, cancel := context.WithTimeout(context.Background(), cfg.http.shutdownTimeout)
		defer cancel()

		return srv.Shutdown(shutCtx)
	})

	err = g.Wait()
	if err != nil && err != http.ErrServerClosed {
		logger.Error("http server stopped with error", "error", err)
		return 1
	}

	logger.Info("http server stopped successfully")

	return 0
}
