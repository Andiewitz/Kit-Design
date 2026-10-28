package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/outbox"
)

func (s *Store) ClaimOutbox(ctx context.Context, limit int) ([]outbox.Message, error) {
	rows, err := s.pool.Query(ctx, `
		WITH claimed AS (
			SELECT id
			FROM outbox_events
			WHERE processed_at IS NULL AND available_at <= now()
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events AS event
		SET attempts = event.attempts + 1,
		    available_at = now() + interval '30 seconds'
		FROM claimed
		WHERE event.id = claimed.id
		RETURNING event.id, event.topic, event.payload, event.attempts
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []outbox.Message
	for rows.Next() {
		var message outbox.Message
		if err := rows.Scan(&message.ID, &message.Topic, &message.Payload, &message.Attempts); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) MarkOutboxSent(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE outbox_events
		SET processed_at = now(), last_error = NULL
		WHERE id = $1
	`, id)
	return err
}

func (s *Store) MarkOutboxFailed(ctx context.Context, id uuid.UUID, reason string, retryAfter time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE outbox_events
		SET last_error = $2,
		    available_at = now() + make_interval(secs => $3)
		WHERE id = $1
	`, id, reason, retryAfter.Seconds())
	return err
}
