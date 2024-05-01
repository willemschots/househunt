package web

import (
	"net/http"

	"github.com/willemschots/househunt/internal/errorz"
)

const (
	authCookieName      = "hh-auth"
	csrfTokenCookieName = "csrf"
	csrfTokenField      = "csrfToken"
)

func (s *Server) public(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *Server) publicOnly(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := sessionFromCtx(r.Context())
		if err != nil {
			s.writeError(w, r, err)
			return
		}

		_, ok := sess.UserID()
		if ok {
			s.writeError(w, r, errorz.ErrNotFound)
			return
		}

		handler.ServeHTTP(w, r)
	}))
}

func (s *Server) loggedIn(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := sessionFromCtx(r.Context())
		if err != nil {
			s.writeError(w, r, err)
			return
		}

		_, ok := sess.UserID()
		if !ok {
			s.writeError(w, r, errorz.ErrNotFound)
			return
		}

		handler.ServeHTTP(w, r)
	}))
}
