package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// IngestMetrics holds all Prometheus metrics for the ingest service.
type IngestMetrics struct {
	EventsTotal       *prometheus.CounterVec
	BytesTotal        prometheus.Counter
	WALActive         prometheus.Gauge
	APIKeyCacheHits   prometheus.Counter
	APIKeyCacheMisses prometheus.Counter
}

// NewIngestMetrics initializes and registers the Prometheus metrics.
func NewIngestMetrics() *IngestMetrics {
	return &IngestMetrics{
		EventsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "log_ingestor",
			Subsystem: "ingest",
			Name:      "events_total",
			Help:      "Total number of ingested events by status.",
		}, []string{"status"}), // status: accepted, error_parse, error_size, error_buffer, error_media_type
		BytesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "log_ingestor",
			Subsystem: "ingest",
			Name:      "bytes_total",
			Help:      "Total number of bytes ingested.",
		}),
		WALActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "log_ingestor",
			Subsystem: "ingest",
			Name:      "wal_active_gauge",
			Help:      "Indicates if the Write-Ahead Log is currently active (1 for active, 0 for inactive).",
		}),
		APIKeyCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "log_ingestor",
			Subsystem: "auth",
			Name:      "api_key_cache_hits_total",
			Help:      "Total number of API key cache hits.",
		}),
		APIKeyCacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "log_ingestor",
			Subsystem: "auth",
			Name:      "api_key_cache_misses_total",
			Help:      "Total number of API key cache misses.",
		}),
	}
}
