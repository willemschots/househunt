package krypto_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/krypto"
)

func Test_Secret_PreventExposure(t *testing.T) {
	raw := "my secret"
	secret := krypto.NewSecret(raw)

	assert := func(t *testing.T, s string) {
		t.Helper()
		if s != krypto.SecretMarker {
			t.Errorf("wanted\n%s\ngot\n%s\n", krypto.SecretMarker, s)
		}
	}

	t.Run("ok, fmt", func(t *testing.T) {
		assert(t, fmt.Sprintf("%s", secret)) //nolint:gosimple
		assert(t, fmt.Sprintf("%d", secret))
		assert(t, fmt.Sprintf("%v", secret))
		assert(t, fmt.Sprintf("%#v", secret))
	})

	t.Run("ok, marshal as text", func(t *testing.T) {
		b, err := secret.MarshalText()
		if err != nil {
			t.Fatalf("failed to marshal as text: %v", err)
		}

		assert(t, string(b))
	})

	t.Run("ok, log output", func(t *testing.T) {
		var buf bytes.Buffer

		logger := slog.New(slog.NewTextHandler(&buf, nil))

		logger.Info("attempting to log a secret", "secret", secret)

		s := buf.String()
		if !strings.Contains(s, krypto.SecretMarker) {
			t.Errorf("log output\n%s\ndoes not contain secret marker: %s", s, krypto.SecretMarker)
		}

		if strings.Contains(s, raw) {
			t.Errorf("log output\n%s\ncontains raw key: %s", s, raw)
		}
	})
}
