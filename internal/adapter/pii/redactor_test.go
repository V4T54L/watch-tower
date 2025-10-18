package pii

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/V4T54L/watch-tower/internal/domain"
)

func TestRedactor(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(nil, nil))
	redactor := NewRedactor([]string{"email", "ssn"}, logger)

	tests := []struct {
		name             string
		inputMetadata    string
		expectedMetadata string
		expectRedacted   bool
		expectErr        bool
	}{
		{
			name:             "Redact single field",
			inputMetadata:    `{"email": "test@example.com", "user_id": 123}`,
			expectedMetadata: `{"email":"[REDACTED]","user_id":123}`,
			expectRedacted:   true,
			expectErr:        false,
		},
		{
			name:             "Redact multiple fields",
			inputMetadata:    `{"email": "test@example.com", "ssn": "000-00-0000"}`,
			expectedMetadata: `{"email":"[REDACTED]","ssn":"[REDACTED]"}`,
			expectRedacted:   true,
			expectErr:        false,
		},
		{
			name:             "No fields to redact",
			inputMetadata:    `{"user_id": 123, "action": "login"}`,
			expectedMetadata: `{"action":"login","user_id":123}`,
			expectRedacted:   false,
			expectErr:        false,
		},
		{
			name:             "Empty metadata",
			inputMetadata:    `{}`,
			expectedMetadata: `{}`,
			expectRedacted:   false,
			expectErr:        false,
		},
		{
			name:             "Invalid JSON metadata",
			inputMetadata:    `{"email": "test@example.com"`,
			expectedMetadata: "",
			expectRedacted:   false,
			expectErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &domain.LogEvent{
				Metadata: json.RawMessage(tt.inputMetadata),
			}

			err := redactor.Redact(event)

			if (err != nil) != tt.expectErr {
				t.Fatalf("Redact() error = %v, wantErr %v", err, tt.expectErr)
			}

			if err != nil {
				return
			}

			if event.PIIRedacted != tt.expectRedacted {
				t.Errorf("event.PIIRedacted got = %v, want %v", event.PIIRedacted, tt.expectRedacted)
			}

			// Unmarshal both to compare maps, avoiding key order issues
			var expectedMap, actualMap map[string]interface{}
			if err := json.Unmarshal([]byte(tt.expectedMetadata), &expectedMap); err != nil {
				t.Fatalf("failed to unmarshal expected metadata: %v", err)
			}
			if err := json.Unmarshal(event.Metadata, &actualMap); err != nil {
				t.Fatalf("failed to unmarshal actual metadata: %v", err)
			}

			if len(expectedMap) != len(actualMap) {
				t.Errorf("metadata map length mismatch: got %d, want %d", len(actualMap), len(expectedMap))
			}
			for k, v := range expectedMap {
				if actualMap[k] != v {
					t.Errorf("metadata mismatch for key %s: got %v, want %v", k, actualMap[k], v)
				}
			}
		})
	}
}
