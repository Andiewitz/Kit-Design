package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Message struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

type Mailer interface {
	Send(ctx context.Context, message Message) error
}

type Log struct {
	Logger *slog.Logger
}

func (l *Log) Send(_ context.Context, message Message) error {
	l.Logger.Info("email delivery skipped in log mode",
		"to", message.To,
		"subject", message.Subject,
	)
	return nil
}

type SMTP struct {
	Addr string
	From string
}

func (s *SMTP) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return fmt.Errorf("invalid smtp address: %w", err)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer connection.Close()

	deadline := time.Now().Add(30 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = connection.SetDeadline(deadline)

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Mail(s.From); err != nil {
		return fmt.Errorf("smtp sender: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write([]byte(messageBody(s.From, message))); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finish: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func messageBody(from string, message Message) string {
	headers := []string{
		"From: " + from,
		"To: " + message.To,
		"Subject: " + sanitizeHeader(message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + message.Text + "\r\n"
}

func sanitizeHeader(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}
