package domain

import (
	"encoding/json"
	"time"
)

// LogEvent represents a single log event.
type LogEvent struct {
	ID              string          `json:"event_id"`
	ReceivedAt      time.Time       `json:"received_at"`
	EventTime       time.Time       `json:"event_time"`
	Source          string          `json:"source,omitempty"`
	Level           string          `json:"level,omitempty"`
	Message         string          `json:"message"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	RawEvent        json.RawMessage `json:"-"` // The original raw event payload, not for final serialization.
	PIIRedacted     bool            `json:"pii_redacted,omitempty"`
	StreamMessageID string          `json:"-"` // Transient field for Redis Stream message ID, not serialized.
}

