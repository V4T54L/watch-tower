package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings" // Keep strings for PII redaction fields split
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/adapter/api"
	"github.com/user/log-ingestor/internal/adapter/api/handler"
	"github.com/user/log-ingestor/internal/adapter/metrics"
	"github.com/user/log-ingestor/internal/adapter/pii"
	"github.com/user/log-ingestor/internal/adapter/repository/postgres"
	redisrepo "github.com/user/log-ingestor/internal/adapter/repository/redis"
	"github.com/user/log-ingestor/internal/adapter/repository/wal"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
	"github.com/user/log-ingestor/internal/usecase"

	_ "github.com/lib/pq" // Keep for postgres driver
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	appLogger := logger.New(cfg.LogLevel)
	appLogger.Info("Configuration loaded")

	// --- Observability Setup ---
	m := metrics.NewIngestMetrics()
	adminMux := http.NewServeMux()
	adminMux.Handle("/metrics", promhttp.Handler())
	adminServer := &http.Server{
		Addr:    ":9091", // Separate port for admin/metrics
		Handler: adminMux,
	}
	go func() {
		appLogger.Info("Starting admin/metrics server", "addr", adminServer.Addr)
		if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("Admin server failed", "error", err)
		}
	}()

	// --- Context and Signal Handling for Graceful Shutdown ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Database Connection
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		appLogger.Error("Failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Redis Client
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		appLogger.Warn("Could not connect to Redis on startup, will rely on WAL", "error", err)
	}

	// WAL Repository
	walRepo, err := wal.NewWALRepository(cfg.WALPath, cfg.WALSegmentSize, cfg.WALMaxDiskSize, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialize WAL repository", "error", err)
		os.Exit(1)
	}
	defer walRepo.Close()

	// Repositories
	apiKeyRepo := postgres.NewAPIKeyRepository(db, appLogger, cfg.APIKeyCacheTTL, m)
	redisRepo, err := redisrepo.NewLogRepository(redisClient, appLogger, "ingest-group", "ingest-consumer", cfg.RedisDLQStream, walRepo, m)
	if err != nil {
		appLogger.Error("Failed to initialize Redis repository", "error", err)
		os.Exit(1)
	}

	// Start Redis health checker and WAL replayer in the background
	go redisRepo.StartHealthCheck(ctx, 5*time.Second)

	// PII Redactor
	piiFields := strings.Split(cfg.PIIRedactionFields, ",") // Keep original way of getting fields
	redactor := pii.NewRedactor(piiFields, appLogger)

	// Use Case
	ingestUseCase := usecase.NewIngestLogUseCase(redisRepo, redactor, appLogger)

	// --- SSE Broker ---
	sseBroker := handler.NewSSEBroker(ctx, appLogger)

	// HTTP Server
	router := api.NewRouter(cfg, appLogger, apiKeyRepo, ingestUseCase, m, sseBroker)
	server := &http.Server{
		Addr:         cfg.IngestServerAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		appLogger.Info("Starting ingest server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("Ingest server failed", "error", err)
			stop() // Trigger shutdown on server error
		}
	}()

	// Graceful Shutdown
	<-ctx.Done()
	appLogger.Info("Shutdown signal received, starting graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Ingest server shutdown failed", "error", err)
	} else {
		appLogger.Info("Ingest server shut down gracefully")
	}

	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("Admin server shutdown failed", "error", err)
	} else {
		appLogger.Info("Admin server shut down gracefully")
	}
}
