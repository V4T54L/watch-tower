package wal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/V4T54L/watch-tower/internal/domain"
)

const (
	segmentPrefix = "segment-"
	filePerm      = 0644
)

// WALRepository implements a file-based Write-Ahead Log.
type WALRepository struct {
	dir            string
	maxSegmentSize int64
	maxTotalSize   int64
	logger         *slog.Logger

	mu             sync.Mutex
	currentSegment *os.File
	currentSize    int64
}

// NewWALRepository creates a new WALRepository.
func NewWALRepository(dir string, maxSegmentSize, maxTotalSize int64, logger *slog.Logger) (*WALRepository, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory %s: %w", dir, err)
	}

	w := &WALRepository{
		dir:            dir,
		maxSegmentSize: maxSegmentSize,
		maxTotalSize:   maxTotalSize,
		logger:         logger.With("component", "wal_repository"),
	}

	if err := w.openLatestSegment(); err != nil {
		return nil, err
	}

	return w, nil
}

// Write appends an event to the current WAL segment.
func (w *WALRepository) Write(ctx context.Context, event domain.LogEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal log event for WAL: %w", err)
	}
	data = append(data, '\n')

	if w.currentSegment == nil {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	// Check total size before writing
	totalSize, err := w.calculateTotalSize()
	if err != nil {
		w.logger.Error("Failed to calculate total WAL size", "error", err)
		return fmt.Errorf("could not verify WAL disk space: %w", err)
	}
	if totalSize+int64(len(data)) > w.maxTotalSize {
		return fmt.Errorf("WAL max total size exceeded (%d > %d)", totalSize, w.maxTotalSize)
	}

	n, err := w.currentSegment.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to WAL segment: %w", err)
	}
	w.currentSize += int64(n)

	if w.currentSize >= w.maxSegmentSize {
		if err := w.rotate(); err != nil {
			w.logger.Error("Failed to rotate WAL segment", "error", err)
		}
	}

	return nil
}

// Replay reads all WAL segments and calls the handler for each event.
func (w *WALRepository) Replay(ctx context.Context, handler func(event domain.LogEvent) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentSegment != nil {
		w.currentSegment.Close()
		w.currentSegment = nil
	}

	segments, err := w.getSortedSegments()
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		w.logger.Info("WAL is empty, nothing to replay")
		return nil
	}
	w.logger.Info("Starting WAL replay", "segment_count", len(segments))

	for _, segmentPath := range segments {
		file, err := os.Open(segmentPath)
		if err != nil {
			return fmt.Errorf("failed to open segment %s for replay: %w", segmentPath, err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if ctx.Err() != nil {
				file.Close()
				return ctx.Err()
			}
			var event domain.LogEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				w.logger.Warn("Failed to unmarshal event from WAL, skipping", "error", err, "line", scanner.Text())
				continue
			}
			if err := handler(event); err != nil {
				file.Close()
				w.logger.Error("WAL replay handler failed, stopping replay", "error", err)
				return fmt.Errorf("replay handler failed: %w", err)
			}
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			return fmt.Errorf("error scanning segment %s: %w", segmentPath, err)
		}
		file.Close()
	}

	w.logger.Info("WAL replay completed")
	return nil
}

// Truncate removes all WAL segment files.
func (w *WALRepository) Truncate(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentSegment != nil {
		w.currentSegment.Close()
		w.currentSegment = nil
	}

	segments, err := w.getSortedSegments()
	if err != nil {
		return err
	}

	for _, segmentPath := range segments {
		if err := os.Remove(segmentPath); err != nil {
			w.logger.Error("Failed to remove WAL segment", "path", segmentPath, "error", err)
		}
	}

	w.logger.Info("WAL truncated")
	return w.openLatestSegment()
}

func (w *WALRepository) rotate() error {
	if w.currentSegment != nil {
		if err := w.currentSegment.Sync(); err != nil {
			w.logger.Error("Failed to sync WAL segment before rotating", "error", err)
		}
		if err := w.currentSegment.Close(); err != nil {
			w.logger.Error("Failed to close WAL segment before rotating", "error", err)
		}
		w.currentSegment = nil
	}

	segmentName := fmt.Sprintf("%s%d.log", segmentPrefix, time.Now().UnixNano())
	path := filepath.Join(w.dir, segmentName)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("failed to create new WAL segment %s: %w", path, err)
	}

	w.currentSegment = f
	w.currentSize = 0
	w.logger.Info("Rotated to new WAL segment", "path", path)
	return nil
}

func (w *WALRepository) openLatestSegment() error {
	segments, err := w.getSortedSegments()
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		return w.rotate()
	}

	latestSegmentPath := segments[len(segments)-1]
	stat, err := os.Stat(latestSegmentPath)
	if err != nil {
		return fmt.Errorf("failed to stat latest segment %s: %w", latestSegmentPath, err)
	}

	f, err := os.OpenFile(latestSegmentPath, os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("failed to open latest segment %s: %w", latestSegmentPath, err)
	}

	w.currentSegment = f
	w.currentSize = stat.Size()
	w.logger.Info("Opened existing WAL segment", "path", latestSegmentPath, "size", w.currentSize)

	if w.currentSize >= w.maxSegmentSize {
		return w.rotate()
	}

	return nil
}

func (w *WALRepository) getSortedSegments() ([]string, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL directory: %w", err)
	}

	var segments []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), segmentPrefix) {
			segments = append(segments, filepath.Join(w.dir, entry.Name()))
		}
	}
	sort.Strings(segments)
	return segments, nil
}

func (w *WALRepository) calculateTotalSize() (int64, error) {
	var totalSize int64
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), segmentPrefix) {
			info, err := entry.Info()
			if err != nil {
				return 0, err
			}
			totalSize += info.Size()
		}
	}
	return totalSize, nil
}

// Close ensures the current segment is closed gracefully.
func (w *WALRepository) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentSegment != nil {
		return w.currentSegment.Close()
	}
	return nil
}
