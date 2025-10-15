package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/user/log-ingestor/internal/adapter/metrics"
	"github.com/user/log-ingestor/internal/domain"
	"github.com/user/log-ingestor/internal/usecase"
)

const (
	contentTypeJSON   = "application/json"
	contentTypeNDJSON = "application/x-ndjson"
)

// IngestHandler handles HTTP requests for log ingestion.
type IngestHandler struct {
	useCase      *usecase.IngestLogUseCase
	logger       *slog.Logger
	maxEventSize int64
	metrics      *metrics.IngestMetrics
	sseBroker    *SSEBroker
}

// NewIngestHandler creates a new IngestHandler.
func NewIngestHandler(uc *usecase.IngestLogUseCase, logger *slog.Logger, maxEventSize int64, m *metrics.IngestMetrics, sse *SSEBroker) *IngestHandler {
	return &IngestHandler{
		useCase:      uc,
		logger:       logger,
		maxEventSize: maxEventSize,
		metrics:      m,
		sseBroker:    sse,
	}
}

// ServeHTTP processes incoming log ingestion requests.
func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	h.metrics.BytesTotal.Add(float64(r.ContentLength))
	// Enforce max body size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxEventSize)

	contentType := r.Header.Get("Content-Type")
	var err error

	switch {
	case strings.HasPrefix(contentType, contentTypeJSON):
		err = h.handleSingleJSON(r.Context(), r.Body)
	case strings.HasPrefix(contentType, contentTypeNDJSON):
		err = h.handleNDJSON(r.Context(), r.Body)
	default:
		h.metrics.EventsTotal.WithLabelValues("error_media_type").Inc()
		http.Error(w, "Unsupported Content-Type. Use application/json or application/x-ndjson.", http.StatusUnsupportedMediaType)
		return
	}

	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.metrics.EventsTotal.WithLabelValues("error_size").Inc()
			http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
		} else {
			h.logger.Error("Failed to process request", "error", err)
			http.Error(w, "Failed to process request", http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *IngestHandler) handleSingleJSON(ctx context.Context, body io.Reader) error {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	var event domain.LogEvent
	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		h.metrics.EventsTotal.WithLabelValues("error_parse").Inc()
		return err
	}
	event.RawEvent = bodyBytes

	if err := h.useCase.Ingest(ctx, &event); err != nil {
		h.metrics.EventsTotal.WithLabelValues("error_buffer").Inc()
		return err
	}

	h.metrics.EventsTotal.WithLabelValues("accepted").Inc()
	h.sseBroker.ReportEvents(1)
	return nil
}

func (h *IngestHandler) handleNDJSON(ctx context.Context, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	var processedCount int
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event domain.LogEvent
		if err := json.Unmarshal(line, &event); err != nil {
			h.logger.Warn("Failed to unmarshal NDJSON line, skipping", "error", err)
			h.metrics.EventsTotal.WithLabelValues("error_parse").Inc()
			continue
		}
		event.RawEvent = line

		if err := h.useCase.Ingest(ctx, &event); err != nil {
			h.logger.Error("Failed to ingest event from NDJSON stream", "error", err)
			h.metrics.EventsTotal.WithLabelValues("error_buffer").Inc()
			// Continue processing other lines
			continue
		}
		processedCount++
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if processedCount > 0 {
		h.metrics.EventsTotal.WithLabelValues("accepted").Add(float64(processedCount))
		h.sseBroker.ReportEvents(processedCount)
	}

	return nil
}

