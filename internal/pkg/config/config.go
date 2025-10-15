package config

import (
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	LogLevel           string        `env:"LOG_LEVEL" envDefault:"info"`
	MaxEventSize       int64         `env:"MAX_EVENT_SIZE_BYTES" envDefault:"1048576"` // 1MB
	WALSegmentSize     int64         `env:"WAL_SEGMENT_SIZE_BYTES" envDefault:"104857600"` // 100MB
	WALMaxDiskSize     int64         `env:"WAL_MAX_DISK_SIZE_BYTES" envDefault:"1073741824"` // 1GB
	BackpressurePolicy string        `env:"BACKPRESSURE_POLICY" envDefault:"block"`
	RedisAddr          string        `env:"REDIS_ADDR,required"`
	PostgresURL        string        `env:"POSTGRES_URL,required"`
	APIKeyCacheTTL     time.Duration `env:"API_KEY_CACHE_TTL" envDefault:"5m"`
	PIIRedactionFields string        `env:"PII_REDACTION_FIELDS" envDefault:"email,password,credit_card,ssn"`
	IngestServerAddr   string        `env:"INGEST_SERVER_ADDR" envDefault:":8080"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Attempt to load .env file for local development.
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

