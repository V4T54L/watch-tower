package domain

import (
	"encoding/json"
	"time"
)

// LogEvent represents the canonical structure of a log event within the system.
type LogEvent struct {
	ID          string          `json:"event_id"`
	ReceivedAt  time.Time       `json:"received_at"`
	EventTime   time.Time       `json:"event_time"`
	Source      string          `json:"source,omitempty"`
	Level       string          `json:"level,omitempty"`
	Message     string          `json:"message"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	RawEvent    json.RawMessage `json:"-"` // The original raw event, not marshalled to the final sink
	PIIRedacted bool            `json:"pii_redacted,omitempty"`
}

