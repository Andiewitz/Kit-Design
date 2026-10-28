package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/mailer"
)

const TopicEmailSend = "email.send"

type Message struct {
	ID       uuid.UUID
	Topic    string
	Payload  []byte
	Attempts int
}

type Store interface {
	ClaimOutbox(ctx context.Context, limit int) ([]Message, error)
	MarkOutboxSent(ctx context.Context, id uuid.UUID) error
	MarkOutboxFailed(ctx context.Context, id uuid.UUID, reason string, retryAfter time.Duration) error
}

type Worker struct {
	Store    Store
	Mailer   mailer.Mailer
	Logger   *slog.Logger
	Interval time.Duration
}

func (w *Worker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.ProcessOnce(ctx); err != nil {
				w.Logger.Error("outbox processing failed", "error", err)
			}
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) error {
	messages, err := w.Store.ClaimOutbox(ctx, 20)
	if err != nil {
		return err
	}

	for _, message := range messages {
		if message.Topic != TopicEmailSend {
			_ = w.Store.MarkOutboxFailed(ctx, message.ID, "unsupported topic", time.Hour)
			continue
		}

		var payload mailer.Message
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			_ = w.Store.MarkOutboxFailed(ctx, message.ID, "invalid payload", time.Hour)
			continue
		}

		if err := w.Mailer.Send(ctx, payload); err != nil {
			retryAfter := time.Duration(message.Attempts*message.Attempts) * 30 * time.Second
			if retryAfter < 30*time.Second {
				retryAfter = 30 * time.Second
			}
			if retryAfter > time.Hour {
				retryAfter = time.Hour
			}
			_ = w.Store.MarkOutboxFailed(ctx, message.ID, err.Error(), retryAfter)
			w.Logger.Warn("email delivery failed", "outbox_id", message.ID, "error", err)
			continue
		}

		if err := w.Store.MarkOutboxSent(ctx, message.ID); err != nil {
			return fmt.Errorf("mark outbox sent: %w", err)
		}
	}

	return nil
}
