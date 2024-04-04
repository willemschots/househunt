package auth_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/auth"
	"github.com/willemschots/househunt/internal/krypto"
)

type passwordTest struct {
	raw     string
	hashStr string
}

func passwordTests() map[string]passwordTest {
	return map[string]passwordTest{
		"min length ascii": {
			raw:     "12345678",
			hashStr: "$argon2id$v=19$m=47104,t=1,p=1$vP9U4C5jsOzFQLj0gvUkYw$YLrSb2dGfcVohlm8syynqHs6/NHxXS9rt/t6TjL7pi0",
		},
		"max length ascii": {
			raw:     stringOfLen(512),
			hashStr: "$argon2id$v=19$m=47104,t=1,p=1$Ndxt51GJNM44qImzaA9REw$t3YTeou+mW65MzPn7n6kF2boqiO4z1LQl24PzXW7rEY",
		},
		"non-ascii": {
			raw:     "🥸🥸🥸",
			hashStr: "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU",
		},
	}
}

func stringOfLen(x int) string {
	alphabet := "1234567890abcdefghijklmnopqrstuvwxyz"
	out := make([]byte, x)
	for i := 0; i < x; i++ {
		out[i] = alphabet[i%len(alphabet)]
	}
	return string(out)
}

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

			otherHash := must(krypto.ParseArgon2Hash(tc.hashStr))

			// We also check if the password matches the other known hash.
			if !pwd.Match(otherHash) {
				t.Errorf("password\n%s\ndoes match other hash\n%+v", tc.raw, otherHash)
			}
		})
	}

	t.Run("ok, password does not match hash", func(t *testing.T) {
		pwd := must(auth.ParsePassword("reallyStrongPassword1"))

		hash, err := pwd.Hash()
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}

		other := must(auth.ParsePassword("reallyStrongPassword2"))
		if other.Match(hash) {
			t.Errorf("password\n%s\nshould not match hash\n%+v", other, hash)
		}
	})

	t.Run("ok, password matches hash with different settings", func(t *testing.T) {
		// Taken these settings from the tests in the argon2 package.
		hash := krypto.Argon2Hash{
			Variant:     "argon2id",
			Version:     19,
			MemoryKiB:   64,
			Iterations:  1,
			Parallelism: 1,
			Salt:        []byte("somesalt"),
			Hash:        must(hex.DecodeString("655ad15eac652dc59f7170a7332bf49b8469be1fdb9c28bb")),
		}

		pwd, err := auth.ParsePassword("password")
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
	pwd := must(auth.ParsePassword(raw))

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
