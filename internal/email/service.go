package email

import (
	"bytes"
	"context"
	"io"
	"net/url"
)

// TemplateElement is used by a renderer to identify the different parts of an email template.
type TemplateElement string

const (
	ElementSubject TemplateElement = "subject"
	ElementBody    TemplateElement = "body"
)

// Renderer is responsible for rendering email templates.
type Renderer interface {
	Render(w io.Writer, name string, element TemplateElement, data any) error
}

// Sender is responsible for actually sending an email.
type Sender interface {
	Send(ctx context.Context, from, recipient Address, subject, body string) error
}

// ServiceConfig is the configuration for the email service.
type ServiceConfig struct {
	From    Address
	BaseURL *url.URL
}

// Service provides the main functionality for sending emails.
type Service struct {
	cfg      ServiceConfig
	renderer Renderer
	sender   Sender
}

func NewService(renderer Renderer, sender Sender, cfg ServiceConfig) *Service {
	return &Service{
		cfg:      cfg,
		renderer: renderer,
		sender:   sender,
	}
}

func (s *Service) Send(ctx context.Context, name string, recipient Address, data any) error {
	var (
		sBuf bytes.Buffer
		bBuf bytes.Buffer
	)

	viewData := struct {
		Global any
		View   any
	}{
		Global: s.cfg,
		View:   data,
	}

	err := s.renderer.Render(&sBuf, name, ElementSubject, viewData)
	if err != nil {
		return err
	}

	err = s.renderer.Render(&bBuf, name, ElementBody, viewData)
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, s.cfg.From, recipient, sBuf.String(), bBuf.String())
}
