package mocks

import (
	"context"
	"sync"

	"github.com/user/log-ingestor/internal/domain"
)

// MockLogRepository is a mock implementation of domain.LogRepository for testing.
type MockLogRepository struct {
	mu              sync.Mutex
	BufferedEvents  []domain.LogEvent
	WrittenEvents   []domain.LogEvent
	AckedMessageIDs []string
	DLQEvents       []domain.LogEvent
	ReadBatchResult []domain.LogEvent
	BufferErr       error
	ReadErr         error
	WriteErr        error
	AckErr          error
	DLQErr          error
}

func (m *MockLogRepository) BufferLog(ctx context.Context, event domain.LogEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.BufferErr != nil {
		return m.BufferErr
	}
	m.BufferedEvents = append(m.BufferedEvents, event)
	return nil
}

func (m *MockLogRepository) ReadLogBatch(ctx context.Context, group, consumer string, count int) ([]domain.LogEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ReadErr != nil {
		return nil, m.ReadErr
	}
	return m.ReadBatchResult, nil
}

func (m *MockLogRepository) WriteLogBatch(ctx context.Context, events []domain.LogEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.WriteErr != nil {
		return m.WriteErr
	}
	m.WrittenEvents = append(m.WrittenEvents, events...)
	return nil
}

func (m *MockLogRepository) AcknowledgeLogs(ctx context.Context, group string, messageIDs ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AckErr != nil {
		return m.AckErr
	}
	m.AckedMessageIDs = append(m.AckedMessageIDs, messageIDs...)
	return nil
}

func (m *MockLogRepository) MoveToDLQ(ctx context.Context, events []domain.LogEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DLQErr != nil {
		return m.DLQErr
	}
	m.DLQEvents = append(m.DLQEvents, events...)
	return nil
}

