package api

import (
	"log/slog"
	"net/http"

	"github.com/user/log-ingestor/internal/adapter/api/handler"
	"github.com/user/log-ingestor/internal/adapter/api/middleware"
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
) http.Handler {
	mux := http.NewServeMux()

	// Ingest Handler
	ingestHandler := handler.NewIngestHandler(ingestUseCase, logger, cfg.MaxEventSize)

	// Middleware
	authMiddleware := middleware.Auth(apiKeyRepo, logger)

	// Routes
	mux.Handle("POST /ingest", authMiddleware(ingestHandler))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// TODO: Add /metrics and /events endpoints in future steps

	return mux
}

