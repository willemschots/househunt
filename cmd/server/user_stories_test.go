package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

// Test_UserStories tests the user stories of the application.
// These are end-to-end tests and won't check the nitty-gritty details or edge cases.
func Test_UserStories(t *testing.T) {
	t.Run("as an agent, I want to", testEnv(func(t *testing.T) {
		// runAppForTest waits for the app to be up and stops it after the test finishes.
		logs := runAppForTest(t)

		c := newClient(t)

		t.Run("prevent mistakes when registering a new account", func(t *testing.T) {
			// first view the form.
			body := c.mustGetBody(t, "/register", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "register-user")

			// then submit it with invalid values.
			form.values.Set("email", "") // empty email
			form.values.Set("password", "reallyStrongPassword1")

			c.mustSubmitForm(t, form, assertStatusCode(t, http.StatusBadRequest))
		})

		t.Run("register a new account", func(t *testing.T) {
			// first view the form.
			body := c.mustGetBody(t, "/register", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "register-user")

			if !form.values.Has("email") || !form.values.Has("password") {
				t.Fatalf("expected form to have email and password fields, got %v", form.values)
			}

			// then submit it.
			form.values.Set("email", "agent@example.com")
			form.values.Set("password", "reallyStrongPassword1")

			// TODO: This should redirect to a success page.
			c.mustSubmitForm(t, form, assertRedirectsTo(t, "/register", http.StatusFound))
		})

		var activationURL *url.URL

		t.Run("wait for the activation email", func(t *testing.T) {
			// wait for the activation email to be logged.
			activationURL = waitAndCaptureURL(t, logs, "agent@example.com", "/user-activations")
		})

		t.Run("activate my new account", func(t *testing.T) {
			// first view the activation page.
			body := c.mustGetBody(t, activationURL.String(), assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "activate-user")
			if form.values.Get("id") != activationURL.Query().Get("id") ||
				form.values.Get("token") != activationURL.Query().Get("token") {
				t.Fatalf("expected form to have id and token fields, got %v", form.values)
			}

			// submit the activation form (will be done automatically by JS in real life).
			c.mustSubmitForm(t, form, assertRedirectsTo(t, "/login", http.StatusFound))
		})

		t.Run("verify I can't access the dashboard", func(t *testing.T) {
			c.mustGetBody(t, "/dashboard", assertStatusCode(t, http.StatusNotFound))
		})

		t.Run("prevent mistakes when logging into my account", func(t *testing.T) {
			// first view the login form.
			body := c.mustGetBody(t, "/login", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "login-user")

			// then submit it with invalid values.
			form.values.Set("email", "") // empty email.
			form.values.Set("password", "reallyStrongPassword1")

			c.mustSubmitForm(t, form, assertStatusCode(t, http.StatusBadRequest))
		})

		t.Run("login to my now active account", func(t *testing.T) {
			// first view the login form.
			body := c.mustGetBody(t, "/login", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "login-user")

			if !form.values.Has("email") || !form.values.Has("password") {
				t.Fatalf("expected form to have email and password fields, got %v", form.values)
			}

			// then submit it.
			form.values.Set("email", "agent@example.com")
			form.values.Set("password", "reallyStrongPassword1")

			c.mustSubmitForm(t, form, func(res *http.Response) {
				t.Logf("cookies: %v", res.Cookies())
				assertCookie(t, "csrf", func(c *http.Cookie) {
					if c.MaxAge >= 0 {
						t.Fatalf("expected csrf cookie to be unset")
					}
				})
				assertRedirectsTo(t, "/dashboard", http.StatusFound)(res)
			})
		})

		t.Run("verify I can now access the dashboard", func(t *testing.T) {
			c.mustGetBody(t, "/dashboard", assertStatusCode(t, http.StatusOK))
		})

		t.Run("get the dashboard and log out", func(t *testing.T) {
			body := c.mustGetBody(t, "/dashboard", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "logout-user")

			c.mustSubmitForm(t, form, func(res *http.Response) {
				assertRedirectsTo(t, "/", http.StatusFound)(res)
			})
		})

		t.Run("verify I can't access the dashboard", func(t *testing.T) {
			c.mustGetBody(t, "/dashboard", assertStatusCode(t, http.StatusNotFound))
		})

		t.Run("prevent mistakes when requesting a new password", func(t *testing.T) {
			// first view the form.
			body := c.mustGetBody(t, "/forgot-password", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "forgot-password")

			// then submit it with invalid values.
			form.values.Set("email", "") // empty email.

			c.mustSubmitForm(t, form, assertStatusCode(t, http.StatusBadRequest))
		})

		t.Run("request a new password because I forgot my old one", func(t *testing.T) {
			// first view the form.
			body := c.mustGetBody(t, "/forgot-password", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "forgot-password")

			if !form.values.Has("email") {
				t.Fatalf("expected form to have email field, got %v", form.values)
			}

			// then submit it.
			form.values.Set("email", "agent@example.com")

			// TODO: This should redirect to a success page.
			c.mustSubmitForm(t, form, assertRedirectsTo(t, "/forgot-password", http.StatusFound))
		})

		var passwordResetURL *url.URL

		t.Run("wait for the password reset email", func(t *testing.T) {
			// wait for the password reset email to be logged.
			passwordResetURL = waitAndCaptureURL(t, logs, "agent@example.com", "/password-resets")
		})

		t.Run("prevent mistakes when resetting my password", func(t *testing.T) {
			// first view the form
			body := c.mustGetBody(t, passwordResetURL.String(), assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "reset-password")

			// then submit it with invalid values.
			form.values.Set("password", "") // empty password.

			c.mustSubmitForm(t, form, assertStatusCode(t, http.StatusBadRequest))
		})

		t.Run("reset my password", func(t *testing.T) {
			// first view the form
			body := c.mustGetBody(t, passwordResetURL.String(), assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "reset-password")

			if !form.values.Has("rawtoken.id") || !form.values.Has("rawtoken.token") || !form.values.Has("password") {
				t.Fatalf("expected form to have id, token, and password fields, got %v", form.values)
			}

			// set new password
			form.values.Set("password", "newReallyStrongPassword1")

			// then submit it.
			c.mustSubmitForm(t, form, assertRedirectsTo(t, "/login", http.StatusFound))
		})

		t.Run("login with the new password", func(t *testing.T) {
			// first view the login form.
			body := c.mustGetBody(t, "/login", assertStatusCode(t, http.StatusOK))

			form := parseHTMLFormWithID(t, strings.NewReader(body), "login-user")

			// then submit it.
			form.values.Set("email", "agent@example.com")
			form.values.Set("password", "newReallyStrongPassword1")

			c.mustSubmitForm(t, form, assertRedirectsTo(t, "/dashboard", http.StatusFound))
		})

		t.Run("verify I can access the dashboard", func(t *testing.T) {
			c.mustGetBody(t, "/dashboard", assertStatusCode(t, http.StatusOK))
		})
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

func newClient(t *testing.T) *client {
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}
	return &client{
		http: &http.Client{
			Timeout: httpClientTimeout,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *client) mustGetBody(t *testing.T, url string, responseFunc func(*http.Response)) string {
	t.Helper()

	if strings.HasPrefix(url, "/") {
		url = baseURL + url
	}

	res, err := c.http.Get(url)
	if err != nil {
		t.Fatalf("unexpected error during get request: %v", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			t.Fatalf("unexpected error closing response body: %v", err)
		}
	}()

	if responseFunc != nil {
		responseFunc(res)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("unexpected error reading response body: %v", err)
	}

	return string(data)
}

func (c *client) mustSubmitForm(t *testing.T, form htmlForm, responseFunc func(*http.Response)) {
	t.Helper()

	url := form.action
	if strings.HasPrefix(url, "/") {
		url = baseURL + url
	}

	if form.method == "" {
		t.Fatalf("form method not set")
	}

	req, err := http.NewRequest(form.method, url, strings.NewReader(form.values.Encode()))
	if err != nil {
		t.Fatalf("unexpected error creating post request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	if responseFunc != nil {
		responseFunc(res)
	}
}

func assertStatusCode(t *testing.T, status int) func(*http.Response) {
	return func(res *http.Response) {
		t.Helper()

		if res.StatusCode != status {
			t.Fatalf("expected status %d, got %d", status, res.StatusCode)
		}
	}
}

func assertRedirectsTo(t *testing.T, location string, status int) func(*http.Response) {
	return func(res *http.Response) {
		if res.StatusCode != status {
			t.Fatalf("expected status %d, got %d", status, res.StatusCode)
		}

		if res.Header.Get("Location") != location {
			t.Fatalf("expected redirect to %q, got %q", location, res.Header.Get("Location"))
		}
	}
}

func assertCookie(t *testing.T, name string, assertFunc func(c *http.Cookie)) func(*http.Response) {
	return func(res *http.Response) {
		foundCookie := false
		for _, cookie := range res.Cookies() {
			if cookie.Name == name {
				foundCookie = true
				assertFunc(cookie)
				break
			}
		}

		if !foundCookie {
			t.Fatalf("expected auth cookie to be set")
		}
	}
}

type htmlForm struct {
	method string
	action string
	values url.Values
}

func parseHTMLFormWithID(t *testing.T, reader io.Reader, id string) htmlForm {
	t.Helper()

	doc, err := html.Parse(reader)
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	form := htmlForm{
		values: make(url.Values),
	}

	node := findFormNodeWithID(t, doc, id)
	if node == nil {
		t.Fatalf("form with id %q not found", id)
	}

	addFormValues(t, node, &form.values)

	for _, a := range node.Attr {
		switch a.Key {
		case "method":
			form.method = a.Val
		case "action":
			form.action = a.Val
		}
	}

	return form
}

func findFormNodeWithID(t *testing.T, n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode && n.Data == "form" {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if form := findFormNodeWithID(t, c, id); form != nil {
			return form
		}
	}

	return nil
}

func addFormValues(t *testing.T, n *html.Node, vals *url.Values) {
	// select all input elements.
	if n.Type == html.ElementNode && n.Data == "input" {
		var name, value string
		for _, a := range n.Attr {
			switch a.Key {
			case "name":
				name = a.Val
			case "value":
				value = a.Val
			}
		}

		if name != "" {
			vals.Set(name, value)
		}

		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		addFormValues(t, c, vals)
	}
}

func waitAndCaptureURL(t *testing.T, logs *safeBuffer, addr, urlPath string) *url.URL {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	captureFunc := func() (*url.URL, bool) {

		lookFor := []string{
			`msg="send email"`,
			fmt.Sprintf(`recipient=%s`, addr),
		}

	OUTER:
		for _, line := range strings.Split(logs.String(), "\n") {
			for _, l := range lookFor {
				if !strings.Contains(line, l) {
					continue OUTER
				}
			}

			activationURL, ok := extractURLWithPath(line, urlPath)
			if ok {
				return activationURL, true
			}
		}

		return nil, false
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

func extractURLWithPath(s, path string) (*url.URL, bool) {
	s = strings.ReplaceAll(s, `\n`, " ")
	pattern := fmt.Sprintf(`\b%s%s\S+`, baseURL, path)
	r := regexp.MustCompile(pattern)
	result := r.FindString(s)
	if result == "" {
		return nil, false
	}

	u, err := url.Parse(result)
	if err != nil {
		return nil, false
	}
	return u, true
}
