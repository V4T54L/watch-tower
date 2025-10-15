package wal

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/user/log-ingestor/internal/domain"
)

func setupTestWAL(t *testing.T, maxSegmentSize, maxTotalSize int64) (*WALRepository, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "wal_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	wal, err := NewWALRepository(dir, maxSegmentSize, maxTotalSize, logger)
	if err != nil {
		t.Fatalf("failed to create WALRepository: %v", err)
	}

	cleanup := func() {
		wal.Close()
		os.RemoveAll(dir)
	}

	return wal, cleanup
}

func TestWAL_WriteAndReplay(t *testing.T) {
	wal, cleanup := setupTestWAL(t, 1024, 10*1024)
	defer cleanup()

	events := []domain.LogEvent{
		{ID: uuid.NewString(), Message: "event 1"},
		{ID: uuid.NewString(), Message: "event 2"},
		{ID: uuid.NewString(), Message: "event 3"},
	}

	for _, event := range events {
		if err := wal.Write(context.Background(), event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}
	wal.Close() // Close to ensure data is flushed

	// Re-open the WAL to simulate a restart
	var err error
	wal, err = NewWALRepository(wal.dir, 1024, 10*1024, wal.logger)
	if err != nil {
		t.Fatalf("failed to re-open WAL: %v", err)
	}

	var replayedEvents []domain.LogEvent
	replayHandler := func(event domain.LogEvent) error {
		replayedEvents = append(replayedEvents, event)
		return nil
	}

	if err := wal.Replay(context.Background(), replayHandler); err != nil {
		t.Fatalf("failed to replay events: %v", err)
	}

	if len(replayedEvents) != len(events) {
		t.Fatalf("expected %d replayed events, got %d", len(events), len(replayedEvents))
	}

	for i, event := range events {
		if replayedEvents[i].ID != event.ID || replayedEvents[i].Message != event.Message {
			t.Errorf("replayed event mismatch at index %d: got %+v, want %+v", i, replayedEvents[i], event)
		}
	}
}

func TestWAL_SegmentRotation(t *testing.T) {
	// Set a very small segment size to force rotation
	wal, cleanup := setupTestWAL(t, 100, 1024)
	defer cleanup()

	event := domain.LogEvent{ID: uuid.NewString(), Message: "a message long enough to cause rotation"}
	eventBytes, _ := json.Marshal(event)
	eventSize := len(eventBytes)

	// Write enough events to create at least 2 segments
	numWrites := (100 / eventSize) + 2
	for i := 0; i < numWrites; i++ {
		if err := wal.Write(context.Background(), event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	segments, err := wal.getSortedSegments()
	if err != nil {
		t.Fatalf("failed to get segments: %v", err)
	}

	if len(segments) < 2 {
		t.Errorf("expected at least 2 segments, got %d", len(segments))
	}
}

func TestWAL_Truncate(t *testing.T) {
	wal, cleanup := setupTestWAL(t, 1024, 1024)
	defer cleanup()

	event := domain.LogEvent{ID: uuid.NewString(), Message: "some data"}
	if err := wal.Write(context.Background(), event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	segments, _ := wal.getSortedSegments()
	if len(segments) == 0 {
		t.Fatal("expected at least one segment before truncate")
	}

	if err := wal.Truncate(context.Background()); err != nil {
		t.Fatalf("failed to truncate WAL: %v", err)
	}

	segments, _ = wal.getSortedSegments()
	if len(segments) != 1 { // Truncate creates a new empty segment
		t.Errorf("expected 1 segment after truncate, got %d", len(segments))
	}
	info, _ := os.Stat(segments[0])
	if info.Size() != 0 {
		t.Errorf("expected new segment to be empty, size is %d", info.Size())
	}
}

func TestWAL_MaxTotalSize(t *testing.T) {
	wal, cleanup := setupTestWAL(t, 100, 150) // Max total size is very small
	defer cleanup()

	event := domain.LogEvent{ID: uuid.NewString(), Message: "some data that will fill up the WAL"}
	var err error
	for i := 0; i < 5; i++ { // Write until we expect an error
		err = wal.Write(context.Background(), event)
		if err != nil {
			break
		}
	}

	if err == nil {
		t.Fatal("expected an error when writing beyond max total size, but got nil")
	}
}

