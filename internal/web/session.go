package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/willemschots/househunt/internal/web/sessions"
)

// session is a middleware that creates a session and injects it in the context.
func sessionMiddleware(srv *Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := srv.deps.SessionStore.Get(r)
			if err != nil {
				srv.handleError(w, r, err)
				return
			}

			err = srv.deps.SessionStore.Save(r, w, sess)
			if err != nil {
				srv.handleError(w, r, err)
				return
			}

			ctx := ctxWithSession(r.Context(), sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type ctxKey string

const sessionCtxKey ctxKey = "_session"

func ctxWithSession(ctx context.Context, sess *sessions.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, sess)
}

func sessionFromCtx(ctx context.Context) (*sessions.Session, error) {
	sess, ok := ctx.Value(sessionCtxKey).(*sessions.Session)
	if !ok {
		return nil, fmt.Errorf("could not get session from context")
	}

	return sess, nil
}
