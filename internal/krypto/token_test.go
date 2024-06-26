package krypto_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/krypto"
)

func failTextToToken() map[string]string {
	return map[string]string{
		"fail, empty":              "",
		"fail, too short":          "010203040506070809101112131415161718192021222324252627282930313",
		"fail, too long":           "01020304050607080910111213141516171819202122232425262728293031323",
		"fail, non-hex characters": "010203040506070809101112131415161718192021222324252627282930313g",
	}
}

func Test_Token_GenerateToken(t *testing.T) {
	t.Run("ok, generate a token", func(t *testing.T) {
		tok, err := krypto.GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(tok) != 32 {
			t.Fatalf("got %d want %d tok(%v)", len(tok), 32, tok)
		}
	})
}

func Test_Token_ParseString(t *testing.T) {
	t.Run("ok, valid", func(t *testing.T) {
		want := krypto.Token{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
			0x17, 0x18, 0x19, 0x20, 0x21, 0x22, 0x23, 0x24,
			0x25, 0x26, 0x27, 0x28, 0x29, 0x30, 0x31, 0x32,
		}

		raw := "0102030405060708091011121314151617181920212223242526272829303132"
		got, err := krypto.ParseToken(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != want {
			t.Fatalf("got\n%v\nwant\n%v\n", got, want)
		}

		if got.String() != raw {
			t.Fatalf("got\n%s\nwant\n%s\n", got.String(), raw)
		}
	})

	for name, raw := range failTextToToken() {
		t.Run(name, func(t *testing.T) {
			_, err := krypto.ParseToken(raw)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !errors.Is(err, krypto.ErrInvalidToken) {
				t.Fatalf("expected error %v, got %v ", krypto.ErrInvalidToken, err)
			}
		})
	}
}

func Test_Token_PreventExposure(t *testing.T) {
	t.Run("ok, log output", func(t *testing.T) {
		tok, err := krypto.GenerateToken()
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		var buf bytes.Buffer

		logger := slog.New(slog.NewTextHandler(&buf, nil))

		logger.Info("attempting to log a password", "password", tok)

		s := buf.String()
		if !strings.Contains(s, krypto.SecretMarker) {
			t.Errorf("log output\n%s\ndoes not contain secret marker: %s", s, krypto.SecretMarker)
		}

		raw := tok.String()
		if strings.Contains(s, raw) {
			t.Errorf("log output\n%s\ncontains raw password: %s", s, raw)
		}
	})
}
