package email

import (
	"context"
	"log/slog"
)

// LogSender is a Sender that logs the email to the logger instead of sending it.
// Note that this is not meant for production use as it logs the email addresses
// and all email contents. Resulting in sensitive information being logged.
type LogSender struct {
	logger *slog.Logger
}

// NewLogSender creates a new LogSender.
func NewLogSender(logger *slog.Logger) *LogSender {
	return &LogSender{
		logger: logger,
	}
}

// Send logs the email to the logger.
func (s *LogSender) Send(_ context.Context, from, recipient Address, subject, body string) error {
	s.logger.Info("send email",
		"from", from,
		"recipient", recipient,
		"subject", subject,
		"body", body,
	)
	return nil
}
