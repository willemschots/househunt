package krypto_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/willemschots/househunt/internal/krypto"
)

func Test_NewEncryptor(t *testing.T) {
	t.Run("fail, no keys", func(t *testing.T) {
		_, err := krypto.NewEncryptor(nil)
		if err == nil {
			t.Fatalf("wanted error, got <nil>")
		}
	})
}

func Test_Encryptor_EncryptAndDecrypt(t *testing.T) {
	okCases := map[string][]byte{
		"ok, minimum input": {0},
		"ok, typical input": []byte("my secret message"),
	}

	for name, raw := range okCases {
		t.Run(name, func(t *testing.T) {
			svc := must(krypto.NewEncryptor([]krypto.Key{
				must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			}))

			result, err := svc.Encrypt([]byte(raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			decrypted, err := svc.Decrypt(result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(decrypted, raw) {
				t.Fatalf("want %q, got %q", raw, decrypted)
			}
		})
	}

	invalidEncrypt := map[string][]byte{
		"nil":         nil,
		"empty slice": {},
	}

	for name, raw := range invalidEncrypt {
		t.Run(name, func(t *testing.T) {
			enc := must(krypto.NewEncryptor([]krypto.Key{
				must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			}))

			_, err := enc.Encrypt(raw)
			if !errors.Is(err, krypto.ErrInvalidData) {
				t.Fatalf("wanted error %v, got %v (via errors.Is)", krypto.ErrInvalidData, err)
			}
		})
	}

	t.Run("ok, multiple keys", func(t *testing.T) {
		enc := must(krypto.NewEncryptor([]krypto.Key{
			must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf")),
		}))

		raw := "my secret message"
		result, err := enc.Encrypt([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		decrypted, err := enc.Decrypt(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(decrypted) != raw {
			t.Fatalf("want %q, got %q", raw, decrypted)
		}
	})

	t.Run("ok, decrypt with older key", func(t *testing.T) {
		keys := []krypto.Key{
			must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf")),
		}

		// old encryptor only has the first key.
		encOld := must(krypto.NewEncryptor(keys[:1]))

		raw := "my secret message"
		result, err := encOld.Encrypt([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// new encryptor has both keys but should use the old key to decrypt.
		encNew := must(krypto.NewEncryptor(keys))

		decrypted, err := encNew.Decrypt(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(decrypted) != raw {
			t.Fatalf("want %q, got %q", raw, decrypted)
		}
	})

	t.Run("fail, no key for this index", func(t *testing.T) {
		keys := []krypto.Key{
			must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf")),
		}

		encOld := must(krypto.NewEncryptor(keys))
		encNew := must(krypto.NewEncryptor(keys[0:1])) // new encryptor only has the first key.

		raw := "my secret message"
		result, err := encOld.Encrypt([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = encNew.Decrypt(result)
		if !errors.Is(err, krypto.ErrUnknownKey) {
			t.Fatalf("wanted error %v, got %v (via errors.Is)", krypto.ErrUnknownKey, err)
		}
	})

	t.Run("fail, key was changed", func(t *testing.T) {
		keys := []krypto.Key{
			must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			must(krypto.ParseKey("90303dfed7994260ea4817a5ca8a392915cd401115b2f97495dadfcbcd14adbf")),
		}

		encOld := must(krypto.NewEncryptor(keys[:1]))
		encNew := must(krypto.NewEncryptor(keys[1:])) // both use a different key.

		raw := "my secret message"
		result, err := encOld.Encrypt([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = encNew.Decrypt(result)
		if err == nil {
			t.Fatalf("wanted error, got <nil>")
		}
	})

	invalidDecrypt := map[string][]byte{
		"nil":            nil,
		"empty slice":    {},
		"short of index": {0, 0, 0},
		"only index":     {0, 0, 0, 0},
		"short of nonce": {
			0, 0, 0, 0, 1, 1, 1, 1,
			1, 1, 1, 1, 1, 1, 1,
		},
		"only index and nonce": {
			0, 0, 0, 0, 1, 1, 1, 1,
			1, 1, 1, 1, 1, 1, 1, 1,
		},
	}

	for name, msg := range invalidDecrypt {
		t.Run(name, func(t *testing.T) {
			svc := must(krypto.NewEncryptor([]krypto.Key{
				must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
			}))

			_, err := svc.Decrypt(msg)
			if !errors.Is(err, krypto.ErrInvalidData) {
				t.Fatalf("wanted error %v, got %v (via errors.Is)", krypto.ErrInvalidData, err)
			}
		})
	}
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
