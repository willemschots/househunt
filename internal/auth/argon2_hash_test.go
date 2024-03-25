package auth_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/willemschots/househunt/internal/auth"
)

func failTextToArgon2Hash() map[string]string {
	return map[string]string{
		"fail, wrong variant":           "$argon2i$v=19$m=47104,t=1,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-numeric version":     "$argon2id$v=abc$m=47104,t=1,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-matching version":    "$argon2id$v=18$m=47104,t=1,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-numeric memory":      "$argon2id$v=19$m=abc,t=1,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-numeric iterations":  "$argon2id$v=19$m=47104,t=abc,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-numeric parallelism": "$argon2id$v=19$m=47104,t=1,p=abc$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-base64 salt":         "$argon2id$v=19$m=47104,t=1,p=1$???????????????????????????????????????????$DVpK1dNdPRmhL8oTSo+RlA",
		"fail, non-base64 hash":         "$argon2id$v=19$m=47104,t=1,p=1$fYJT8cAysfuYCBjxTEmCkaCz0RfRtlLQOw2Fj8gM5Uw$??????????????????????",
	}
}

func Test_Argon2Hash_String(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			got := tc.hash.String()
			if got != tc.hashStr {
				t.Errorf("got\n%s\nwant\n%s\n", got, tc.hashStr)
			}
		})
	}
}

func Test_Argon2Hash_MarshalText(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			got, err := tc.hash.MarshalText()
			if err != nil {
				t.Fatalf("failed to marshal text: %v", err)
			}

			if string(got) != tc.hashStr {
				t.Errorf("got\n%s\nwant\n%s\n", got, tc.hashStr)
			}
		})
	}
}

func Test_Argon2Hash_ParseArgon2Hash(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			got, err := auth.ParseArgon2Hash(tc.hashStr)
			if err != nil {
				t.Fatalf("failed to parse argon2 hash: %v", err)
			}

			if !reflect.DeepEqual(got, tc.hash) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, tc.hash)
			}
		})
	}

	for name, txt := range failTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			_, err := auth.ParseArgon2Hash(txt)
			if !errors.Is(err, auth.ErrInvalidArgon2Hash) {
				t.Errorf("expected error to match (using errors.Is)\n%v\ngot\n%v\n", auth.ErrInvalidArgon2Hash, err)
			}
		})
	}
}

func Test_Argon2Hash_UnmarshalText(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			var got auth.Argon2Hash
			err := got.UnmarshalText([]byte(tc.hashStr))
			if err != nil {
				t.Fatalf("failed to unmarshal text to argon2 hash: %v", err)
			}

			if !reflect.DeepEqual(got, tc.hash) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, tc.hash)
			}
		})
	}

	for name, txt := range failTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			var got auth.Argon2Hash
			err := got.UnmarshalText([]byte(txt))
			if !errors.Is(err, auth.ErrInvalidArgon2Hash) {
				t.Errorf("expected errors to match (using errors.Is)\n%v\ngot\n%v\n", auth.ErrInvalidArgon2Hash, err)
			}
		})
	}
}

func Test_Argon2Hash_Scan(t *testing.T) {
	for name, tc := range passwordTests() {
		t.Run(name, func(t *testing.T) {
			var got auth.Argon2Hash
			err := got.Scan(tc.hashStr)
			if err != nil {
				t.Fatalf("failed to scan to argon2 hash: %v", err)
			}

			if !reflect.DeepEqual(got, tc.hash) {
				t.Errorf("got\n%#v\nwant\n%#v\n", got, tc.hash)
			}
		})
	}

	for name, txt := range failTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			var got auth.Argon2Hash
			err := got.Scan(txt)
			if !errors.Is(err, auth.ErrInvalidArgon2Hash) {
				t.Errorf("expected errors to match (using errors.Is)\n%v\ngot\n%v\n", auth.ErrInvalidArgon2Hash, err)
			}
		})
	}

	t.Run("fail, not a string", func(t *testing.T) {
		var got auth.Argon2Hash
		err := got.Scan(42)
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
	})
}
