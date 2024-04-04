package krypto_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/willemschots/househunt/internal/krypto"
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

type argon2HashTest struct {
	raw     string
	hashStr string
	hash    krypto.Argon2Hash
}

func okTextToArgon2Hash() map[string]argon2HashTest {
	return map[string]argon2HashTest{
		"ascii": {
			raw:     "12345678",
			hashStr: "$argon2id$v=19$m=47104,t=1,p=1$vP9U4C5jsOzFQLj0gvUkYw$YLrSb2dGfcVohlm8syynqHs6/NHxXS9rt/t6TjL7pi0",
			hash: krypto.Argon2Hash{
				Variant:     "argon2id",
				Version:     19,
				MemoryKiB:   47104,
				Iterations:  1,
				Parallelism: 1,
				Salt: []byte{
					0xbc, 0xff, 0x54, 0xe0, 0x2e, 0x63, 0xb0, 0xec,
					0xc5, 0x40, 0xb8, 0xf4, 0x82, 0xf5, 0x24, 0x63,
				},
				Hash: []byte{
					0x60, 0xba, 0xd2, 0x6f, 0x67, 0x46, 0x7d, 0xc5,
					0x68, 0x86, 0x59, 0xbc, 0xb3, 0x2c, 0xa7, 0xa8,
					0x7b, 0x3a, 0xfc, 0xd1, 0xf1, 0x5d, 0x2f, 0x6b,
					0xb7, 0xfb, 0x7a, 0x4e, 0x32, 0xfb, 0xa6, 0x2d,
				},
			},
		},
		"non-ascii": {
			raw:     "🥸🥸🥸",
			hashStr: "$argon2id$v=19$m=47104,t=1,p=1$CkX5zzYLJMWm0y/17eScyw$Qfah+NewdsdeF0+iV72mShZhRO93Qwzdj17TUZCH6ZU",
			hash: krypto.Argon2Hash{
				Variant:     "argon2id",
				Version:     19,
				MemoryKiB:   47104,
				Iterations:  1,
				Parallelism: 1,
				Salt: []byte{
					0xa, 0x45, 0xf9, 0xcf, 0x36, 0xb, 0x24, 0xc5,
					0xa6, 0xd3, 0x2f, 0xf5, 0xed, 0xe4, 0x9c, 0xcb,
				},
				Hash: []byte{
					0x41, 0xf6, 0xa1, 0xf8, 0xd7, 0xb0, 0x76, 0xc7,
					0x5e, 0x17, 0x4f, 0xa2, 0x57, 0xbd, 0xa6, 0x4a,
					0x16, 0x61, 0x44, 0xef, 0x77, 0x43, 0xc, 0xdd,
					0x8f, 0x5e, 0xd3, 0x51, 0x90, 0x87, 0xe9, 0x95,
				},
			},
		},
	}
}

func Test_Argon2Hash_HashArgon2AndMatch(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			// Rehash the raw value.
			got, err := krypto.HashArgon2([]byte(tc.raw))
			if err != nil {
				t.Fatalf("failed to hash argon2: %v", err)
			}

			// The new hash and the existing hash should not be equal because of the random salt.
			if reflect.DeepEqual(got, tc.hash) {
				t.Errorf("did not expect\n%#v\nto equal\n%#v\n", got, tc.hash)
			}

			// Additionaly, the raw value should match the new hash.
			if !got.MatchBytes([]byte(tc.raw)) {
				t.Errorf("expected raw value to match hash, but it did not")
			}
		})
	}

	failTests := map[string][]byte{
		"fail, nil":   {},
		"fail, empty": {},
	}

	for name, raw := range failTests {
		t.Run(name, func(t *testing.T) {
			_, err := krypto.HashArgon2(raw)
			if !errors.Is(err, krypto.ErrInvalidInput) {
				t.Fatalf("expected %v, but got %v (via errors.Is)", krypto.ErrInvalidInput, err)
			}
		})
	}
}

func Test_Argon2Hash_ParseArgon2HashAndMatch(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			got, err := krypto.ParseArgon2Hash(tc.hashStr)
			if err != nil {
				t.Fatalf("failed to parse argon2 hash: %v", err)
			}

			// The parsed hash should match the existing hash exactly.
			if !reflect.DeepEqual(got, tc.hash) {
				t.Errorf("wanted\n%#v\nbut got\n%#v\n", got, tc.hash)
			}

			// The raw value should match the parsed hash.
			if !got.MatchBytes([]byte(tc.raw)) {
				t.Errorf("expected raw value to match hash, but it did not")
			}
		})
	}

	for name, txt := range failTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			_, err := krypto.ParseArgon2Hash(txt)
			if !errors.Is(err, krypto.ErrInvalidInput) {
				t.Errorf("expected error to match (using errors.Is)\n%v\ngot\n%v\n", krypto.ErrInvalidInput, err)
			}
		})
	}
}

func Test_Argon2Hash_String(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			got := tc.hash.String()
			if got != tc.hashStr {
				t.Errorf("got\n%s\nwant\n%s\n", got, tc.hashStr)
			}
		})
	}
}

func Test_Argon2Hash_MarshalText(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
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

func Test_Argon2Hash_UnmarshalText(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			var got krypto.Argon2Hash
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
			var got krypto.Argon2Hash
			err := got.UnmarshalText([]byte(txt))
			if !errors.Is(err, krypto.ErrInvalidInput) {
				t.Errorf("expected errors to match (using errors.Is)\n%v\ngot\n%v\n", krypto.ErrInvalidInput, err)
			}
		})
	}
}

func Test_Argon2Hash_Scan(t *testing.T) {
	for name, tc := range okTextToArgon2Hash() {
		t.Run(name, func(t *testing.T) {
			var got krypto.Argon2Hash
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
			var got krypto.Argon2Hash
			err := got.Scan(txt)
			if !errors.Is(err, krypto.ErrInvalidInput) {
				t.Errorf("expected errors to match (using errors.Is)\n%v\ngot\n%v\n", krypto.ErrInvalidInput, err)
			}
		})
	}

	t.Run("fail, not a string", func(t *testing.T) {
		var got krypto.Argon2Hash
		err := got.Scan(42)
		if err == nil {
			t.Fatalf("expected error to be non-nil")
		}
	})
}
