package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/user/log-ingestor/internal/adapter/api"
	"github.com/user/log-ingestor/internal/adapter/api/handler"
	"github.com/user/log-ingestor/internal/adapter/api/middleware"
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
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := logger.New(cfg.LogLevel)
	slog.SetDefault(logger)

	m := metrics.NewIngestMetrics()

	// --- Start Admin and Metrics Server ---
	adminMux := http.NewServeMux()
	adminMux.Handle("/metrics", promhttp.Handler())

	adminServer := &http.Server{
		Addr:    ":9091",
		Handler: adminMux,
	}

	go func() {
		logger.Info("starting admin & metrics server", "addr", adminServer.Addr)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("admin & metrics server failed", "error", err)
		}
	}()

	// --- Graceful Shutdown Context ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Database and Redis Connections ---
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisAddr)
	if err != nil {
		logger.Error("failed to parse redis url", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Warn("could not connect to redis, will proceed in WAL-only mode", "error", err)
	}

	// --- Initialize Repositories ---
	walRepo, err := wal.NewWALRepository(cfg.WALPath, cfg.WALSegmentSize, cfg.WALMaxDiskSize, logger)
	if err != nil {
		logger.Error("failed to initialize WAL repository", "error", err)
		os.Exit(1)
	}
	defer walRepo.Close()

	apiKeyRepo := postgres.NewAPIKeyRepository(db, logger, cfg.APIKeyCacheTTL, m)
	redisLogRepo, err := redisrepo.NewLogRepository(redisClient, logger, "log-processors", "ingest-service", cfg.RedisDLQStream, walRepo, m)
	if err != nil && !errors.Is(err, redisrepo.ErrRedisNotAvailable) {
		logger.Error("failed to initialize redis log repository", "error", err)
		os.Exit(1)
	}

	// Start Redis health check and WAL replay loop
	go redisLogRepo.StartHealthCheck(ctx, 5*time.Second)

	// --- Initialize Admin API ---
	redisAdminRepo := redisrepo.NewAdminRepository(redisClient, logger)
	adminUseCase := usecase.NewAdminStreamUseCase(redisAdminRepo)
	adminRouter := api.NewAdminRouter(adminUseCase, logger)
	adminMux.Handle("/", adminRouter) // Mount admin router at the root of the admin server

	// --- Initialize Use Cases and Services ---
	piiRedactor := pii.NewRedactor(strings.Split(cfg.PIIRedactionFields, ","), logger)
	ingestUseCase := usecase.NewIngestLogUseCase(redisLogRepo, piiRedactor, logger)

	// --- Initialize SSE Broker ---
	sseBroker := handler.NewSSEBroker(ctx, logger)

	// --- Initialize Ingest Server ---
	ingestRouter := api.NewRouter(cfg, logger, apiKeyRepo, ingestUseCase, m, sseBroker)
	ingestServer := &http.Server{
		Addr:         cfg.IngestServerAddr,
		Handler:      middleware.Logging(logger)(ingestRouter),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		logger.Info("starting ingest server", "addr", ingestServer.Addr)
		if err := ingestServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("ingest server failed", "error", err)
			stop() // Trigger shutdown on server error
		}
	}()

	// --- Wait for shutdown signal ---
	<-ctx.Done()
	logger.Info("shutting down servers...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin server shutdown failed", "error", err)
	}
	if err := ingestServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("ingest server shutdown failed", "error", err)
	}

	logger.Info("servers shut down gracefully")
}

