package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/user/log-ingestor/internal/adapter/metrics"
)

type cacheEntry struct {
	isValid   bool
	expiresAt time.Time
}

// APIKeyRepository implements the domain.APIKeyRepository interface using PostgreSQL
// as the source of truth and an in-memory, time-based cache.
type APIKeyRepository struct {
	db       *sql.DB
	logger   *slog.Logger
	cache    map[string]cacheEntry
	mu       sync.RWMutex
	cacheTTL time.Duration
	metrics  *metrics.IngestMetrics
}

// NewAPIKeyRepository creates a new instance of the PostgreSQL API key repository.
func NewAPIKeyRepository(db *sql.DB, logger *slog.Logger, cacheTTL time.Duration, m *metrics.IngestMetrics) *APIKeyRepository {
	return &APIKeyRepository{
		db:       db,
		logger:   logger,
		cache:    make(map[string]cacheEntry),
		cacheTTL: cacheTTL,
		metrics:  m,
	}
}

// IsValid checks if an API key is valid. It first checks a local cache and falls
// back to the database if the key is not found or the cache entry has expired.
func (r *APIKeyRepository) IsValid(ctx context.Context, key string) (bool, error) {
	// 1. Check cache with a read lock
	r.mu.RLock()
	entry, found := r.cache[key]
	r.mu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		if r.metrics != nil {
			r.metrics.APIKeyCacheHits.Inc()
		}
		return entry.isValid, nil
	}

	// 2. Cache miss or expired, query DB and update cache with a write lock
	if r.metrics != nil {
		r.metrics.APIKeyCacheMisses.Inc()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check cache in case another goroutine populated it while waiting for the lock
	entry, found = r.cache[key]
	if found && time.Now().Before(entry.expiresAt) {
		return entry.isValid, nil
	}

	// 3. Query the database
	var isValid bool
	// A key is valid if it exists, is active, and has not expired.
	query := `SELECT EXISTS(SELECT 1 FROM api_keys WHERE key = $1 AND is_active = true AND (expires_at IS NULL OR expires_at > NOW()))`
	err := r.db.QueryRowContext(ctx, query, key).Scan(&isValid)
	if err != nil {
		r.logger.Error("failed to validate API key in database", "error", err)
		// Don't cache errors, let the next request retry from the DB
		return false, err
	}

	// 4. Update cache
	r.cache[key] = cacheEntry{
		isValid:   isValid,
		expiresAt: time.Now().Add(r.cacheTTL),
	}

	return isValid, nil
}

