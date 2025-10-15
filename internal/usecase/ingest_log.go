package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/user/log-ingestor/internal/adapter/pii"
	"github.com/user/log-ingestor/internal/domain"
)

// IngestLogUseCase handles the business logic for ingesting a log event.
type IngestLogUseCase struct {
	repo     domain.LogRepository
	redactor *pii.Redactor
	logger   *slog.Logger
}

// NewIngestLogUseCase creates a new IngestLogUseCase.
func NewIngestLogUseCase(repo domain.LogRepository, redactor *pii.Redactor, logger *slog.Logger) *IngestLogUseCase {
	return &IngestLogUseCase{
		repo:     repo,
		redactor: redactor,
		logger:   logger,
	}
}

// Ingest validates, enriches, redacts, and buffers a log event.
func (uc *IngestLogUseCase) Ingest(ctx context.Context, event *domain.LogEvent) error {
	// 1. Enrich with server-side data
	event.ReceivedAt = time.Now().UTC()
	if event.ID == "" {
		event.ID = uuid.NewString()
	}

	// 2. Redact PII
	if err := uc.redactor.Redact(event); err != nil {
		uc.logger.Warn("failed to redact PII, proceeding with original event", "error", err, "event_id", event.ID)
		// Non-fatal error, we still ingest the log
	}

	// 3. Buffer the log
	if err := uc.repo.BufferLog(ctx, *event); err != nil {
		uc.logger.Error("failed to buffer log event", "error", err, "event_id", event.ID)
		// TODO: Implement WAL fallback logic here
		return err
	}

	return nil
}

