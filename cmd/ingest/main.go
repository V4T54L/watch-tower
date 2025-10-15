package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/adapter/api"
	"github.com/user/log-ingestor/internal/adapter/pii"
	"github.com/user/log-ingestor/internal/adapter/repository/postgres"
	redisrepo "github.com/user/log-ingestor/internal/adapter/repository/redis"
	"github.com/user/log-ingestor/internal/adapter/repository/wal"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
	"github.com/user/log-ingestor/internal/usecase"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	appLogger := logger.New(cfg.LogLevel)

	// Database Connection
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer db.Close()

	// Redis Client
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		appLogger.Warn("Could not connect to Redis on startup, will operate in degraded mode", "error", err)
	}

	// WAL Repository
	walRepo, err := wal.NewWALRepository(cfg.WALPath, cfg.WALSegmentSize, cfg.WALMaxDiskSize, appLogger)
	if err != nil {
		log.Fatalf("failed to initialize WAL repository: %v", err)
	}
	defer walRepo.Close()

	// Repositories
	apiKeyRepo := postgres.NewAPIKeyRepository(db, appLogger, cfg.APIKeyCacheTTL)
	redisRepo, err := redisrepo.NewLogRepository(redisClient, appLogger, "ingest-group", "ingest-worker", cfg.RedisDLQStream, walRepo)
	if err != nil {
		log.Fatalf("failed to create redis log repository: %v", err)
	}

	// Start Redis health checker and WAL replayer in the background
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	go redisRepo.StartHealthCheck(appCtx, 10*time.Second)

	// PII Redactor
	piiFields := strings.Split(cfg.PIIRedactionFields, ",")
	redactor := pii.NewRedactor(piiFields, appLogger)

	// Use Case
	ingestUseCase := usecase.NewIngestLogUseCase(redisRepo, redactor, appLogger)

	// HTTP Server
	router := api.NewRouter(cfg, appLogger, apiKeyRepo, ingestUseCase)
	server := &http.Server{
		Addr:    cfg.IngestServerAddr,
		Handler: router,
	}

	go func() {
		appLogger.Info("Starting ingest server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", server.Addr, err)
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	appLogger.Info("Shutting down server...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	appLogger.Info("Server gracefully stopped")
}

