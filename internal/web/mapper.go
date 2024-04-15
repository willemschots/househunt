package web

import (
	"context"
	"net/http"
)

// mapper is a generic HTTP handler that maps requests to target
// function calls and writes the output to the response.
type mapper[IN, OUT any] struct {
	s      *Server
	req    func(*http.Request) (IN, error)
	target func(context.Context, IN) (OUT, error)
	res    func(result[IN, OUT]) error
}

// result is the result of a succesful request.
// it contains all relevant data because we can't know
// in advance what we will need to construct a response.
type result[IN, OUT any] struct {
	s   *Server
	r   *http.Request
	w   http.ResponseWriter
	in  IN
	out OUT
}

// mapBoth creates a HTTP Handler that:
// 1. Maps the request to a value of input type IN.
// 2. Calls the mapBoth func with that value.
// 3. Writes the output of type OUT to the response with status 200.
//
// Errors are written using the server error handler.
func mapBoth[IN, OUT any](s *Server, targetFunc func(context.Context, IN) (OUT, error)) *mapper[IN, OUT] {
	return &mapper[IN, OUT]{
		s: s,
		req: func(r *http.Request) (IN, error) {
			return defaultRequest[IN](s, r)
		},
		target: targetFunc,
		res: func(r result[IN, OUT]) error {
			return defaultResponse(r)
		},
	}
}

// mapRequest creates a HTTP Handler that:
// 1. Maps the request to a value of type IN.
// 2. Calls the target func with that value.
// 3. Writes a status 200 response to the client if target func was successful.
//
// Errors are written using the server error handler.
func mapRequest[IN any](s *Server, targetFunc func(context.Context, IN) error) *mapper[IN, struct{}] {
	return &mapper[IN, struct{}]{
		s: s,
		req: func(r *http.Request) (IN, error) {
			return defaultRequest[IN](s, r)
		},
		target: func(ctx context.Context, in IN) (struct{}, error) {
			err := targetFunc(ctx, in)
			if err != nil {
				return struct{}{}, err
			}

			return struct{}{}, nil
		},
		res: func(r result[IN, struct{}]) error {
			// TODO: actually write a response.
			return nil
		},
	}
}

// mapResponse creates a HTTP Handler that:
// 1. Calls the target func.
// 2. Maps the returned value of type OUT to the response with a status 200.
//
// Errors are written using the server error handler.
func mapResponse[OUT any](s *Server, targetFunc func(context.Context) (OUT, error)) *mapper[struct{}, OUT] {
	return &mapper[struct{}, OUT]{
		s: s,
		req: func(r *http.Request) (struct{}, error) {
			return struct{}{}, nil
		},
		target: func(ctx context.Context, _ struct{}) (OUT, error) {
			out, err := targetFunc(ctx)
			if err != nil {
				return out, err
			}

			return out, nil
		},
		res: func(r result[struct{}, OUT]) error {
			return defaultResponse(r)
		},
	}
}

// request overwrites the function that maps the request to the input type.
func (e *mapper[IN, OUT]) request(fn func(r *http.Request) (IN, error)) *mapper[IN, OUT] {
	e.req = fn
	return e
}

// response overwrites the function that writes the output to the response.
func (e *mapper[IN, OUT]) response(fn func(result[IN, OUT]) error) *mapper[IN, OUT] {
	e.res = fn
	return e
}

func (e *mapper[IN, OUT]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in, err := e.req(r)
	if err != nil {
		e.s.handleError(w, err)
		return
	}

	out, err := e.target(r.Context(), in)
	if err != nil {
		e.s.handleError(w, err)
		return
	}

	result := result[IN, OUT]{
		s:   e.s,
		r:   r,
		w:   w,
		in:  in,
		out: out,
	}

	err = e.res(result)
	if err != nil {
		e.s.handleError(w, err)
		return
	}
}

// defaultRequest is the default way to map a request to a struct.
func defaultRequest[IN any](s *Server, r *http.Request) (IN, error) {
	var in IN
	err := r.ParseForm()
	if err != nil {
		return in, err
	}

	err = s.decoder.Decode(&in, r.Form)
	if err != nil {
		return in, err
	}

	return in, nil
}

// defaultResponse is the default way to write a response to the client.
func defaultResponse[IN, OUT any](result[IN, OUT]) error {
	// TODO: Implement.
	return nil
}
