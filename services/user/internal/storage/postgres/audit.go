package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"kitdesign/services/user/internal/audit"
)

func (s *Store) Record(ctx context.Context, event audit.Event) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.Must(uuid.NewV7())
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_events (id, actor_id, subject_id, action, ip, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
	`,
		event.ID,
		nullableUUID(event.ActorID),
		nullableUUID(event.SubjectID),
		event.Action,
		event.IP,
		event.UserAgent,
		string(payload),
		event.CreatedAt,
	)
	return err
}

func (s *Store) ListAuditEvents(ctx context.Context, userID uuid.UUID, limit int) ([]audit.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, subject_id, action, ip, user_agent, metadata, created_at
		FROM audit_events
		WHERE subject_id = $1 OR actor_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []audit.Event
	for rows.Next() {
		var event audit.Event
		var actor pgtype.UUID
		var subject pgtype.UUID
		var metadata []byte
		if err := rows.Scan(&event.ID, &actor, &subject, &event.Action, &event.IP, &event.UserAgent, &metadata, &event.CreatedAt); err != nil {
			return nil, err
		}
		if actor.Valid {
			id := uuid.UUID(actor.Bytes)
			event.ActorID = &id
		}
		if subject.Valid {
			id := uuid.UUID(subject.Bytes)
			event.SubjectID = &id
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &event.Metadata)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}
