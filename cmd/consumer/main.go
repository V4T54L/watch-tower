package main

import (
	"log/slog"
	"os"

	"github.com/user/log-ingestor/internal/pkg/config"
	"github.com/user/log-ingestor/internal/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("starting consumer worker...")

	// TODO: Initialize and run consumer logic
	// - Connect to Redis
	// - Start consumer group processing loop
	// - Connect to PostgreSQL
	// - Setup signal handling for graceful shutdown

	log.Info("consumer worker stopped")
}

