package usecase

import (
	"context"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/google/uuid"
)

type tailService struct {
	subscriber TailingSubscriber
}

// NewTailService creates a new tailing service.
func NewTailService(subscriber TailingSubscriber) TailUseCase {
	return &tailService{
		subscriber: subscriber,
	}
}

// Tail subscribes to a tenant's real-time log stream.
func (s *tailService) Tail(ctx context.Context, tenantID uuid.UUID, filters map[string]string) (<-chan *domain.Log, error) {
	// The subscriber will handle the context for its own lifecycle.
	return s.subscriber.Subscribe(ctx, tenantID)
}
