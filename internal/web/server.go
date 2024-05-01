package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/errorz"
	"github.com/willemschots/househunt/internal/krypto"
	"github.com/willemschots/househunt/internal/web/sessions"
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
	SessionStore *sessions.Store
	DistFS       http.FileSystem
}

// ServerConfig is the configuration for the server.
type ServerConfig struct {
	CSRFKey      krypto.Key
	SecureCookie bool
}

// Server implements the server for the application.
// Generally:
// - Methods prefixed with "write" write a full response including headers and status code.
// - Methods prefixed with "render" only write a response body.
type Server struct {
	deps    *ServerDeps
	mux     *http.ServeMux
	decoder *schema.Decoder
	handler http.Handler
}

func NewServer(deps *ServerDeps, cfg ServerConfig) *Server {
	s := &Server{
		deps:    deps,
		mux:     http.NewServeMux(),
		decoder: schema.NewDecoder(),
	}

	// Below we set up all the endpoints of the server.
	//
	// Most non-static endpoints below are created using the newHandler functions.
	// These functions return handlers that automatically map between HTTP requests, target functions and HTTP responses.
	// The request mapping and response writing is customizable.

	// Homepage endpoint.
	s.public("GET /{$}", newViewHandler(s, "hello-world"))

	// Register user endpoints.
	{
		s.publicOnly("GET /register", newViewHandler(s, "register-user"))
	}
	{
		const route = "POST /register"
		h := newInputHandler(s, deps.AuthService.RegisterUser)
		h.onFail = func(r shared, err error) {
			s.writeErrorView(r.w, r.r, "register-user", err)
		}
		h.onSuccess = func(r result[auth.Credentials, struct{}]) error {
			r.sess.AddFlash("Thank you for your registration. Please follow the instructions that have arrived in your inbox.")
			s.writeRedirect(r.w, r.r, "/register", http.StatusFound)
			return nil
		}

		s.publicOnly(route, h)
	}

	// Activate user endpoints.
	{
		const route = "GET /user-activations"
		h := newHandler(s, func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
			// this target function ensures the input is validated before it's forwared to the view.
			return token, nil
		})
		h.onSuccess = func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
			s.writeView(r.w, r.r, "activate-user", r.out)
			return nil
		}

		s.publicOnly(route, h)
	}
	{
		const route = "POST /user-activations"
		h := newInputHandler(s, deps.AuthService.ActivateUser)
		h.onFail = func(r shared, err error) {
			s.writeErrorView(r.w, r.r, "activate-user", err)
		}
		h.onSuccess = func(r result[auth.EmailTokenRaw, struct{}]) error {
			r.sess.AddFlash("Your account has been activated, login below.")
			s.writeRedirect(r.w, r.r, "/login", http.StatusFound)
			return nil
		}

		s.publicOnly(route, h)
	}

	// Login user endpoints
	{
		s.publicOnly("GET /login", newViewHandler(s, "login-user"))
	}
	{
		const route = "POST /login"
		h := newHandler(s, deps.AuthService.Authenticate)
		h.onFail = func(r shared, err error) {
			s.writeErrorView(r.w, r.r, "login-user", err)
		}
		h.onSuccess = func(r result[auth.Credentials, auth.User]) error {
			// If we get here, the user has been authenticated.

			// We clear the CSRF token to provide defense in depth against fixation attacks.
			// If an attacker somehow gains access to the CSRF token before the user logged in, it will
			// be worthless after the user logs in.
			// See this link for more information:
			// https://security.stackexchange.com/questions/209993/csrf-token-unique-per-user-session-why
			//
			// A new CSRF token will be generated on the next GET request after the redirect.
			http.SetCookie(r.w, &http.Cookie{
				Name:   csrfTokenCookieName,
				MaxAge: -1,
			})

			r.sess.SetUserID(r.out.ID)
			s.writeRedirect(r.w, r.r, "/dashboard", http.StatusFound)
			return nil
		}

		s.publicOnly(route, h)
	}

	// Logout user endpoint
	{
		const route = "POST /logout"
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := sessionFromCtx(r.Context())
			if err != nil {
				s.writeError(w, r, err)
				return
			}

			sess.DeleteUserID()
			s.writeRedirect(w, r, "/", http.StatusFound)
		})

		s.loggedIn(route, h)
	}

	// Request password reset endpoints
	{
		s.publicOnly("GET /forgot-password", newViewHandler(s, "forgot-password"))
	}
	{
		const route = "POST /forgot-password"

		type passwordReset struct {
			Email email.Address
		}

		h := newInputHandler(s, func(ctx context.Context, reset passwordReset) error {
			// Need to adapt because RequestPasswordReset only accepts an email address, not a struct.
			s.deps.AuthService.RequestPasswordReset(ctx, reset.Email)
			return nil
		})
		h.onFail = func(r shared, err error) {
			s.writeErrorView(r.w, r.r, "forgot-password", err)
		}
		h.onSuccess = func(r result[passwordReset, struct{}]) error {
			r.sess.AddFlash("Check your inbox for instructions to reset your password.")
			s.writeRedirect(r.w, r.r, "/forgot-password", http.StatusFound)
			return nil
		}

		s.publicOnly(route, h)
	}

	// Reset password endpoints
	{
		const route = "GET /password-resets"
		h := newHandler(s, func(ctx context.Context, token auth.EmailTokenRaw) (auth.EmailTokenRaw, error) {
			// this target function ensures the input is validated before it's forwared to the view.
			return token, nil
		})
		h.onSuccess = func(r result[auth.EmailTokenRaw, auth.EmailTokenRaw]) error {
			s.writeView(r.w, r.r, "reset-password", r.out)
			return nil
		}

		s.publicOnly(route, h)
	}

	{
		const route = "POST /password-resets"
		h := newInputHandler(s, deps.AuthService.ResetPassword)
		h.onFail = func(r shared, err error) {
			s.writeErrorView(r.w, r.r, "reset-password", err)
		}
		h.onSuccess = func(r result[auth.NewPassword, struct{}]) error {
			r.sess.AddFlash("Your password was reset, login with your new password below")
			s.writeRedirect(r.w, r.r, "/login", http.StatusFound)
			return nil
		}

		s.publicOnly(route, h)
	}

	// Dashboard endpoints
	s.loggedIn("GET /dashboard", newViewHandler(s, "dashboard"))

	// Static frontend files endpoint.
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(s.deps.DistFS)))

	// Below we set up the global middlewares.
	//
	// Wrap the mux with global middlewares.
	csrfMW := csrf.Protect(
		cfg.CSRFKey.SecretValue(),
		csrf.CookieName(csrfTokenCookieName),
		csrf.FieldName(csrfTokenField),
		csrf.Secure(cfg.SecureCookie),
	)

	middlewares := []func(http.Handler) http.Handler{
		csrfMW,
		sessionMiddleware(s),
	}
	s.handler = s.mux
	for i := len(middlewares) - 1; i >= 0; i-- {
		s.handler = middlewares[i](s.handler)
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// preWrite should always be called before writing a response, the returned value indicates
// if it was successful.
func (s *Server) preWrite(w http.ResponseWriter, r *http.Request) bool {
	sess, err := sessionFromCtx(r.Context())
	if err != nil {
		s.deps.Logger.Error("failed to get session from context", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return false
	}

	if sess.NeedsSave() {
		err = s.deps.SessionStore.Save(r, w, sess)
		if err != nil {
			s.deps.Logger.Error("failed to save session", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return false
		}
	}

	return true
}

func (s *Server) writeRedirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	if !s.preWrite(w, r) {
		return
	}

	http.Redirect(w, r, url, code)
}

func (s *Server) writeView(w http.ResponseWriter, r *http.Request, name string, data any) {
	vd := s.prepViewData(r, w, data)
	if vd == nil {
		return
	}

	if !s.preWrite(w, r) {
		return
	}

	s.renderView(w, name, vd)
}

func (s *Server) writeErrorView(w http.ResponseWriter, r *http.Request, name string, err error) {
	vd := s.prepViewData(r, w, nil)
	if vd == nil {
		return
	}

	if !s.preWrite(w, r) {
		return
	}

	if errors.Is(err, errorz.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		s.renderView(w, name, vd)
		return
	}

	var invalidInput errorz.InvalidInput
	if errors.As(err, &invalidInput) {
		vd.InputErrors = invalidInput
		w.WriteHeader(http.StatusBadRequest)
		s.renderView(w, name, vd)
		return
	}

	s.deps.Logger.Error("internal server error", "url", r.URL.String(), "error", err)
	w.WriteHeader(http.StatusInternalServerError)
	s.renderView(w, name, vd)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	s.writeErrorView(w, r, "error", err)
}

func (s *Server) renderView(w http.ResponseWriter, name string, vd *viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := s.deps.ViewRenderer.Render(w, name, vd)
	if err != nil {
		s.deps.Logger.Error("failed to render view", "error", err)
	}
}
