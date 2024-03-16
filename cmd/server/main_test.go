package main

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// testURLPrefix is used to identify the scheme, host and port of the test server.
	testURLPrefix = "http://localhost:8888"
	// publicURL is an unauthenticated URL that we check to see if the server is available.
	publicURL = testURLPrefix

	// httpClientTimeout is the timeout for the http client to wait for a response.
	httpClientTimeout = 50 * time.Millisecond
)

func Test_Run(t *testing.T) {
	t.Run("ok, says it started then stopped http server", func(t *testing.T) {
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
	})

	t.Run("fail, invalid environment", func(t *testing.T) {
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
	})
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

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
