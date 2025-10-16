package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

func main() {
	targetURL := flag.String("url", "http://localhost:8080/ingest", "Target URL for ingestion")
	apiKey := flag.String("api-key", "supersecretkey", "API Key for authentication")
	concurrency := flag.Int("c", 10, "Number of concurrent workers")
	duration := flag.Duration("d", 30*time.Second, "Duration of the load test")
	rps := flag.Int("rps", 1000, "Requests per second limit")
	flag.Parse()

	log.Printf("Starting load test on %s", *targetURL)
	log.Printf("Concurrency: %d, Duration: %s, RPS: %d", *concurrency, *duration, *rps)

	var wg sync.WaitGroup
	var successCount, errorCount atomic.Int64
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	limiter := rate.NewLimiter(rate.Limit(*rps), 100) // Allow bursts up to 100

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
					limiter.Wait(ctx) // Wait for token from rate limiter

					eventID := uuid.NewString()
					payload := fmt.Sprintf(`{"event_id": "%s", "message": "load test event from worker %d", "timestamp": "%s"}`,
						eventID, workerID, time.Now().Format(time.RFC3339Nano))

					req, err := http.NewRequestWithContext(ctx, http.MethodPost, *targetURL, bytes.NewBufferString(payload))
					if err != nil {
						continue // Should not happen
					}
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-API-Key", *apiKey)

					resp, err := client.Do(req)
					if err != nil {
						errorCount.Add(1)
						continue
					}

					if resp.StatusCode == http.StatusAccepted {
						successCount.Add(1)
					} else {
						errorCount.Add(1)
					}
					resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()

	totalRequests := successCount.Load() + errorCount.Load()
	actualRPS := float64(totalRequests) / duration.Seconds()

	log.Println("Load test finished.")
	log.Printf("Total Requests: %d", totalRequests)
	log.Printf("Successful (202 Accepted): %d", successCount.Load())
	log.Printf("Errors: %d", errorCount.Load())
	log.Printf("Actual RPS: %.2f", actualRPS)
}
