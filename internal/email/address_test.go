package email_test

import (
	"errors"
	"testing"

	"github.com/willemschots/househunt/internal/email"
)

func Test_ParseAddress(t *testing.T) {
	okTests := map[string]struct {
		raw  string
		want email.Address
	}{
		"shortest possible": {
			raw:  "a@b",
			want: "a@b",
		},
		"typical": {
			raw:  "alice@example.com",
			want: "alice@example.com",
		},
		"whitespace is trimmed": {
			raw:  " 	alice@example.com  ",
			want: "alice@example.com",
		},
	}

	for name, tc := range okTests {
		t.Run(name, func(t *testing.T) {
			got, err := email.ParseAddress(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}

	failTests := map[string]string{
		"empty":                 "",
		"whitespace only":       " 	",
		"missing @":             "alice.example.com",
		"missing domain":        "alice@",
		"missing local part":    "@example.com",
		"with name":             "Alice <alice@example.com>",
		"with name and comment": "Alice <alice@example.com>(comment)",
	}

	for name, raw := range failTests {
		t.Run(name, func(t *testing.T) {
			_, err := email.ParseAddress(raw)
			if !errors.Is(err, email.ErrInvalidEmail) {
				t.Fatalf("expected error to be email.ErrInvalidEmail via errors.Is, but got %v", err)
			}
		})
	}
}
