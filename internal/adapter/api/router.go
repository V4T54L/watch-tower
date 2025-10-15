package api

import (
	"log/slog"
	"net/http"

	"github.com/user/log-ingestor/internal/adapter/api/handler"
	"github.com/user/log-ingestor/internal/adapter/api/middleware"
	"github.com/user/log-ingestor/internal/adapter/metrics"
	"github.com/user/log-ingestor/internal/domain"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/usecase"
)

// NewRouter creates and configures the main HTTP router for the ingest service.
func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	apiKeyRepo domain.APIKeyRepository,
	ingestUseCase *usecase.IngestLogUseCase,
	m *metrics.IngestMetrics,
	sseBroker *handler.SSEBroker,
) http.Handler {
	mux := http.NewServeMux()

	authMiddleware := middleware.Auth(apiKeyRepo, logger)

	// Ingest Handler
	ingestHandler := handler.NewIngestHandler(ingestUseCase, logger, cfg.MaxEventSize, m, sseBroker)

	// Routes
	mux.Handle("POST /ingest", authMiddleware(ingestHandler))
	mux.Handle("/events", sseBroker)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return mux
}

