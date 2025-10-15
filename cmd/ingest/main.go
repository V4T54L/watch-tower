package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/adapter/api"
	pii_adapter "github.com/user/log-ingestor/internal/adapter/pii"
	postgres_repo "github.com/user/log-ingestor/internal/adapter/repository/postgres"
	redis_repo "github.com/user/log-ingestor/internal/adapter/repository/redis"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
	"github.com/user/log-ingestor/internal/usecase"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Use slog here because the custom logger 'log' is not yet initialized.
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize logger
	log := logger.New(cfg.LogLevel)
	log.Info("starting ingest gateway")
	defer log.Info("ingest gateway shut down")

	// 3. Initialize dependencies (DB, Redis, etc.)
	// PostgreSQL
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Error("failed to ping postgres", "error", err)
		os.Exit(1)
	}

	// Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Error("failed to connect to redis", "error", err)
		// TODO: In a future step, this should trigger WAL mode instead of exiting.
		os.Exit(1)
	}
	defer redisClient.Close()

	// 4. Initialize repositories
	apiKeyRepo := postgres_repo.NewAPIKeyRepository(db, log, cfg.APIKeyCacheTTL)
	logRepo := redis_repo.NewLogRepository(redisClient, log)
	// TODO: Initialize WAL repository

	// 5. Initialize adapters
	piiFields := strings.Split(cfg.PIIRedactionFields, ",")
	piiRedactor := pii_adapter.NewRedactor(piiFields, log)

	// 6. Initialize use cases
	ingestUseCase := usecase.NewIngestLogUseCase(logRepo, piiRedactor, log)

	// 7. Setup HTTP router and server
	router := api.NewRouter(cfg, log, apiKeyRepo, ingestUseCase)
	server := &http.Server{
		Addr:    cfg.IngestServerAddr,
		Handler: router,
	}

	// 8. Start server and handle graceful shutdown
	go func() {
		log.Info("server listening", "addr", cfg.IngestServerAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
}
