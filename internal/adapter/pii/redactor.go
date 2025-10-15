package pii

import (
	"encoding/json"
	"log/slog"

	"github.com/user/log-ingestor/internal/domain"
)

const RedactedPlaceholder = "[REDACTED]"

// Redactor is responsible for redacting sensitive information from log events.
type Redactor struct {
	fieldsToRedact map[string]struct{} // Use a map for O(1) lookups
	logger         *slog.Logger
}

// NewRedactor creates a new Redactor instance with a given set of fields to redact.
func NewRedactor(fields []string, logger *slog.Logger) *Redactor {
	fieldSet := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldSet[field] = struct{}{}
	}
	return &Redactor{
		fieldsToRedact: fieldSet,
		logger:         logger,
	}
}

// Redact modifies the LogEvent in place to remove PII from its metadata.
// It returns an error if JSON processing fails.
func (r *Redactor) Redact(event *domain.LogEvent) error {
	if len(r.fieldsToRedact) == 0 || len(event.Metadata) == 0 {
		return nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
		r.logger.Warn("failed to unmarshal metadata for PII redaction", "error", err, "event_id", event.ID)
		// We can't process it, so we leave it as is.
		return err
	}

	redacted := false
	for field := range r.fieldsToRedact {
		if _, ok := metadata[field]; ok {
			metadata[field] = RedactedPlaceholder
			redacted = true
		}
	}

	if redacted {
		event.PIIRedacted = true
		modifiedMetadata, err := json.Marshal(metadata)
		if err != nil {
			r.logger.Error("failed to marshal modified metadata after PII redaction", "error", err, "event_id", event.ID)
			// This is a more serious internal error.
			return err
		}
		event.Metadata = modifiedMetadata
	}

	return nil
}

