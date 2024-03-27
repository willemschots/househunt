package mailgun

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/willemschots/househunt/internal/email"
)

// Settings contains the settings for the Mailgun API.
type Settings struct {
	APIHost  string
	Domain   string
	Username string
	Password string
}

// Sender is an email sender that sends emails using the Mailgun API.
type Sender struct {
	client   *http.Client
	settings Settings
}

// NewSender creates a new sender.
func NewSender(client *http.Client, s Settings) *Sender {
	return &Sender{
		client:   client,
		settings: s,
	}
}

// Send sends an email using the Mailgun API.
func (s *Sender) Send(ctx context.Context, from, recipient email.Address, subject, body string) error {
	// Below we send a POST request to the Mailgun API to send an email. We don't use the Go mailgun package,
	// because it brings in a lot of dependencies that we don't need. If we need more advanced features, we can
	// reconsider using it.

	// We first map the input fields to a multipart form.
	data := map[string]io.Reader{
		"from":    strings.NewReader(string(from)),
		"to":      strings.NewReader(string(recipient)),
		"subject": strings.NewReader(subject),
		"text":    strings.NewReader(body),
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for field, v := range data {
		ff, err := w.CreateFormField(field)
		if err != nil {
			return err
		}
		_, err = io.Copy(ff, v)
		if err != nil {
			return err
		}
	}

	err := w.Close()
	if err != nil {
		return err
	}

	// Then we construct the request.
	reqURL := fmt.Sprintf("https://%s/v3/%s/messages", s.settings.APIHost, s.settings.Domain)
	reqBody := bytes.NewReader(buf.Bytes())
	req, err := http.NewRequest(http.MethodPost, reqURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.SetBasicAuth(s.settings.Username, s.settings.Password)

	// And finally we send the request.
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request did not succeeed %d: %v", resp.StatusCode, string(resBody))
	}

	return nil
}
