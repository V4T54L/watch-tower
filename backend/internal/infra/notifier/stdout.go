package notifier

import (
	"context"
	"fmt"

	"github.com/V4T54L/watch-tower/internal/domain"
)

// StdoutNotifier is an implementation of Notifier that prints to standard output.
type StdoutNotifier struct{}

// NewStdoutNotifier creates a new StdoutNotifier.
func NewStdoutNotifier() *StdoutNotifier {
	return &StdoutNotifier{}
}

// Notify prints the alert details to stdout.
func (n *StdoutNotifier) Notify(ctx context.Context, rule *domain.AlertRule, count int) error {
	fmt.Printf(
		"--- ALERT TRIGGERED ---\nRule Name: %s\nTenant ID: %s\nThreshold: %d\nActual Count: %d\nQuery: %s\n-----------------------\n",
		rule.Name,
		rule.TenantID.String(),
		rule.Threshold,
		count,
		rule.Query,
	)
	return nil
}
