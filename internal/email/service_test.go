package email_test

import (
	"context"
	"testing"

	"github.com/willemschots/househunt/internal/email"
)

func Test_NewService(t *testing.T) {

}

func Test_SendEmail(t *testing.T) {
}

type fakeSender struct {
	gotSender    email.Address
	gotRecipient email.Address
	gotSubject   string
	gotBody      string
	willError    error
}

func (f *fakeSender) Send(ctx context.Context, sender, recipient email.Address, subject, body string) error {
	return nil
}
