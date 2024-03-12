package main

import (
	"os"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	t.Run("ok, uses defaults for empty environment", func(t *testing.T) {
		want := defaultConfig()
		got, err := configFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want != got {
			t.Errorf("got\n%+v\nwant\n%+v", got, want)
		}
	})

	valid := map[string]struct {
		key string
		val string
		mf  func(*config) // modify default config to create wanted config.
	}{
		"ok, non-default HTTP_ADDR": {
			key: "HTTP_ADDR", val: "localhost:8080", mf: func(c *config) { c.http.addr = "localhost:8080" },
		},
		"ok, non-default HTTP_READ_TIMEOUT": {
			key: "HTTP_READ_TIMEOUT", val: "101ms", mf: func(c *config) { c.http.readTimeout = 101 * time.Millisecond },
		},
		"ok, non-default HTTP_WRITE_TIMEOUT": {
			key: "HTTP_WRITE_TIMEOUT", val: "202ms", mf: func(c *config) { c.http.writeTimeout = 202 * time.Millisecond },
		},
		"ok, non-default HTTP_IDLE_TIMEOUT": {
			key: "HTTP_IDLE_TIMEOUT", val: "303ms", mf: func(c *config) { c.http.idleTimeout = 303 * time.Millisecond },
		},
		"ok, non-default HTTP_SHUTDOWN_TIMEOUT": {
			key: "HTTP_SHUTDOWN_TIMEOUT", val: "404ms", mf: func(c *config) { c.http.shutdownTimeout = 404 * time.Millisecond },
		},
	}

	for name, tc := range valid {
		t.Run(name, func(t *testing.T) {
			want := defaultConfig()
			tc.mf(&want)

			envForTest(t, tc.key, tc.val)

			got, err := configFromEnv()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if want != got {
				t.Errorf("got\n%+v\nwant\n%+v", got, want)
			}
		})
	}

	invalid := map[string]struct {
		key string
		val string
	}{
		"fail, negative HTTP_READ_TIMEOUT":     {"HTTP_READ_TIMEOUT", "-1ms"},
		"fail, negative HTTP_WRITE_TIMEOUT":    {"HTTP_WRITE_TIMEOUT", "-1ms"},
		"fail, negative HTTP_IDLE_TIMEOUT":     {"HTTP_IDLE_TIMEOUT", "-1ms"},
		"fail, negative HTTP_SHUTDOWN_TIMEOUT": {"HTTP_SHUTDOWN_TIMEOUT", "-1ms"},
	}

	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			envForTest(t, tc.key, tc.val)

			_, err := configFromEnv()
			if err == nil {
				t.Error("expected error, got <nil>")
			}
		})
	}
}

// envForTest sets an environment variable for a test and unsets it when the test is done.
func envForTest(t *testing.T, key, val string) {
	t.Helper()

	t.Cleanup(func() {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("failed to unset env var %s: %v", key, err)
		}
	})

	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("failed to set env var %s: %v", key, err)
	}
}
