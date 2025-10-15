package config

import (
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config holds all configuration for the application, loaded from environment variables.
type Config struct {
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	MaxEventSize   int64  `env:"MAX_EVENT_SIZE_BYTES" envDefault:"1048576"` // 1MB
	WALSegmentSize int64  `env:"WAL_SEGMENT_SIZE_BYTES" envDefault:"104857600"` // 100MB
	WALMaxDiskSize int64  `env:"WAL_MAX_DISK_SIZE_BYTES" envDefault:"1073741824"` // 1GB

	// BackpressurePolicy defines how the ingest API behaves when internal buffers are full.
	// Supported values: "block", "429", "drop".
	BackpressurePolicy string `env:"BACKPRESSURE_POLICY" envDefault:"429"`

	// Redis configuration
	RedisAddr string `env:"REDIS_ADDR,required"`

	// PostgreSQL configuration
	PostgresURL string `env:"POSTGRES_URL,required"`

	// API Key Cache configuration
	APIKeyCacheTTL time.Duration `env:"API_KEY_CACHE_TTL" envDefault:"5m"`
}

// Load parses environment variables into the Config struct.
// It also loads a .env file if it exists, for local development.
func Load() (*Config, error) {
	// Load .env file if it exists. This is useful for local development.
	// In production, environment variables should be set directly.
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

