package api

import (
	"log/slog"
	"net/http"

	"github.com/user/log-ingestor/internal/adapter/api/handler"
	"github.comcom/user/log-ingestor/internal/usecase"
)

// NewAdminRouter creates and configures the HTTP router for admin operations.
// Note: This router uses path patterns (e.g., "/{streamName}/") available in Go 1.22+.
func NewAdminRouter(adminUseCase *usecase.AdminStreamUseCase, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	adminHandler := handler.NewAdminHandler(adminUseCase, logger)

	mux.HandleFunc("GET /health", adminHandler.HealthCheck)

	// Stream Info
	mux.HandleFunc("GET /admin/streams/{streamName}/groups", adminHandler.GetGroupInfo)
	mux.HandleFunc("GET /admin/streams/{streamName}/groups/{groupName}/consumers", adminHandler.GetConsumerInfo)

	// Pending Messages
	mux.HandleFunc("GET /admin/streams/{streamName}/groups/{groupName}/pending", adminHandler.GetPendingSummary)
	mux.HandleFunc("GET /admin/streams/{streamName}/groups/{groupName}/pending/messages", adminHandler.GetPendingMessages)

	// Stream Operations
	mux.HandleFunc("POST /admin/streams/{streamName}/groups/{groupName}/claim", adminHandler.ClaimMessages)
	mux.HandleFunc("POST /admin/streams/{streamName}/groups/{groupName}/ack", adminHandler.AcknowledgeMessages)
	mux.HandleFunc("POST /admin/streams/{streamName}/trim", adminHandler.TrimStream)

	return mux
}

