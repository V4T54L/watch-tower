package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SSEMessage defines the structure of the message sent to the frontend.
type SSEMessage struct {
	Rate float64 `json:"rate"`
}

// SSEBroker manages SSE client connections and broadcasts messages.
type SSEBroker struct {
	logger       *slog.Logger
	clients      map[chan []byte]struct{}
	mu           sync.RWMutex
	eventCounter chan int
}

// NewSSEBroker creates a new SSEBroker and starts its processing loop.
func NewSSEBroker(ctx context.Context, logger *slog.Logger) *SSEBroker {
	broker := &SSEBroker{
		logger:       logger,
		clients:      make(map[chan []byte]struct{}),
		eventCounter: make(chan int, 1000), // Buffered channel
	}
	go broker.run(ctx)
	return broker
}

// ServeHTTP handles new client connections for the SSE stream.
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan []byte)
	b.addClient(messageChan)
	defer b.removeClient(messageChan)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messageChan:
			if !ok {
				return // Channel was closed
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// ReportEvents is called by the ingest handler to report the number of events processed.
func (b *SSEBroker) ReportEvents(count int) {
	select {
	case b.eventCounter <- count:
	default:
		// Channel is full, drop the report to avoid blocking the ingest path.
		b.logger.Warn("SSE event counter channel is full, dropping report.")
	}
}

func (b *SSEBroker) addClient(client chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[client] = struct{}{}
	b.logger.Info("SSE client connected")
}

func (b *SSEBroker) removeClient(client chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[client]; ok {
		delete(b.clients, client)
		close(client)
		b.logger.Info("SSE client disconnected")
	}
}

func (b *SSEBroker) broadcast(msg []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for client := range b.clients {
		select {
		case client <- msg:
		default:
			// Client channel is full, maybe slow client.
			// We don't block the broadcast for one slow client.
		}
	}
}

// run is the main processing loop for the broker.
func (b *SSEBroker) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var currentCount int
	lastTimestamp := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case count := <-b.eventCounter:
			currentCount += count
		case <-ticker.C:
			now := time.Now()
			duration := now.Sub(lastTimestamp).Seconds()
			rate := 0.0
			if duration > 0 {
				rate = float64(currentCount) / duration
			}

			msg := SSEMessage{Rate: rate}
			jsonData, err := json.Marshal(msg)
			if err != nil {
				b.logger.Error("Failed to marshal SSE message", "error", err)
				continue
			}

			b.broadcast(jsonData)

			// Reset for the next interval
			lastTimestamp = now
			currentCount = 0
		}
	}
}
