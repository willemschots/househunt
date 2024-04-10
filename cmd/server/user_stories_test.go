package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Test_UserStories tests the user stories of the application.
// These are end-to-end tests and won't check the nitty-gritty details or edge cases.
func Test_UserStories(t *testing.T) {
	t.Run("as an unauthenticated agent, I want to", appTest(func(t *testing.T) {
		runAppForTest(t)

		t.Run("view the user registration form", func(t *testing.T) {
			c := newClient(t)

			body := c.MustGetBody("/register")

			// Symbolic check for the form. I'm not checking the HTML too much,
			// because I don't want every change to the front-end break these tests.
			symbol := `id="register-user"`
			if !strings.Contains(body, symbol) {
				t.Errorf("did not find\n%s\nin body\n%s", symbol, body)
			}
		})

		// TODO:
		//t.Run("submit the registration form", func(t *testing.T) {
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
func runAppForTest(t *testing.T) {
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
		// stop both tasks if they're still in progress.
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
}

type client struct {
	t    *testing.T
	http *http.Client
}

func newClient(t *testing.T) *client {
	return &client{
		t: t,
		http: &http.Client{
			Timeout: httpClientTimeout,
		},
	}
}

func (c *client) MustGetBody(url string) string {
	resp, err := c.http.Get(testURLPrefix + url)
	if err != nil {
		c.t.Fatalf("unexpected error during get request: %v", err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			c.t.Fatalf("unexpected error closing response body: %v", err)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("unexpected error reading response body: %v", err)
	}

	return string(data)
}
