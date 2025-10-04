package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration.
type Config struct {
	ServerPort                string        `envconfig:"SERVER_PORT" default:"8080"`
	DatabaseURL               string        `envconfig:"DATABASE_URL" required:"true"`
	RedisURL                  string        `envconfig:"REDIS_URL" required:"true"`
	// KafkaBrokers              string        `envconfig:"KAFKA_BROKERS" required:"true"`
	JWTSecret                 string        `envconfig:"JWT_SECRET" required:"true"`
	// KafkaLogsTopic            string        `envconfig:"KAFKA_LOGS_TOPIC" default:"logs.ingest"`
	// KafkaConsumerGroup        string        `envconfig:"KAFKA_CONSUMER_GROUP" default:"log-processor-group"`
	OtelExporterOtlpEndpoint  string        `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	AlerterEvaluationInterval time.Duration `envconfig:"ALERTER_EVALUATION_INTERVAL" default:"60s"`
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// .env file is optional, mainly for local development
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
