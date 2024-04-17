package postmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/krypto"
)

// Settings contains the settings for the Postmark API.
type Settings struct {
	APIURL        *url.URL
	ServerToken   krypto.Secret
	MessageStream string
}

// Sender is an email sender that sends emails using the Postmark API.
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

type emailJSON struct {
	From          string
	To            string
	Subject       string
	TextBody      string
	MessageStream string
}

type response struct {
	ErrorCode int
	Message   string
	MessageID string
}

// Send sends an email using the Postmark API.
func (s *Sender) Send(ctx context.Context, from, recipient email.Address, subject, body string) error {
	data := emailJSON{
		From:          string(from),
		To:            string(recipient),
		Subject:       subject,
		TextBody:      body,
		MessageStream: s.settings.MessageStream,
	}

	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(data)
	if err != nil {
		return fmt.Errorf("failed to encode email json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.settings.APIURL.String(), &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", string(s.settings.ServerToken.SecretValue()))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	//if resp.StatusCode != http.StatusOK {
	//	return fmt.Errorf("request did not succeed, status code %d", resp.StatusCode)
	//}

	var res response
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if res.ErrorCode != 0 {
		return fmt.Errorf("error code in response: %d %v", res.ErrorCode, res.Message)
	}

	return nil
}
