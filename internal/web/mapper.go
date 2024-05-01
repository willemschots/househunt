package web

import (
	"context"
	"net/http"

	"github.com/willemschots/househunt/internal/web/sessions"
)

// mapper is a generic HTTP handler that maps:
// - Incoming HTTP requests to a target function.
// - Return values and errors to a response.
// The mapping logic is customizable.
type mapper[IN, OUT any] struct {
	// reqToInFunc will be called to convert s.r to a value of type IN.
	reqToInFunc func(s shared) (IN, error)
	// targetFunc is the function that will be called with c.in as input.
	targetFunc func(context.Context, IN) (OUT, error)
	// onSuccess will be called if targetFunc was successful. Overwriting it
	// will allow you to customize the logic for successful responses.
	onSuccess func(c result[IN, OUT]) error
	// onFail will be called if reqToInFunc or targetFunc failed.
	// By default this will call the standard server error handler. Overwriting
	// it will allow you to customize the error responses.
	onFail func(c shared, err error)
}

type shared struct {
	r    *http.Request
	sess *sessions.Session
	w    http.ResponseWriter
}

type result[IN, OUT any] struct {
	shared
	in  IN
	out OUT
}

func (m *mapper[IN, OUT]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s := shared{
		r: r,
		w: w,
	}

	sess, err := sessionFromCtx(r.Context())
	if err != nil {
		m.onFail(s, err)
		return
	}

	s.sess = sess

	in, err := m.reqToInFunc(s)
	if err != nil {
		m.onFail(s, err)
		return
	}

	out, err := m.targetFunc(r.Context(), in)
	if err != nil {
		m.onFail(s, err)
		return
	}

	pc := result[IN, OUT]{
		shared: s,
		in:     in,
		out:    out,
	}

	err = m.onSuccess(pc)
	if err != nil {
		m.onFail(s, err)
		return
	}
}
