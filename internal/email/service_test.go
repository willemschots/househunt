package email_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/willemschots/househunt/internal/email"
	"github.com/willemschots/househunt/internal/email/view"
)

func Test_SendEmail(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		renderer := view.NewFSRenderer(os.DirFS("testdata"))

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		sender := email.NewLogSender(logger)

		cfg := email.ServiceConfig{
			From:    must(email.ParseAddress("alice@example.com")),
			BaseURL: must(url.Parse("http://example.com")),
		}

		svc := email.NewService(renderer, sender, cfg)

		data := struct {
			Name    string
			Message string
		}{
			Name:    "Jacob",
			Message: "Today is a beautiful day",
		}
		err := svc.Send(context.Background(), "test", email.Address("jacob@example.com"), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		for _, want := range []string{
			`msg="send email"`,
			`from=alice@example.com`,
			`recipient=jacob@example.com`,
			`subject="Hello Jacob!"`,
			`body="Your message is Today is a beautiful day"`,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("want %q to contain %q", got, want)
			}
		}
	})
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
