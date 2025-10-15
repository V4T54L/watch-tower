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
	log.Info("starting ingest gateway...")

	// TODO: Initialize and run ingest server
	// - Initialize repositories (Redis, WAL, PostgreSQL for API keys)
	// - Initialize use cases
	// - Setup HTTP router and handlers
	// - Start HTTP server
	// - Setup signal handling for graceful shutdown

	log.Info("ingest gateway stopped")
}

