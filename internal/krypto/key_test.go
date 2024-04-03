package krypto_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/krypto"
)

func Test_ParseKey(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		_, err := krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	failCases := map[string]string{
		"empty string":          "",
		"too short":             "2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45",
		"too long":              "2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45da",
		"invalid hex character": "zb671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d",
	}

	for name, val := range failCases {
		t.Run(name, func(t *testing.T) {
			_, err := krypto.ParseKey(val)
			if err == nil {
				t.Fatalf("wanted error, got <nil>")
			}
		})
	}
}

func Test_Key_PreventExposure(t *testing.T) {
	raw := "2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d"
	key := must(krypto.ParseKey(raw))

	assert := func(t *testing.T, s string) {
		t.Helper()
		if s != krypto.SecretMarker {
			t.Errorf("wanted\n%s\ngot\n%s\n", krypto.SecretMarker, s)
		}
	}

	t.Run("ok, fmt", func(t *testing.T) {
		assert(t, fmt.Sprintf("%s", key)) //nolint:gosimple
		assert(t, fmt.Sprintf("%d", key))
		assert(t, fmt.Sprintf("%v", key))
		assert(t, fmt.Sprintf("%#v", key))
	})

	t.Run("ok, marshal as text", func(t *testing.T) {
		b, err := key.MarshalText()
		if err != nil {
			t.Fatalf("failed to marshal as text: %v", err)
		}

		assert(t, string(b))
	})

	t.Run("ok, log output", func(t *testing.T) {
		var buf bytes.Buffer

		logger := slog.New(slog.NewTextHandler(&buf, nil))

		logger.Info("attempting to log a key", "key", key)

		s := buf.String()
		if !strings.Contains(s, krypto.SecretMarker) {
			t.Errorf("log output\n%s\ndoes not contain secret marker: %s", s, krypto.SecretMarker)
		}

		if strings.Contains(s, raw) {
			t.Errorf("log output\n%s\ncontains raw key: %s", s, raw)
		}
	})
}
