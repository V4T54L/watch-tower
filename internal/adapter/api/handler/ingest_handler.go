package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/user/log-ingestor/internal/domain"
	"github.com/user/log-ingestor/internal/usecase"
)

// IngestHandler handles HTTP requests for log ingestion.
type IngestHandler struct {
	useCase      *usecase.IngestLogUseCase
	logger       *slog.Logger
	maxEventSize int64
}

// NewIngestHandler creates a new IngestHandler.
func NewIngestHandler(uc *usecase.IngestLogUseCase, logger *slog.Logger, maxEventSize int64) *IngestHandler {
	return &IngestHandler{
		useCase:      uc,
		logger:       logger,
		maxEventSize: maxEventSize,
	}
}

// ServeHTTP processes incoming log ingestion requests.
func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce max body size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxEventSize)

	contentType := r.Header.Get("Content-Type")
	var err error

	switch contentType {
	case "application/json":
		err = h.handleSingleJSON(r.Context(), r.Body)
	case "application/x-ndjson":
		err = h.handleNDJSON(r.Context(), r.Body)
	default:
		http.Error(w, "Unsupported Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.logger.Error("failed to process ingest request", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *IngestHandler) handleSingleJSON(ctx context.Context, body io.Reader) error {
	var event domain.LogEvent
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	// To capture the raw event, we need to read the body first
	rawBody, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(rawBody, &event); err != nil {
		return err
	}

	event.RawEvent = rawBody // Store raw event before redaction

	return h.useCase.Ingest(ctx, &event)
}

func (h *IngestHandler) handleNDJSON(ctx context.Context, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event domain.LogEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Log the error but continue processing other lines
			h.logger.Warn("failed to unmarshal ndjson line", "error", err, "line", string(line))
			continue
		}

		event.RawEvent = line // Store raw event before redaction

		if err := h.useCase.Ingest(ctx, &event); err != nil {
			// If a single event fails to be ingested, we log it and continue.
			// A more robust strategy might involve a DLQ at this stage or returning a partial success.
			h.logger.Error("failed to ingest event from ndjson stream", "error", err, "event_id", event.ID)
		}
	}

	return scanner.Err()
}

