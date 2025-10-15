package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/user/log-ingestor/internal/domain"
	"github.com/user/log-ingestor/internal/domain/mocks"
)

func TestProcessLogsUseCase_ProcessBatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	testEvents := []domain.LogEvent{
		{ID: "1", StreamMessageID: "msg1", Message: "event 1"},
		{ID: "2", StreamMessageID: "msg2", Message: "event 2"},
	}

	t.Run("Successful Processing", func(t *testing.T) {
		bufferRepo := &mocks.MockLogRepository{ReadBatchResult: testEvents}
		sinkRepo := &mocks.MockLogRepository{}
		uc := NewProcessLogsUseCase(bufferRepo, sinkRepo, logger, "group", "consumer", 3, 1*time.Millisecond)

		count, err := uc.ProcessBatch(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if count != len(testEvents) {
			t.Errorf("expected processed count to be %d, got %d", len(testEvents), count)
		}
		if len(sinkRepo.WrittenEvents) != 2 {
			t.Errorf("expected 2 events written to sink, got %d", len(sinkRepo.WrittenEvents))
		}
		if len(bufferRepo.AckedMessageIDs) != 2 {
			t.Errorf("expected 2 messages to be acked, got %d", len(bufferRepo.AckedMessageIDs))
		}
		if len(bufferRepo.DLQEvents) != 0 {
			t.Errorf("expected 0 events in DLQ, got %d", len(bufferRepo.DLQEvents))
		}
	})

	t.Run("Sink Failure with Retry and DLQ", func(t *testing.T) {
		bufferRepo := &mocks.MockLogRepository{ReadBatchResult: testEvents}
		sinkRepo := &mocks.MockLogRepository{WriteErr: errors.New("database is down")}
		uc := NewProcessLogsUseCase(bufferRepo, sinkRepo, logger, "group", "consumer", 2, 1*time.Millisecond)

		count, err := uc.ProcessBatch(context.Background())

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if count != 0 {
			t.Errorf("expected processed count to be 0, got %d", count)
		}
		if len(sinkRepo.WrittenEvents) != 0 {
			t.Errorf("expected 0 events written to sink, got %d", len(sinkRepo.WrittenEvents))
		}
		if len(bufferRepo.DLQEvents) != 2 {
			t.Errorf("expected 2 events in DLQ, got %d", len(bufferRepo.DLQEvents))
		}
		// Messages should be acked even if they go to DLQ
		if len(bufferRepo.AckedMessageIDs) != 2 {
			t.Errorf("expected 2 messages to be acked, got %d", len(bufferRepo.AckedMessageIDs))
		}
	})

	t.Run("Buffer Read Error", func(t *testing.T) {
		bufferRepo := &mocks.MockLogRepository{ReadErr: errors.New("redis connection failed")}
		sinkRepo := &mocks.MockLogRepository{}
		uc := NewProcessLogsUseCase(bufferRepo, sinkRepo, logger, "group", "consumer", 3, 1*time.Millisecond)

		count, err := uc.ProcessBatch(context.Background())

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if count != 0 {
			t.Errorf("expected processed count to be 0, got %d", count)
		}
	})

	t.Run("No Events to Process", func(t *testing.T) {
		bufferRepo := &mocks.MockLogRepository{ReadBatchResult: []domain.LogEvent{}}
		sinkRepo := &mocks.MockLogRepository{}
		uc := NewProcessLogsUseCase(bufferRepo, sinkRepo, logger, "group", "consumer", 3, 1*time.Millisecond)

		count, err := uc.ProcessBatch(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if count != 0 {
			t.Errorf("expected processed count to be 0, got %d", count)
		}
		if len(sinkRepo.WrittenEvents) != 0 {
			t.Error("sink should not be called with no events")
		}
	})
}

