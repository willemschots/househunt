package email

import (
	"context"
)

// TemplateElement is used by a renderer to identify the different parts of an email template.
type TemplateElement string

const (
	ElementSubject TemplateElement = "subject"
	ElementBody    TemplateElement = "body"
)

// Renderer is responsible for rendering email templates.
type Renderer interface {
	Render(ctx context.Context, name string, element TemplateElement, data any) (string, error)
}

// Sender is responsible for actually sending an email.
type Sender interface {
	Send(ctx context.Context, sender, recipient Address, subject, body string) error
}

// Service provides the main functionality for sending emails.
type Service struct {
	renderer Renderer
	sender   Sender
}

func NewService(renderer Renderer, sender Sender) *Service {
	return &Service{
		renderer: renderer,
		sender:   sender,
	}
}

func (s *Service) SendMessage(ctx context.Context, name string, recipient Address, data any) error {
	// TODO: Implement.
	return nil
}
