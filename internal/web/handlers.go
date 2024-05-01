package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/gorilla/schema"
	"github.com/willemschots/househunt/internal/errorz"
)

// newViewHandler creates a HTTP Handler that renders the view with the given name.
func newViewHandler(s *Server, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := s.writeView(w, r, name, nil)
		if err != nil {
			s.handleError(w, r, err)
			return
		}
	})
}

// newHandler creates a HTTP Handler that:
// 1. Maps the request to a value of input type IN.
// 2. Calls the newHandler func with that value.
// 3. Writes the output of type OUT to the response with status 200.
//
// Errors are written using the server error handler.
func newHandler[IN, OUT any](srv *Server, targetFunc func(context.Context, IN) (OUT, error)) *mapper[IN, OUT] {
	return &mapper[IN, OUT]{
		reqToInFunc: func(s shared) (IN, error) {
			return defaultReqToIn[IN](srv, s)
		},
		targetFunc: targetFunc,
		onSuccess: func(c result[IN, OUT]) error {
			return defaultSuccess[IN, OUT](srv, c)
		},
		onFail: func(s shared, err error) {
			srv.handleError(s.w, s.r, err)
		},
	}
}

// newInputHandler creates a HTTP Handler that:
// 1. Maps the request to a value of type IN.
// 2. Calls the target func with that value.
// 3. Writes a status 200 response to the client if target func was successful.
//
// Errors are written using the server error handler.
func newInputHandler[IN any](srv *Server, targetFunc func(context.Context, IN) error) *mapper[IN, struct{}] {
	return &mapper[IN, struct{}]{
		reqToInFunc: func(s shared) (IN, error) {
			return defaultReqToIn[IN](srv, s)
		},
		targetFunc: func(ctx context.Context, in IN) (struct{}, error) {
			err := targetFunc(ctx, in)
			if err != nil {
				return struct{}{}, err
			}

			return struct{}{}, nil
		},
		onSuccess: func(c result[IN, struct{}]) error {
			return defaultSuccess[IN, struct{}](srv, c)
		},
		onFail: func(s shared, err error) {
			srv.handleError(s.w, s.r, err)
		},
	}
}

// defaultReqToIn is the default way to map a request to a struct.
func defaultReqToIn[IN any](srv *Server, s shared) (IN, error) {
	var in IN
	err := s.r.ParseForm()
	if err != nil {
		return in, err
	}

	// Remove the CSRF token from the form, it won't need to be mapped
	// to any target types and the decoder will fail on it.
	s.r.Form.Del(csrfTokenField)

	err = srv.decoder.Decode(&in, s.r.Form)
	return in, decodeError(err)
}

func decodeError(err error) error {
	if err == nil {
		return nil
	}

	var multiErr schema.MultiError
	if errors.As(err, &multiErr) {
		var invalidInput errorz.InvalidInput
		for key, e := range multiErr {
			invalidInput = append(invalidInput, errorz.Keyed{
				Key: key,
				Err: e,
			})
		}

		return invalidInput
	}

	return err
}

// defaultSuccess is the default way to write a response to the client.
func defaultSuccess[IN, OUT any](_ *Server, _ result[IN, OUT]) error {
	// TODO: Implement.
	return nil
}
