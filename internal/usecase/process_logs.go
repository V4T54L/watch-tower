package usecase

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/user/log-ingestor/internal/domain"
)

const (
	defaultBatchSize = 1000
)

// ProcessLogsUseCase orchestrates reading logs from a buffer and writing to a sink.
type ProcessLogsUseCase struct {
	bufferRepo   domain.LogRepository
	sinkRepo     domain.LogRepository
	logger       *slog.Logger
	group        string
	consumer     string
	retryCount   int
	retryBackoff time.Duration
}

// NewProcessLogsUseCase creates a new ProcessLogsUseCase.
func NewProcessLogsUseCase(bufferRepo, sinkRepo domain.LogRepository, logger *slog.Logger, group, consumer string, retryCount int, retryBackoff time.Duration) *ProcessLogsUseCase {
	return &ProcessLogsUseCase{
		bufferRepo:   bufferRepo,
		sinkRepo:     sinkRepo,
		logger:       logger.With("component", "process_logs_usecase"),
		group:        group,
		consumer:     consumer,
		retryCount:   retryCount,
		retryBackoff: retryBackoff,
	}
}

// ProcessBatch reads a batch of logs, attempts to write them to the sink with retries,
// moves to DLQ on failure, and acknowledges on success.
func (u *ProcessLogsUseCase) ProcessBatch(ctx context.Context) (int, error) {
	events, err := u.bufferRepo.ReadLogBatch(ctx, u.group, u.consumer, defaultBatchSize)
	if err != nil {
		u.logger.Error("Failed to read log batch from buffer", "error", err)
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	u.logger.Debug("Read batch from buffer", "count", len(events))

	err = u.writeWithRetry(ctx, events)
	if err != nil {
		u.logger.Error("Failed to write batch to sink after all retries, moving to DLQ", "error", err, "batch_size", len(events))
		if dlqErr := u.bufferRepo.MoveToDLQ(ctx, events); dlqErr != nil {
			u.logger.Error("CRITICAL: Failed to move events to DLQ. Events will be re-processed.", "error", dlqErr)
			return 0, dlqErr
		}
	}

	messageIDs := make([]string, len(events))
	for i, event := range events {
		messageIDs[i] = event.StreamMessageID
	}

	if ackErr := u.bufferRepo.AcknowledgeLogs(ctx, u.group, messageIDs...); ackErr != nil {
		u.logger.Error("Failed to acknowledge processed logs", "error", ackErr)
		return 0, ackErr
	}

	u.logger.Info("Successfully processed batch", "count", len(events), "final_status", map[bool]string{true: "SINKED", false: "DLQED"}[err == nil])
	return len(events), nil
}

func (u *ProcessLogsUseCase) writeWithRetry(ctx context.Context, events []domain.LogEvent) error {
	var lastErr error

	for i := 0; i < u.retryCount; i++ {
		err := u.sinkRepo.WriteLogBatch(ctx, events)
		if err == nil {
			return nil // Success
		}
		lastErr = err

		if i == u.retryCount-1 || ctx.Err() != nil {
			break
		}

		// Exponential backoff: 1s, 2s, 4s, ... for default backoff of 1s
		delay := time.Duration(float64(u.retryBackoff) * math.Pow(2, float64(i)))
		u.logger.Warn("Failed to write to sink, retrying...", "attempt", i+1, "delay", delay, "error", err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}

