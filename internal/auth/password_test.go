package auth_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/auth"
)

func Test_Password_ParseHashMatch(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			pwd, err := auth.ParsePassword(tc.raw)
			if err != nil {
				t.Fatalf("failed to parse password: %v", err)
			}

			hash, err := pwd.Hash()
			if err != nil {
				t.Fatalf("failed to hash password: %v", err)
			}

			// We can't compare the resulting hash to a known value, because of the random salt,
			// so we check if the password matches its own hash instead.
			if !pwd.Match(hash) {
				t.Errorf("password\n%s\ndoes match own hash\n%+v", tc.raw, hash)
			}

			// We also check if the password matches the other known hash.
			if !pwd.Match(tc.hash) {
				t.Errorf("password\n%s\ndoes match other hash\n%+v", tc.raw, tc.hash)
			}
		})
	}

	t.Run("ok, password does not match hash", func(t *testing.T) {
		pwd, err := auth.ParsePassword("reallyStrongPassword1")
		if err != nil {
			t.Fatalf("failed to parse password: %v", err)
		}

		hash, err := pwd.Hash()
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		other, err := auth.ParsePassword("reallyStrongPassword2")
		if err != nil {
			t.Fatalf("failed to parse password: %v", err)
		}

		if other.Match(hash) {
			t.Errorf("password\n%s\nshould not match hash\n%+v", other, hash)
		}
	})

	t.Run("ok, password matches hash with different settings", func(t *testing.T) {
		// Taken these settings from the tests in the argon2 package.
		hash := auth.Argon2Hash{
			Variant:     "argon2id",
			Version:     19,
			MemoryKiB:   64,
			Iterations:  1,
			Parallelism: 1,
			Salt:        []byte("somesalt"),
			Hash:        mustHexDecodeString(t, "655ad15eac652dc59f7170a7332bf49b8469be1fdb9c28bb"),
		}

		pwd, err := auth.ParsePassword(string([]byte("password")))
		if err != nil {
			t.Fatalf("failed to parse password: %v", err)
		}

		if !pwd.Match(hash) {
			t.Errorf("password\n%s\ndoes not match hash\n%+v", pwd, hash)
		}
	})

	failParsing := map[string]string{
		"empty":     "",
		"too short": "1234567",
		"too long":  stringOfLen(513),
	}

	for name, raw := range failParsing {
		t.Run(name, func(t *testing.T) {
			_, err := auth.ParsePassword(raw)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func Test_Password_PreventExposure(t *testing.T) {
	raw := "12345678"
	pwd, err := auth.ParsePassword(raw)
	if err != nil {
		t.Fatalf("failed to parse password: %v", err)
	}

	assert := func(t *testing.T, s string) {
		t.Helper()
		if s != auth.SecretMarker {
			t.Errorf("wanted\n%s\ngot\n%s\n", auth.SecretMarker, s)
		}
	}

	t.Run("ok, fmt", func(t *testing.T) {
		assert(t, fmt.Sprintf("%s", pwd)) //nolint:gosimple
		assert(t, fmt.Sprintf("%d", pwd))
		assert(t, fmt.Sprintf("%v", pwd))
		assert(t, fmt.Sprintf("%#v", pwd))
	})

	t.Run("ok, marshal as text", func(t *testing.T) {
		b, err := pwd.MarshalText()
		if err != nil {
			t.Fatalf("failed to marshal as text: %v", err)
		}

		assert(t, string(b))
	})

	t.Run("ok, log output", func(t *testing.T) {
		var buf bytes.Buffer

		logger := slog.New(slog.NewTextHandler(&buf, nil))

		logger.Info("attempting to log a password", "password", pwd)

		s := buf.String()
		if !strings.Contains(s, auth.SecretMarker) {
			t.Errorf("log output\n%s\ndoes not contain secret marker: %s", s, auth.SecretMarker)
		}

		if strings.Contains(s, raw) {
			t.Errorf("log output\n%s\ncontains raw password: %s", s, raw)
		}
	})
}

func mustHexDecodeString(t *testing.T, str string) []byte {
	t.Helper()

	b, err := hex.DecodeString(str)
	if err != nil {
		t.Fatalf("failed to decode hex string: %v", err)
	}

	return b
}
