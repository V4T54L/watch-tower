package notifier

import (
	"context"

	"github.com/V4T54L/watch-tower/internal/domain"
)

// Notifier defines the interface for sending alert notifications.
type Notifier interface {
	Notify(ctx context.Context, rule *domain.AlertRule, count int) error
}
