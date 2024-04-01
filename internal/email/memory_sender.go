package email

import "context"

type MemorySender struct {
	Emails []struct {
		From      Address
		Recipient Address
		Subject   string
		Body      string
	}
}

func NewMemorySender() *MemorySender {
	return &MemorySender{}
}

func (s *MemorySender) Send(_ context.Context, from, recipient Address, subject, body string) error {
	s.Emails = append(s.Emails, struct {
		From      Address
		Recipient Address
		Subject   string
		Body      string
	}{
		From:      from,
		Recipient: recipient,
		Subject:   subject,
		Body:      body,
	})
	return nil
}
