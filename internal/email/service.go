package email

import (
	"bytes"
	"context"
	"io"
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

// Service provides the main functionality for sending emails.
type Service struct {
	from     Address
	renderer Renderer
	sender   Sender
}

func NewService(from Address, renderer Renderer, sender Sender) *Service {
	return &Service{
		from:     from,
		renderer: renderer,
		sender:   sender,
	}
}

func (s *Service) Send(ctx context.Context, name string, recipient Address, data any) error {
	var (
		sBuf bytes.Buffer
		bBuf bytes.Buffer
	)

	err := s.renderer.Render(&sBuf, name, ElementSubject, data)
	if err != nil {
		return err
	}

	err = s.renderer.Render(&bBuf, name, ElementBody, data)
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, s.from, recipient, sBuf.String(), bBuf.String())
}
