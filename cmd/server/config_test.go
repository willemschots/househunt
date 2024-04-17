package main

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/krypto"
)

func requiredEnv() map[string]string {
	return map[string]string{
		"HTTP_COOKIE_KEYS":    "568554094ec040ab8a6b3e6d7cc138b0dc855f39ba1aeb2ffc903f7260b3a452,d503685b5e0848dcd1026711a5d92e8a087dfaffa489fb563e0de73db2f2476c",
		"HTTP_CSRF_KEY":       "dfab77e26917c6e37a173690443a0016808ef7b24e32424d45cd83454198a6ec",
		"DB_BLIND_INDEX_SALT": "b61115eeb1bdf0847f1d7ea978c7da71e3b31361f7450bc8aa12566a16b7b03f",
		"DB_ENCRYPTION_KEYS":  "2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d",
		"EMAIL_FROM":          "househunt@example.com",
	}
}

func newConfig(mf func(*config)) config {
	c := defaultConfig()
	c.http.cookieKeys = []krypto.Key{
		must(krypto.ParseKey("568554094ec040ab8a6b3e6d7cc138b0dc855f39ba1aeb2ffc903f7260b3a452")),
		must(krypto.ParseKey("d503685b5e0848dcd1026711a5d92e8a087dfaffa489fb563e0de73db2f2476c")),
	}
	c.http.server.CSRFKey = must(krypto.ParseKey("dfab77e26917c6e37a173690443a0016808ef7b24e32424d45cd83454198a6ec"))
	c.db.blindIndexSalt = must(krypto.ParseKey("b61115eeb1bdf0847f1d7ea978c7da71e3b31361f7450bc8aa12566a16b7b03f"))
	c.db.encryptionKeys = []krypto.Key{
		must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
	}
	c.email.service.From = must(email.ParseAddress("househunt@example.com"))

	if mf != nil {
		mf(&c)
	}
	return c
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("ok, uses defaults for non-required env variables", func(t *testing.T) {
		// set the required env variables.
		for key, val := range requiredEnv() {
			envForTest(t, key, val)
		}

		want := newConfig(nil)
		got, err := configFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got\n%+v\nwant\n%+v", got, want)
		}
	})

	valid := map[string]struct {
		key string
		val string
		mf  func(*config) // modify default config to create wanted config.
	}{
		"ok, non-default BASE_URL": {
			key: "BASE_URL",
			val: "https://example.com:9999",
			mf: func(c *config) {
				c.email.service.BaseURL = must(url.Parse("https://example.com:9999"))
			},
		},
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
		"ok, other HTTP_COOKIE_KEYS": {
			key: "HTTP_COOKIE_KEYS",
			val: "04017690e77c6a19671178e1950c7519389b58f6ffb8dcf53b2acfcaca398778,ddadbbe8b69c757875b80b8522a40da8ed882a1a368160247ef769acad61f88a",
			mf: func(c *config) {
				c.http.cookieKeys = []krypto.Key{
					must(krypto.ParseKey("04017690e77c6a19671178e1950c7519389b58f6ffb8dcf53b2acfcaca398778")),
					must(krypto.ParseKey("ddadbbe8b69c757875b80b8522a40da8ed882a1a368160247ef769acad61f88a")),
				}
			},
		},
		"ok, non-default HTTP_SECURE_COOKIE": {
			key: "HTTP_SECURE_COOKIE",
			val: "false",
			mf: func(c *config) {
				c.http.server.SecureCookie = false
			},
		},
		"ok, other HTTP_CSRF_KEY": {
			key: "HTTP_CSRF_KEY",
			val: "218dbd640d2ae9bd7a81e45f1ad963ecea3027fea21b9c3b93ca3ad69915f733",
			mf: func(c *config) {
				c.http.server.CSRFKey = must(krypto.ParseKey("218dbd640d2ae9bd7a81e45f1ad963ecea3027fea21b9c3b93ca3ad69915f733"))
			},
		},
		"ok, non-default DB_FILENAME": {
			key: "DB_FILENAME", val: "test.db", mf: func(c *config) { c.db.file = "test.db" },
		},
		"ok, non-default DB_MIGRATE": {
			key: "DB_MIGRATE", val: "false", mf: func(c *config) { c.db.migrate = false },
		},
		"ok, other DB_BLIND_INDEX_KEY": {
			key: "DB_BLIND_INDEX_SALT",
			val: "d1d92ba246dc05e7c1e935dd52d02272a218c7ea2ed514d1f68e7baa5f861ddd",
			mf: func(c *config) {
				c.db.blindIndexSalt = must(krypto.ParseKey("d1d92ba246dc05e7c1e935dd52d02272a218c7ea2ed514d1f68e7baa5f861ddd"))
			},
		},
		"ok, multiple DB_ENCRYPTION_KEYS": {
			key: "DB_ENCRYPTION_KEYS",
			val: "2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d,cf55b868d8c7a640265365910093113edce9b6c9226f3bd7c87987d23062d421",
			mf: func(c *config) {
				c.db.encryptionKeys = []krypto.Key{
					must(krypto.ParseKey("2b671594b775f371eab4050b4d58326682df6b1a6cc2e886717b1a26b4d6c45d")),
					must(krypto.ParseKey("cf55b868d8c7a640265365910093113edce9b6c9226f3bd7c87987d23062d421")),
				}
			},
		},
		"ok, non-default AUTH_WORKER_TIMEOUT": {
			key: "AUTH_WORKER_TIMEOUT", val: "42s", mf: func(c *config) { c.auth.WorkerTimeout = 42 * time.Second },
		},
		"ok, non-default AUTH_TOKEN_EXPIRY": {
			key: "AUTH_TOKEN_EXPIRY", val: "51m", mf: func(c *config) { c.auth.TokenExpiry = 51 * time.Minute },
		},
		"ok, non-default EMAIL_DRIVER": {
			key: "EMAIL_DRIVER",
			val: "postmark",
			mf: func(c *config) {
				c.email.driver = "postmark"
			},
		},
		"ok, other EMAIL_FROM": {
			key: "EMAIL_FROM",
			val: "test@example.com",
			mf: func(c *config) {
				c.email.service.From = must(email.ParseAddress("test@example.com"))
			},
		},
		"ok, non-default POSTMARK_API_URL": {
			key: "POSTMARK_API_URL",
			val: "https://example.com",
			mf: func(c *config) {
				c.email.postmark.APIURL = must(url.Parse("https://example.com"))
			},
		},
		"ok, other POSTMARK_MESSAGE_STREAM": {
			key: "POSTMARK_MESSAGE_STREAM",
			val: "other_stream",
			mf: func(c *config) {
				c.email.postmark.MessageStream = "other_stream"
			},
		},
		"ok, other POSTMARK_SERVER_TOKEN": {
			key: "POSTMARK_SERVER_TOKEN",
			val: "testToken",
			mf: func(c *config) {
				c.email.postmark.ServerToken = krypto.NewSecret("testToken")
			},
		},
	}

	for name, tc := range valid {
		t.Run(name, func(t *testing.T) {
			// set the required env variables.
			for key, val := range requiredEnv() {
				envForTest(t, key, val)
			}

			// set the tested env variable
			envForTest(t, tc.key, tc.val)

			want := newConfig(tc.mf)
			got, err := configFromEnv()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got\n%+v\nwant\n%+v", got, want)
			}
		})
	}

	invalid := map[string]struct {
		key string
		val string
	}{
		"fail, no host in BASE_URL":            {"BASE_URL", "/just-a-path"},
		"fail, negative HTTP_READ_TIMEOUT":     {"HTTP_READ_TIMEOUT", "-1ms"},
		"fail, negative HTTP_WRITE_TIMEOUT":    {"HTTP_WRITE_TIMEOUT", "-1ms"},
		"fail, negative HTTP_IDLE_TIMEOUT":     {"HTTP_IDLE_TIMEOUT", "-1ms"},
		"fail, negative HTTP_SHUTDOWN_TIMEOUT": {"HTTP_SHUTDOWN_TIMEOUT", "-1ms"},
		"fail, invalid HTTP_COOKIE_KEYS":       {"HTTP_COOKIE_KEYS", "abc"},
		"fail, invalid HTTP_SECURE_COOKIE":     {"HTTP_SECURE_COOKIE", "abc"},
		"fail, invalid HTTP_CSRF_KEY":          {"HTTP_CSRF_KEY", "abc"},
		"fail, empty DB_FILENAME":              {"DB_FILENAME", ""},
		"fail, invalid DB_MIGRATE":             {"DB_MIGRATE", "no!"},
		"fail, invalid DB_BLIND_INDEX_SALT":    {"DB_BLIND_INDEX_SALT", "abc"},
		"fail, empty DB_ENCRYPTION_KEYS":       {"DB_ENCRYPTION_KEYS", ""},
		"fail, invalid DB_ENCRYPTION_KEYS":     {"DB_ENCRYPTION_KEYS", "abc"},
		"fail, negative AUTH_WORKER_TIMEOUT":   {"AUTH_WORKER_TIMEOUT", "-1ms"},
		"fail, negative AUTH_TOKEN_EXPIRY":     {"AUTH_TOKEN_EXPIRY", "-1ms"},
		"fail, invalid EMAIL_FROM":             {"EMAIL_FROM", "@@"},
		"fail, invalid POSTMARK_API_URL":       {"POSTMARK_API_URL", "not-a-url"},
	}

	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			// set the required env variables.
			for key, val := range requiredEnv() {
				envForTest(t, key, val)
			}

			// set the tested env variable.
			envForTest(t, tc.key, tc.val)

			_, err := configFromEnv()
			if err == nil {
				t.Fatal("expected error, got <nil>")
			}

			// Check that the error message contains the invalid env variable.
			// These errors are immediately logged, so I'm fine comparing on a string level.
			msg := err.Error()
			if !strings.Contains(msg, tc.key) {
				t.Errorf("expected error message to mention %s, got %s", tc.key, msg)
			}
		})
	}

	for key := range requiredEnv() {
		t.Run(fmt.Sprintf("fail, env variable %s not set", key), func(t *testing.T) {
			// set all required env variables except the one being tested.
			for k, val := range requiredEnv() {
				if k != key {
					envForTest(t, k, val)
				}
			}

			_, err := configFromEnv()
			if err == nil {
				t.Fatal("expected error, got <nil>")
			}

			// Check that the error message contains the missing env variable.
			// These errors are immediately logged, so I'm fine comparing on a string level.
			msg := err.Error()
			if !strings.Contains(msg, key) {
				t.Errorf("expected error message to mention %s, got %s", key, msg)
			}
		})
	}

	t.Run("fail, multiple invalid env variables", func(t *testing.T) {
		// set the required env variables.
		for key, val := range requiredEnv() {
			envForTest(t, key, val)
		}

		// set two invalid env variables.
		envForTest(t, "HTTP_READ_TIMEOUT", "-1ms")
		envForTest(t, "HTTP_WRITE_TIMEOUT", "-1ms")

		_, err := configFromEnv()
		if err == nil {
			t.Error("expected error, got <nil>")
		}

		// Check that the error message contains both invalid env variables.
		// Again, these errors are immediately logged, so I'm fine comparing on a string level.
		msg := err.Error()
		for _, key := range []string{"HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT"} {
			if !strings.Contains(msg, key) {
				t.Errorf("expected error message to mention %s, got %s", key, msg)
			}
		}
	})
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
