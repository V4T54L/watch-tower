package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/adapter/repository/postgres"
	redisrepo "github.com/user/log-ingestor/internal/adapter/repository/redis"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
	"github.com/user/log-ingestor/internal/usecase"

	_ "github.com/lib/pq"
)

const (
	consumerGroup    = "log-processors"
	processingInterval = 1 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	appLogger := logger.New(cfg.LogLevel)
	hostname, _ := os.Hostname()
	consumerName := fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8])
	appLogger = appLogger.With("consumer_name", consumerName)

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
		log.Fatalf("failed to connect to redis: %v", err)
	}

	// Repositories
	// The consumer doesn't need a WAL, so we pass nil.
	redisBufferRepo, err := redisrepo.NewLogRepository(redisClient, appLogger, consumerGroup, consumerName, cfg.RedisDLQStream, nil)
	if err != nil {
		log.Fatalf("failed to create redis buffer repository: %v", err)
	}
	pgSinkRepo := postgres.NewLogRepository(db, appLogger)

	// Use Case
	processUseCase := usecase.NewProcessLogsUseCase(
		redisBufferRepo,
		pgSinkRepo,
		appLogger,
		consumerGroup,
		consumerName,
		cfg.ConsumerRetryCount,
		cfg.ConsumerRetryBackoff,
	)

	// Graceful Shutdown Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		appLogger.Info("Shutting down consumer...")
		cancel()
	}()

	appLogger.Info("Starting consumer worker")
	ticker := time.NewTicker(processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			appLogger.Info("Consumer stopped")
			return
		case <-ticker.C:
			processed, err := processUseCase.ProcessBatch(ctx)
			if err != nil {
				appLogger.Error("Error processing batch", "error", err)
			}
			if processed > 0 {
				appLogger.Debug("Processed batch", "count", processed)
			}
		}
	}
}
