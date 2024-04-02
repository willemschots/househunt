package db

import "strings"

// Query helps build SQL queries using bind parameters.
// Use Query to construct parts of a query and use Param to add bind parameters.
// The final query and parameters can be retrieved using the Get method.
//
// The zero value is ready to use.
type Query struct {
	b      strings.Builder
	params []any
}

// Query writes a non-parameterized part of a query.
func (q *Query) Query(s string) {
	q.b.WriteString(s)
}

// Param writes a parameterized part of a query.
func (q *Query) Param(v any) {
	q.b.WriteString("?")
	q.params = append(q.params, v)
}

// Params writes multiple parameterized parts of a query seperated by commas.
func (q *Query) Params(v ...any) {
	for i, p := range v {
		if i > 0 {
			q.b.WriteString(", ")
		}
		q.b.WriteString("?")
		q.params = append(q.params, p)
	}
}

// Get returns the constructed query and parameter values.
func (q *Query) Get() (string, []any) {
	return q.b.String(), q.params
}
