package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/user/log-ingestor/internal/domain"
)

const (
	defaultBatchSize    = 1000
	defaultRetryCount   = 3
	defaultRetryBackoff = 1 * time.Second
)

// ProcessLogsUseCase orchestrates reading log events from a buffer,
// processing them, and writing them to a final sink.
type ProcessLogsUseCase struct {
	bufferRepo domain.LogRepository
	sinkRepo   domain.LogRepository
	logger     *slog.Logger
	group      string
	consumer   string
}

// NewProcessLogsUseCase creates a new use case for processing logs.
func NewProcessLogsUseCase(bufferRepo, sinkRepo domain.LogRepository, logger *slog.Logger, group, consumer string) *ProcessLogsUseCase {
	return &ProcessLogsUseCase{
		bufferRepo: bufferRepo,
		sinkRepo:   sinkRepo,
		logger:     logger,
		group:      group,
		consumer:   consumer,
	}
}

// ProcessBatch reads a batch of logs, attempts to write them to the sink,
// and acknowledges them in the buffer on success.
func (uc *ProcessLogsUseCase) ProcessBatch(ctx context.Context) (int, error) {
	// 1. Read a batch of events from the buffer (Redis)
	events, err := uc.bufferRepo.ReadLogBatch(ctx, uc.group, uc.consumer, defaultBatchSize)
	if err != nil {
		uc.logger.Error("failed to read log batch from buffer", "error", err)
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil // No new events, not an error
	}

	uc.logger.Debug("read batch of events from buffer", "count", len(events))

	// 2. Attempt to write the batch to the sink (PostgreSQL) with retries
	err = uc.writeWithRetry(ctx, events)
	if err != nil {
		uc.logger.Error("failed to write log batch to sink after retries", "error", err)
		// In a real scenario, we would move these messages to a DLQ here.
		// For now, we just return the error, and the messages will be re-processed later.
		return 0, err
	}

	// 3. Acknowledge the messages in the buffer (Redis)
	messageIDs := make([]string, len(events))
	for i, event := range events {
		messageIDs[i] = event.StreamMessageID
	}

	if err := uc.bufferRepo.AcknowledgeLogs(ctx, uc.group, messageIDs...); err != nil {
		uc.logger.Error("failed to acknowledge logs in buffer", "error", err)
		// This is a critical error. The events are in PG but not ACK'd in Redis.
		// They will be reprocessed, but the PG upsert should handle the duplication.
		return 0, err
	}

	uc.logger.Info("successfully processed and sinked log batch", "count", len(events))
	return len(events), nil
}

func (uc *ProcessLogsUseCase) writeWithRetry(ctx context.Context, events []domain.LogEvent) error {
	var lastErr error
	for i := 0; i < defaultRetryCount; i++ {
		err := uc.sinkRepo.WriteLogBatch(ctx, events)
		if err == nil {
			return nil // Success
		}
		lastErr = err
		uc.logger.Warn("failed to write batch to sink, retrying...", "attempt", i+1, "error", err)
		select {
		case <-time.After(defaultRetryBackoff):
			// continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
```
