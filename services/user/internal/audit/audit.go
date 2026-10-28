package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        uuid.UUID
	ActorID   *uuid.UUID
	SubjectID *uuid.UUID
	Action    string
	IP        string
	UserAgent string
	Metadata  map[string]any
	CreatedAt time.Time
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
}

type Noop struct{}

func (Noop) Record(context.Context, Event) error {
	return nil
}
