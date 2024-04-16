package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/willemschots/househunt/internal/errorz"
)

const (
	AuthSession = "hh-auth"
)

func (s *Server) public(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *Server) publicOnly(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := UserIDFromContext(r.Context())
		if ok {
			s.handleError(w, errorz.ErrNotFound)
			return
		}

		handler.ServeHTTP(w, r)
	}))
}

func (s *Server) loggedIn(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		_, ok := UserIDFromContext(r.Context())
		if !ok {
			s.handleError(w, errorz.ErrNotFound)
			return
		}

		handler.ServeHTTP(w, r)
	}))
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

func (s *Server) stopAuthSession(w http.ResponseWriter, r *http.Request) error {
	session, err := s.deps.SessionStore.Get(r, AuthSession)
	if err != nil {
		return err
	}

	if session.IsNew {
		return errorz.ErrNotFound
	}

	session.Options.MaxAge = -1 // Setting the age in the past will delete the cookie.
	return s.deps.SessionStore.Save(r, w, session)
}

func (s *Server) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.readAuthSession(r)

		switch {
		case err == nil:
			// Existing session.
			ctx := ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		case errors.Is(err, errorz.ErrNotFound):
			// No session.
			next.ServeHTTP(w, r)
		default:
			// Unexpected error
			s.handleError(w, err)
			return
		}
	})
}

type ctxKey string

const userIDKey ctxKey = "househuntUserID"

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}

	return userID, true
}
