package config

import (
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config holds all application configuration parameters.
type Config struct {
	LogLevel             string        `env:"LOG_LEVEL" envDefault:"info"`
	MaxEventSize         int64         `env:"MAX_EVENT_SIZE" envDefault:"1048576"`    // 1MB
	WALPath              string        `env:"WAL_PATH" envDefault:"./wal"`             // Path for Write-Ahead Log files
	WALSegmentSize       int64         `env:"WAL_SEGMENT_SIZE" envDefault:"104857600"` // 100MB
	WALMaxDiskSize       int64         `env:"WAL_MAX_DISK_SIZE" envDefault:"1073741824"` // 1GB
	BackpressurePolicy   string        `env:"BACKPRESSURE_POLICY" envDefault:"block"`
	RedisAddr            string        `env:"REDIS_ADDR,required"`
	RedisDLQStream       string        `env:"REDIS_DLQ_STREAM" envDefault:"log_events_dlq"`
	PostgresURL          string        `env:"POSTGRES_URL,required"`
	APIKeyCacheTTL       time.Duration `env:"API_KEY_CACHE_TTL" envDefault:"5m"`
	PIIRedactionFields   string        `env:"PII_REDACTION_FIELDS" envDefault:"email,password,credit_card,ssn"`
	IngestServerAddr     string        `env:"INGEST_SERVER_ADDR" envDefault:":8080"`
	ConsumerRetryCount   int           `env:"CONSUMER_RETRY_COUNT" envDefault:"3"`
	ConsumerRetryBackoff time.Duration `env:"CONSUMER_RETRY_BACKOFF" envDefault:"1s"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists (for local development)
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

