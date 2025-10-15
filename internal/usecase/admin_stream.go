package usecase

import (
	"context"
	"time"

	"github.com/user/log-ingestor/internal/domain"
)

// AdminStreamUseCase provides use cases for stream administration.
type AdminStreamUseCase struct {
	repo domain.StreamAdminRepository
}

// NewAdminStreamUseCase creates a new AdminStreamUseCase.
func NewAdminStreamUseCase(repo domain.StreamAdminRepository) *AdminStreamUseCase {
	return &AdminStreamUseCase{repo: repo}
}

func (uc *AdminStreamUseCase) GetGroupInfo(ctx context.Context, stream string) ([]domain.ConsumerGroupInfo, error) {
	return uc.repo.GetGroupInfo(ctx, stream)
}

func (uc *AdminStreamUseCase) GetConsumerInfo(ctx context.Context, stream, group string) ([]domain.ConsumerInfo, error) {
	return uc.repo.GetConsumerInfo(ctx, stream, group)
}

func (uc *AdminStreamUseCase) GetPendingSummary(ctx context.Context, stream, group string) (*domain.PendingMessageSummary, error) {
	return uc.repo.GetPendingSummary(ctx, stream, group)
}

func (uc *AdminStreamUseCase) GetPendingMessages(ctx context.Context, stream, group, consumer string, startID string, count int64) ([]domain.PendingMessageDetail, error) {
	if startID == "" {
		startID = "-"
	}
	if count <= 0 {
		count = 100 // Default count
	}
	return uc.repo.GetPendingMessages(ctx, stream, group, consumer, startID, count)
}

func (uc *AdminStreamUseCase) ClaimMessages(ctx context.Context, stream, group, consumer string, minIdleTime time.Duration, messageIDs []string) ([]domain.LogEvent, error) {
	return uc.repo.ClaimMessages(ctx, stream, group, consumer, minIdleTime, messageIDs)
}

func (uc *AdminStreamUseCase) AcknowledgeMessages(ctx context.Context, stream, group string, messageIDs ...string) (int64, error) {
	return uc.repo.AcknowledgeMessages(ctx, stream, group, messageIDs...)
}

func (uc *AdminStreamUseCase) TrimStream(ctx context.Context, stream string, maxLen int64) (int64, error) {
	return uc.repo.TrimStream(ctx, stream, maxLen)
}

