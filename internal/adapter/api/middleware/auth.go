package middleware

import (
	"log/slog"
	"net/http"

	"github.com/user/log-ingestor/internal/domain"
)

const APIKeyHeader = "X-API-Key"

// Auth is a middleware factory that returns a new authentication middleware.
// It checks for a valid API key in the X-API-Key header.
func Auth(repo domain.APIKeyRepository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get(APIKeyHeader)
			if apiKey == "" {
				logger.Warn("API key missing from request", "remote_addr", r.RemoteAddr)
				http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
				return
			}

			isValid, err := repo.IsValid(r.Context(), apiKey)
			if err != nil {
				logger.Error("failed to validate API key", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !isValid {
				logger.Warn("invalid API key provided", "remote_addr", r.RemoteAddr)
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

