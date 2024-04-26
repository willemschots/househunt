package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// baseURL is used to identify the scheme, host and port of the test server.
	baseURL = "http://localhost:8888"
	// publicURL is an unauthenticated URL that we check to see if the server is available.
	publicURL = baseURL + "/static/.keep"

	// httpClientTimeout is the timeout for the http client to wait for a response.
	httpClientTimeout = 500 * time.Millisecond

	// tryServingDuration is how long we check try to see if the server is available.
	tryServingDuration = 5 * time.Second
)

func Test_Run(t *testing.T) {
	t.Run("ok, says it started then stopped http server", testEnv(func(t *testing.T) {
		out := newBuffer()

		ctx := cancelOnceServed(t, publicURL)

		got := run(ctx, out)
		want := 0
		if got != want {
			t.Fatalf("got exit code %d, want %d. logs:\n%s", got, want, out.String())
		}

		assertLog(t, out.String(),
			"starting http server",
			"stopping http server",
			"http server stopped successfully",
		)
	}))

	t.Run("ok, says it ran migrations", testEnv(func(t *testing.T) {
		out := newBuffer()

		ctx := cancelOnceServed(t, publicURL)

		got := run(ctx, out)
		want := 0
		if got != want {
			t.Fatalf("got exit code %d, want %d. logs:\n%s", got, want, out.String())
		}

		assertLog(t, out.String(),
			"attempting to migrate database",
			"migration ran",
		)
	}))

	t.Run("ok, did not say it ran migrations when DB_MIGRATE=false", testEnv(func(t *testing.T) {
		envForTest(t, "DB_MIGRATE", "false")

		out := newBuffer()

		ctx := cancelOnceServed(t, publicURL)

		got := run(ctx, out)
		want := 0
		if got != want {
			t.Fatalf("got exit code %d, want %d. logs:\n%s", got, want, out.String())
		}

		if strings.Contains(out.String(), "attempting to migrate database") {
			t.Errorf("expected not to run migrations, but did")
		}
	}))

	t.Run("ok, says it loaded templates from directory when HTTP_VIEW_DIR is provided", testEnv(func(t *testing.T) {
		// load the templates from disk instead of using the embedded ones.
		envForTest(t, "HTTP_VIEW_DIR", "../../assets/templates")

		out := newBuffer()

		ctx := cancelOnceServed(t, publicURL)

		got := run(ctx, out)
		want := 0
		if got != want {
			t.Fatalf("got exit code %d, want %d. logs:\n%s", got, want, out.String())
		}

		assertLog(t, out.String(), "loading templates from disk")
	}))

	t.Run("fail, invalid environment", testEnv(func(t *testing.T) {
		envForTest(t, "HTTP_READ_TIMEOUT", "-1ms")

		out := newBuffer()

		// if the run function somehow ends up starting the http server, stop after a timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		got := run(ctx, out)
		want := 1
		if got != want {
			t.Fatalf("got exit code %d, want %d. logs:\n%s", got, want, out.String())
		}

		assertLog(t, out.String(), "failed to get config from environment")
	}))
}

// safeBuffer is a buffer that is safe for concurrent use.
type safeBuffer struct {
	mutex  *sync.Mutex
	buffer *bytes.Buffer
}

func newBuffer() *safeBuffer {
	return &safeBuffer{
		mutex:  &sync.Mutex{},
		buffer: &bytes.Buffer{},
	}
}

func (sb *safeBuffer) WriteString(s string) (n int, err error) {
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	return sb.buffer.WriteString(s)
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) {
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	return sb.buffer.Write(p)
}

func (sb *safeBuffer) String() string {
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	return sb.buffer.String()
}

// assertLog checks that the log contains the wanted strings in
// order. Non-matching parts in log are ignored.
func assertLog(t *testing.T, log string, want ...string) {
	t.Helper()

	for i, line := range want {
		x := strings.Index(log, line)
		if x == -1 {
			t.Errorf("log does not contain line in pos %d: %q", i, line)
			return
		}

		log = log[x+len(line):]
	}
}

// cancelOnceServed returns a context that is canceled after given url returned status OK.
func cancelOnceServed(t *testing.T, url string) context.Context {
	t.Helper()

	//
	ctx, cancel := context.WithTimeout(context.Background(), tryServingDuration)

	result := make(chan error, 1)

	t.Cleanup(func() {
		err := <-result
		if err != nil {
			t.Fatalf("error waiting for status ok: %v", err)
		}
	})

	go func() {
		err := waitForStatusOK(ctx, url)
		result <- err
		cancel()
	}()

	return ctx
}

func waitForStatusOK(ctx context.Context, url string) error {
	httpClient := &http.Client{
		Timeout: httpClientTimeout,
	}

	ticks := time.NewTicker(httpClientTimeout * 2)
	for {
		select {
		case <-ticks.C:
			res, err := httpClient.Get(url)
			if err != nil {
				continue
			}
			defer res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// testEnv returns a test function that is ensures the environment is ready to run the app and cleans up afterwards.
func testEnv(testFunc func(t *testing.T)) func(t *testing.T) {
	env := make(map[string]string, 0)

	for key, val := range requiredEnv() {
		env[key] = val
	}

	// add/overwrite env variables.
	env["DB_FILENAME"] = "househunt-unit-test.db"

	// Disable secure cookies.
	// the Go cookiejar does not sent secure cookies over localhost
	// so we disable secure cookies for testing. See this issue for more info:
	// https://github.com/golang/go/issues/60997
	env["HTTP_SECURE_COOKIE"] = "false"

	return func(t *testing.T) {
		t.Helper()

		for key, val := range env {
			envForTest(t, key, val)
		}

		t.Cleanup(func() {
			// remove database files.
			dbFile := env["DB_FILENAME"]
			files := []string{
				dbFile,
				fmt.Sprintf("%s-shm", dbFile),
				fmt.Sprintf("%s-wal", dbFile),
			}

			for _, file := range files {
				err := os.Remove(file)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed to remove file %s: %v", file, err)
				}
			}
		})

		testFunc(t)
	}
}
