package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/user/log-ingestor/internal/adapter/repository/postgres"
	redisrepo "github.com/user/log-ingestor/internal/adapter/repository/redis"
	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
	"github.com/user/log-ingestor/internal/usecase"
)

const (
	consumerGroup      = "log-processors"
	processingInterval = 1 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("starting consumer worker")

	// Create a context that we can cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopChan
		log.Info("shutdown signal received, stopping consumer...")
		cancel()
	}()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	log.Info("connected to redis")

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Error("failed to open postgres connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	log.Info("connected to postgres")

	// Create a unique consumer name for this instance
	consumerName, err := os.Hostname()
	if err != nil {
		log.Warn("could not get hostname for consumer name, using default", "error", err)
		consumerName = "consumer-default"
	}

	// Instantiate repositories
	redisLogRepo, err := redisrepo.NewLogRepository(redisClient, log, consumerGroup, consumerName)
	if err != nil {
		log.Error("failed to create redis log repository", "error", err)
		os.Exit(1)
	}
	pgLogRepo := postgres.NewLogRepository(db, log)

	// Instantiate the use case
	processLogsUseCase := usecase.NewProcessLogsUseCase(redisLogRepo, pgLogRepo, log, consumerGroup, consumerName)

	// Start the consumer processing loop
	ticker := time.NewTicker(processingInterval)
	defer ticker.Stop()

	log.Info("consumer worker started, processing logs...", "group", consumerGroup, "consumer", consumerName)

Loop:
	for {
		select {
		case <-ticker.C:
			processedCount, err := processLogsUseCase.ProcessBatch(ctx)
			if err != nil {
				log.Error("error processing batch", "error", err)
			}
			if processedCount == 0 {
				// No new messages, just wait for the next tick
			}
		case <-ctx.Done():
			log.Info("context cancelled, shutting down consumer loop")
			break Loop
		}
	}

	log.Info("consumer worker shut down gracefully")
}

