package errorz

import "strings"

// InvalidInput signals that a provided input is invalid due to the wrapped errors.
type InvalidInput []error

func (e InvalidInput) Error() string {
	var b strings.Builder
	b.WriteString("invalid input:\n")
	for _, err := range e {
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return b.String()
}

func (e InvalidInput) Unwrap() []error {
	return e
}
