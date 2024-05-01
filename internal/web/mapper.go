package web

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"
)

// mapper is a generic HTTP handler that maps:
// - Incoming HTTP requests to a target function.
// - Return values and errors to a response.
// The mapping logic is customizable.
type mapper[IN, OUT any] struct {
	s *Server
	// reqToInFunc will be called to convert s.r to a value of type IN.
	reqToInFunc func(s shared) (IN, error)
	// targetFunc is the function that will be called with c.in as input.
	targetFunc func(context.Context, IN) (OUT, error)
	// successFunc will be called if targetFunc was successful, it will
	successFunc func(c result[IN, OUT]) error
	// failFunc will be called if reqToInFunc or targetFunc failed.
	// By default this will call the standard server error handler.
	failFunc func(c shared, err error)
}

type shared struct {
	s    *Server
	r    *http.Request
	sess *sessions.Session
	w    http.ResponseWriter
}

type result[IN, OUT any] struct {
	shared
	in  IN
	out OUT
}

// newHandler creates a HTTP Handler that:
// 1. Maps the request to a value of input type IN.
// 2. Calls the newHandler func with that value.
// 3. Writes the output of type OUT to the response with status 200.
//
// Errors are written using the server error handler.
func newHandler[IN, OUT any](s *Server, targetFunc func(context.Context, IN) (OUT, error)) *mapper[IN, OUT] {
	return &mapper[IN, OUT]{
		s:           s,
		reqToInFunc: defaultReqToIn[IN],
		targetFunc:  targetFunc,
		successFunc: defaultSuccess[IN, OUT],
		failFunc: func(s shared, err error) {
			s.s.handleError(s.w, s.r, err)
		},
	}
}

// newInputHandler creates a HTTP Handler that:
// 1. Maps the request to a value of type IN.
// 2. Calls the target func with that value.
// 3. Writes a status 200 response to the client if target func was successful.
//
// Errors are written using the server error handler.
func newInputHandler[IN any](s *Server, targetFunc func(context.Context, IN) error) *mapper[IN, struct{}] {
	return &mapper[IN, struct{}]{
		s:           s,
		reqToInFunc: defaultReqToIn[IN],
		targetFunc: func(ctx context.Context, in IN) (struct{}, error) {
			err := targetFunc(ctx, in)
			if err != nil {
				return struct{}{}, err
			}

			return struct{}{}, nil
		},
		successFunc: defaultSuccess[IN, struct{}],
		failFunc: func(s shared, err error) {
			s.s.handleError(s.w, s.r, err)
		},
	}
}

// onSuccess provides custom logic to deal with successful target function calls.
func (e *mapper[IN, OUT]) onSuccess(fn func(result[IN, OUT]) error) *mapper[IN, OUT] {
	e.successFunc = fn
	return e
}

// onFail provides custom logic to deal with errors.
func (e *mapper[IN, OUT]) onFail(fn func(shared, error)) *mapper[IN, OUT] {
	e.failFunc = fn
	return e
}

func (e *mapper[IN, OUT]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, err := sessionFromCtx(r.Context())
	if err != nil {
		// no shared data yet, use the default error handler.
		e.s.handleError(w, r, err)
		return
	}

	s := shared{
		s:    e.s,
		r:    r,
		sess: sess,
		w:    w,
	}

	in, err := e.reqToInFunc(s)
	if err != nil {
		e.failFunc(s, err)
		return
	}

	out, err := e.targetFunc(r.Context(), in)
	if err != nil {
		e.failFunc(s, err)
		return
	}

	pc := result[IN, OUT]{
		shared: s,
		in:     in,
		out:    out,
	}

	err = e.successFunc(pc)
	if err != nil {
		e.failFunc(s, err)
		return
	}
}

// defaultReqToIn is the default way to map a request to a struct.
func defaultReqToIn[IN any](s shared) (IN, error) {
	var in IN
	err := s.r.ParseForm()
	if err != nil {
		return in, err
	}

	// Remove the CSRF token from the form, it won't need to be mapped
	// to any target types and the decoder will fail on it.
	s.r.Form.Del(csrfTokenField)

	err = s.s.decoder.Decode(&in, s.r.Form)
	if err != nil {
		return in, err
	}

	return in, nil
}

// defaultSuccess is the default way to write a response to the client.
func defaultSuccess[IN, OUT any](result[IN, OUT]) error {
	// TODO: Implement.
	return nil
}
