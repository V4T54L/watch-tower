package api

import (
	"log/slog"
	"net/http"

	"github.com/V4T54L/watch-tower/internal/adapter/api/handler"
	"github.com/V4T54L/watch-tower/internal/adapter/api/middleware"
	"github.com/V4T54L/watch-tower/internal/adapter/metrics"
	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/V4T54L/watch-tower/internal/pkg/config"
	"github.com/V4T54L/watch-tower/internal/usecase"
)

// NewRouter creates and configures the main HTTP router for the ingest service.
func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	apiKeyRepo domain.APIKeyRepository,
	ingestUseCase usecase.IngestLogUseCase,
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
