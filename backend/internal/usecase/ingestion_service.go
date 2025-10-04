package usecase

import (
	"context"
	"encoding/json"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// IngestionProducer defines the contract for publishing to Kafka.
type IngestionProducer interface {
	Publish(ctx context.Context, messages ...kafka.Message) error
}

type ingestionService struct {
	producer      IngestionProducer
	tailPublisher TailingPublisher
}

func NewIngestionService(producer IngestionProducer, tailPublisher TailingPublisher) IngestionUseCase {
	return &ingestionService{
		producer:      producer,
		tailPublisher: tailPublisher,
	}
}

func (s *ingestionService) IngestBatch(ctx context.Context, tenantID uuid.UUID, logs []IngestRequest) (int, int, error) {
	_, span := otel.Tracer("ingestion-service").Start(ctx, "IngestBatch")
	defer span.End()

	var acceptedCount, rejectedCount int
	kafkaMessages := make([]kafka.Message, 0, len(logs))
	acceptedLogs := make([]*domain.Log, 0, len(logs))

	for _, req := range logs {
		// Basic validation
		if req.Message == "" || req.Timestamp.IsZero() {
			rejectedCount++
			continue
		}

		logEntry := &domain.Log{
			ID:        uuid.New(),
			TenantID:  tenantID,
			Timestamp: req.Timestamp,
			Severity:  req.Severity,
			Message:   req.Message,
			Service:   req.Service,
		}

		logJSON, err := json.Marshal(logEntry)
		if err != nil {
			rejectedCount++
			continue
		}

		kafkaMessages = append(kafkaMessages, kafka.Message{
			Key:   []byte(tenantID.String()),
			Value: logJSON,
		})
		acceptedLogs = append(acceptedLogs, logEntry)
		acceptedCount++
	}

	if acceptedCount > 0 {
		if err := s.producer.Publish(ctx, kafkaMessages...); err != nil {
			return 0, len(logs), err // If Kafka fails, reject all
		}

		// Publish to tailing service after successful Kafka publish
		for _, log := range acceptedLogs {
			// This is best-effort, don't fail the whole batch if tailing fails
			_ = s.tailPublisher.Publish(ctx, tenantID, log)
		}
	}

	return acceptedCount, rejectedCount, nil
}
