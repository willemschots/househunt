package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Test_UserStories tests the user stories of the application.
// These are end-to-end tests and won't check the nitty-gritty details or edge cases.
func Test_UserStories(t *testing.T) {
	t.Run("as an unauthenticated agent, I want to", testEnv(func(t *testing.T) {
		// runAppForTest waits for the app to be up and stops it after the test finishes.
		logs := runAppForTest(t)

		c := newClient()

		t.Run("view the user registration form", func(t *testing.T) {
			body := c.mustGetBody(t, "/register", http.StatusOK)

			// Symbolic check for the form. I'm not checking the HTML too much,
			// because I don't want every change to the front-end break these tests.
			symbol := `id="register-user"`
			if !strings.Contains(body, symbol) {
				t.Errorf("did not find\n%s\nin body\n%s", symbol, body)
			}
		})

		var activationURL string

		t.Run("submit the registration form", func(t *testing.T) {
			form := url.Values{
				"Email":    {"agent@example.com"},
				"Password": {"reallyStrongPassword1"},
			}

			c.mustPostForm(t, "/register", form, http.StatusOK)

			// wait for the activation email to be logged.
			activationURL = waitAndCaptureActivationURL(t, logs, "agent@example.com")
			t.Logf("found activation url: %s", activationURL)
		})

		//t.Run("view and submit the activation url", func(t *testing.T) {
		//})
		//
		//t.Run("confirm my email address", func(t *testing.T) {
		//})
		//
		//t.Run("login to my account", func(t *testing.T) {
		//})
	}))

}

// runAppForTest runs the app while the test is running.
// This function returns after the app is confirmed to be up and stops
// the app when the test is cleaned up.
func runAppForTest(t *testing.T) *safeBuffer {
	t.Helper()

	// This helper function does two things:
	// 1. Run the app in a goroutine.
	// 2. Wait for the app to be up and running.

	// Both these tasks are done concurrently and share the same context.
	// When this context is cancelled, both tasks will stop.

	buf := newBuffer()

	// we will stop the server after a timeout or when the test is cleaned up.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(func() {
		// stop both tasks if it's still in progress.
		cancel()

		if t.Failed() {
			t.Logf("app output:\n%s", buf.String())
		}
	})

	// Task 1: Run the app.
	go func() {
		code := run(ctx, buf)
		if code != 0 {
			t.Errorf("run exited with code %d", code)
		}

		// stop the other task
		cancel()
	}()

	// Task 2: Wait for the app to be available.
	err := waitForStatusOK(ctx, publicURL)
	if err != nil {
		t.Fatalf("error waiting for status ok: %v", err)
	}

	return buf
}

type client struct {
	http *http.Client
}

func newClient() *client {
	return &client{
		http: &http.Client{
			Timeout: httpClientTimeout,
		},
	}
}

func (c *client) mustGetBody(t *testing.T, url string, wantStatus int) string {
	res, err := c.http.Get(testURLPrefix + url)
	if err != nil {
		t.Fatalf("unexpected error during get request: %v", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			t.Fatalf("unexpected error closing response body: %v", err)
		}
	}()

	if res.StatusCode != wantStatus {
		t.Fatalf("unexpected status code: %d", res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("unexpected error reading response body: %v", err)
	}

	return string(data)
}

func (c *client) mustPostForm(t *testing.T, url string, form url.Values, wantStatus int) {
	req, err := http.NewRequest(http.MethodPost, testURLPrefix+url, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("unexpected error creating post request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = form

	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("unexpected error during post request: %v", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			t.Fatalf("unexpected error closing response body: %v", err)
		}
	}()

	if res.StatusCode != wantStatus {
		t.Fatalf("unexpected status code: %d", res.StatusCode)
	}
}

func waitAndCaptureActivationURL(t *testing.T, logs *safeBuffer, addr string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	captureFunc := func() (string, bool) {

		lookFor := []string{
			`msg="send email"`,
			`subject="Please confirm your email address"`,
			fmt.Sprintf(`recipient=%s`, addr),
		}

	OUTER:
		for _, line := range strings.Split(logs.String(), "\n") {
			for _, l := range lookFor {
				if !strings.Contains(line, l) {
					continue OUTER
				}
			}

			url, ok := extractActivationURL(line)
			if ok {
				return url, true
			}
		}

		return "", false
	}

	for {
		select {
		case <-ticker.C:
			if url, ok := captureFunc(); ok {
				return url
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for email to %s", addr)
		}
	}
}

func extractActivationURL(s string) (string, bool) {
	s = strings.ReplaceAll(s, `\n`, " ")
	r := regexp.MustCompile(`\b(https?)://localhost:8888/user-activations\S+`)
	result := r.FindString(s)
	if result == "" {
		return "", false
	}
	return result, true
}
