package email

import (
	"errors"
	"net/mail"
	"strings"
)

// ErrInvalidEmail indicates an email address is not valid.
var ErrInvalidEmail = errors.New("invalid email address")

// Address is how househunt represents email addresses.
type Address string

// ParseAddress parses the given string and checks if it's shaped like an email address.
// It returns an error if the input is not a valid email address.
// Note that this doesn't guarantee the email address actually exists, it only checks the format.
func ParseAddress(raw string) (Address, error) {
	trimmed := strings.TrimSpace(raw)

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return Address(""), ErrInvalidEmail
	}

	// mail.ParseAddress accepts addresses with names and comments:
	// "Alice <alice@example.com>(comment)".
	//
	// We only want to accept inputs that consist of the address part.
	if addr.Address != trimmed {
		return Address(""), ErrInvalidEmail
	}

	return Address(addr.Address), nil
}

func (a *Address) UnmarshalText(text []byte) error {
	addr, err := ParseAddress(string(text))
	if err != nil {
		return err
	}

	*a = addr

	return nil
}
