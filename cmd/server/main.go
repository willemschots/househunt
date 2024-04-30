package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/willemschots/househunt/assets"
	"github.com/willemschots/househunt/internal"
	"github.com/willemschots/househunt/internal/auth"
	authdb "github.com/willemschots/househunt/internal/auth/db"
	"github.com/willemschots/househunt/internal/db"
	"github.com/willemschots/househunt/internal/db/migrate"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/email/postmark"
	emailview "github.com/willemschots/househunt/internal/email/view"
	"github.com/willemschots/househunt/internal/krypto"
	"github.com/willemschots/househunt/internal/web"
	"github.com/willemschots/househunt/internal/web/view"
	"github.com/willemschots/househunt/migrations"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Stderr))
}

func run(ctx context.Context, w io.Writer) int {
	logger := slog.New(slog.NewTextHandler(w, nil))
	logger = logger.With("revision", internal.BuildRevision, "revision_time", internal.BuildRevisionTime)

	cfg, err := configFromEnv()
	if err != nil {
		logger.Error("failed to get config from environment", "error", err)
		return 1
	}

	// Connect to the database.
	dbh, err := connectDB(cfg)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}

	defer func() {
		err := dbh.close()
		if err != nil {
			logger.Error("failed to close database handles", "error", err)
			return
		}
	}()

	// Run the migrations if desired.
	if cfg.db.migrate {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		logger.Info("attempting to migrate database")

		migrations, err := migrate.RunFS(ctx, dbh.write, migrations.FS, migrate.Metadata{
			Revision:          internal.BuildRevision,
			RevisionTimestamp: internal.BuildRevisionTime,
		})

		if err != nil {
			logger.Error("failed to run migrations", "error", err)
			return 1
		}

		if len(migrations) == 0 {
			logger.Info("no migrations ran")
		} else {
			for _, m := range migrations {
				logger.Info("migration ran", "name", m.Filename, "sequence", m.Sequence)
			}
		}
	}

	// Create DB encryptor.
	encryptor, err := krypto.NewEncryptor(cfg.db.encryptionKeys)
	if err != nil {
		logger.Error("failed to create encryptor", "error", err)
		return 1
	}

	// Create emailer.
	emailRenderer := emailview.NewFSRenderer(assets.EmailFS)

	var sender email.Sender
	switch cfg.email.driver {
	case "log":
		sender = email.NewLogSender(logger)
	case "postmark":
		httpClient := &http.Client{
			Timeout: 10 * time.Second,
		}
		sender = postmark.NewSender(httpClient, cfg.email.postmark)
	default:
		logger.Error("unknown email driver", "driver", cfg.email.driver)
	}
	emailer := email.NewService(emailRenderer, sender, cfg.email.service)

	// Create authentication store and service.
	authStore := authdb.New(dbh.write, dbh.read, encryptor, cfg.db.blindIndexSalt)

	authErrHandler := func(err error) {
		logger.Error("authentication service error", "error", err)
	}

	authSvc, err := auth.NewService(authStore, emailer, authErrHandler, cfg.auth)
	if err != nil {
		logger.Error("failed to create auth service", "error", err)
		return 1
	}

	// Create cookie store to store sessions.
	keysAsBytes := make([][]byte, len(cfg.http.cookieKeys))
	for i, key := range cfg.http.cookieKeys {
		keysAsBytes[i] = key.SecretValue()
	}
	sessionStore := sessions.NewCookieStore(keysAsBytes...)
	sessionStore.Options.Secure = cfg.http.server.SecureCookie
	sessionStore.Options.HttpOnly = true
	sessionStore.MaxAge(7 * 24 * 60 * 60) // 1 week

	// Register UUID type for inclusion in the session values.
	gob.Register(uuid.UUID{})

	var viewRenderer web.ViewRenderer

	viewRenderer, err = view.NewMemRenderer(assets.TemplateFS)
	if err != nil {
		logger.Error("failed to create in memory view renderer", "error", err)
		return 1
	}

	if cfg.http.viewDir != "" {
		logger.Info("loading templates from disk", "dir", cfg.http.viewDir)
		viewRenderer = view.NewFSRenderer(os.DirFS(cfg.http.viewDir))
	}

	serverDeps := &web.ServerDeps{
		Logger:       logger,
		ViewRenderer: viewRenderer,
		AuthService:  authSvc,
		SessionStore: sessionStore,
		DistFS:       http.FS(assets.DistFS),
	}

	srv := &http.Server{
		Addr:         cfg.http.addr,
		ReadTimeout:  cfg.http.readTimeout,
		WriteTimeout: cfg.http.writeTimeout,
		IdleTimeout:  cfg.http.idleTimeout,
		Handler:      web.NewServer(serverDeps, cfg.http.server),
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

type dbHandles struct {
	write *sql.DB
	read  *sql.DB
}

// connectDB connects to the database and returns the write and read handles.
func connectDB(cfg config) (*dbHandles, error) {
	writeDB, err := db.OpenSQLite(cfg.db.file, true)
	if err != nil {
		return nil, fmt.Errorf("failed to open database write handle: %w", err)
	}

	readDB, err := db.OpenSQLite(cfg.db.file, false)
	if err != nil {
		closeErr := writeDB.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("failed to open database read handle: %w", err)
	}

	handles := &dbHandles{write: writeDB, read: readDB}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = handles.ping(ctx)
	if err != nil {
		closeErr := handles.close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	return handles, nil
}

func (h *dbHandles) ping(ctx context.Context) error {
	err := h.write.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping database write handle: %w", err)
	}

	err = h.read.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping database read handle: %w", err)
	}

	return nil
}

func (h *dbHandles) close() error {
	err := h.write.Close()
	if err != nil {
		return fmt.Errorf("failed to close database write handle: %w", err)
	}

	err = h.read.Close()
	if err != nil {
		return fmt.Errorf("failed to close database read handle: %w", err)
	}

	return nil
}
