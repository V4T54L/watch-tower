package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/V4T54L/watch-tower/internal/adapter/metrics"
	"github.com/V4T54L/watch-tower/internal/domain"
)

// MockIngestUseCase is a mock implementation of the IngestLogUseCase.
type MockIngestUseCase struct {
	IngestFunc func(ctx context.Context, event *domain.LogEvent) error
}

func (m *MockIngestUseCase) Ingest(ctx context.Context, event *domain.LogEvent) error {
	if m.IngestFunc != nil {
		return m.IngestFunc(ctx, event)
	}
	return nil
}

func TestIngestHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockMetrics := metrics.NewIngestMetrics()
	mockSSEBroker := NewSSEBroker(context.Background(), logger)

	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		mockIngestErr  error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid Single JSON",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"message": "hello"}`,
			expectedStatus: http.StatusAccepted,
			expectedBody:   "",
		},
		{
			name:           "Valid NDJSON",
			method:         http.MethodPost,
			contentType:    "application/x-ndjson",
			body:           `{"message": "line 1"}` + "\n" + `{"message": "line 2"}`,
			expectedStatus: http.StatusAccepted,
			expectedBody:   "",
		},
		{
			name:           "Invalid Method",
			method:         http.MethodGet,
			contentType:    "application/json",
			body:           `{}`,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name:           "Unsupported Content-Type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			body:           `hello`,
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedBody:   "Unsupported Media Type: text/plain\n",
		},
		{
			name:           "Bad JSON",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"message": "hello"`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Failed to decode JSON\n",
		},
		{
			name:           "Bad NDJSON line",
			method:         http.MethodPost,
			contentType:    "application/x-ndjson",
			body:           `{"message": "line 1"}` + "\n" + `{"message": "bad`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Failed to decode NDJSON line\n",
		},
		{
			name:           "Ingest Use Case Error",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"message": "fail me"}`,
			mockIngestErr:  errors.New("internal buffer error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:           "Payload Too Large",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"message": "this payload is definitely too large for the test limit"}`,
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedBody:   "http: request body too large\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUseCase := &MockIngestUseCase{
				IngestFunc: func(ctx context.Context, event *domain.LogEvent) error {
					return tt.mockIngestErr
				},
			}
			// Use a small max size for the "Payload Too Large" test
			maxSize := int64(1024)
			if tt.name == "Payload Too Large" {
				maxSize = 50
			}

			handler := NewIngestHandler(mockUseCase, logger, maxSize, mockMetrics, mockSSEBroker)

			req := httptest.NewRequest(tt.method, "/ingest", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			if body := rr.Body.String(); body != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %q want %q", body, tt.expectedBody)
			}
		})
	}
}
