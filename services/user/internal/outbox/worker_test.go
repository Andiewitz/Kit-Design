package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/outbox"
)

type fakeStore struct {
	messages []outbox.Message
	claimErr error

	sent   []uuid.UUID
	failed map[uuid.UUID]string
	retry  map[uuid.UUID]time.Duration
}

func (f *fakeStore) ClaimOutbox(context.Context, int) ([]outbox.Message, error) {
	return f.messages, f.claimErr
}

func (f *fakeStore) MarkOutboxSent(_ context.Context, id uuid.UUID) error {
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeStore) MarkOutboxFailed(_ context.Context, id uuid.UUID, reason string, retryAfter time.Duration) error {
	if f.failed == nil {
		f.failed = make(map[uuid.UUID]string)
		f.retry = make(map[uuid.UUID]time.Duration)
	}
	f.failed[id] = reason
	f.retry[id] = retryAfter
	return nil
}

type fakeMailer struct {
	err      error
	messages []mailer.Message
}

func (f *fakeMailer) Send(_ context.Context, message mailer.Message) error {
	f.messages = append(f.messages, message)
	return f.err
}

func newWorker(store *fakeStore, sender *fakeMailer) *outbox.Worker {
	return &outbox.Worker{
		Store:  store,
		Mailer: sender,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func message(t *testing.T, topic string, payload mailer.Message) outbox.Message {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return outbox.Message{ID: uuid.Must(uuid.NewV7()), Topic: topic, Payload: encoded, Attempts: 1}
}

func TestProcessOnceSendsEmail(t *testing.T) {
	t.Parallel()

	payload := message(t, outbox.TopicEmailSend, mailer.Message{To: "user@example.com", Subject: "Hi"})
	store := &fakeStore{messages: []outbox.Message{payload}}
	sender := &fakeMailer{}
	worker := newWorker(store, sender)

	if err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || sender.messages[0].To != "user@example.com" {
		t.Fatalf("messages = %+v", sender.messages)
	}
	if len(store.sent) != 1 || store.sent[0] != payload.ID {
		t.Fatalf("sent = %+v", store.sent)
	}
}

func TestProcessOnceRetriesFailedEmail(t *testing.T) {
	t.Parallel()

	payload := message(t, outbox.TopicEmailSend, mailer.Message{To: "user@example.com"})
	store := &fakeStore{messages: []outbox.Message{payload}}
	sender := &fakeMailer{err: errors.New("smtp down")}
	worker := newWorker(store, sender)

	if err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.sent) != 0 {
		t.Fatal("message should not be marked sent")
	}
	if store.failed[payload.ID] == "" {
		t.Fatal("message should be marked failed")
	}
	if store.retry[payload.ID] <= 0 {
		t.Fatal("retry delay should be positive")
	}
}

func TestProcessOnceRejectsUnsupportedTopicAndPayload(t *testing.T) {
	t.Parallel()

	unsupported := outbox.Message{ID: uuid.Must(uuid.NewV7()), Topic: "other.topic", Payload: []byte(`{}`)}
	invalid := outbox.Message{ID: uuid.Must(uuid.NewV7()), Topic: outbox.TopicEmailSend, Payload: []byte(`not json`)}
	store := &fakeStore{messages: []outbox.Message{unsupported, invalid}}
	worker := newWorker(store, &fakeMailer{})

	if err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.failed) != 2 {
		t.Fatalf("failed = %+v", store.failed)
	}
}
