package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/V4T54L/watch-tower/internal/adapter/pii"
	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/V4T54L/watch-tower/internal/domain/mocks"
)

func TestIngestLogUseCase_Ingest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	redactor := pii.NewRedactor([]string{"email"}, logger)

	t.Run("Successful Ingestion", func(t *testing.T) {
		mockRepo := &mocks.MockLogRepository{}
		uc := NewIngestLogUseCase(mockRepo, redactor, logger)

		event := &domain.LogEvent{Message: "test message"}
		err := uc.Ingest(context.Background(), event)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if event.ID == "" {
			t.Error("expected event ID to be generated")
		}
		if event.ReceivedAt.IsZero() {
			t.Error("expected ReceivedAt to be set")
		}
		if len(mockRepo.BufferedEvents) != 1 {
			t.Errorf("expected 1 event to be buffered, got %d", len(mockRepo.BufferedEvents))
		}
		if mockRepo.BufferedEvents[0].ID != event.ID {
			t.Error("buffered event ID mismatch")
		}
	})

	t.Run("Repository Error", func(t *testing.T) {
		mockRepo := &mocks.MockLogRepository{
			BufferErr: errors.New("buffer is full"),
		}
		uc := NewIngestLogUseCase(mockRepo, redactor, logger)

		event := &domain.LogEvent{Message: "test message"}
		err := uc.Ingest(context.Background(), event)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if err.Error() != "buffer is full" {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})

	t.Run("PII Redaction", func(t *testing.T) {
		mockRepo := &mocks.MockLogRepository{}
		uc := NewIngestLogUseCase(mockRepo, redactor, logger)

		event := &domain.LogEvent{
			Message:  "user login",
			Metadata: []byte(`{"email": "test@example.com"}`),
		}
		err := uc.Ingest(context.Background(), event)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !mockRepo.BufferedEvents[0].PIIRedacted {
			t.Error("expected PIIRedacted flag to be true")
		}
		expectedMetadata := `{"email":"[REDACTED]"}`
		if string(mockRepo.BufferedEvents[0].Metadata) != expectedMetadata {
			t.Errorf("expected metadata to be redacted: got %s, want %s", string(mockRepo.BufferedEvents[0].Metadata), expectedMetadata)
		}
	})
}
