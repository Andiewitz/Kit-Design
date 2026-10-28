package mailer

import (
	"context"
	"strings"
	"testing"
)

func TestMessageBodySanitizesSubject(t *testing.T) {
	t.Parallel()

	body := messageBody("no-reply@example.com", Message{
		To:      "user@example.com",
		Subject: "Hello\r\nBcc: attacker@example.com",
		Text:    "Welcome",
	})
	if strings.Contains(body, "\r\nBcc:") {
		t.Fatal("subject header injection was not removed")
	}
	if !strings.Contains(body, "Subject: HelloBcc: attacker@example.com") {
		t.Fatalf("unexpected body: %q", body)
	}
	if !strings.HasSuffix(body, "\r\n\r\nWelcome\r\n") {
		t.Fatalf("message body missing: %q", body)
	}
}

func TestSMTPRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	sender := &SMTP{Addr: "not-an-address", From: "no-reply@example.com"}
	if err := sender.Send(context.Background(), Message{To: "user@example.com"}); err == nil {
		t.Fatal("expected invalid address error")
	}
}
