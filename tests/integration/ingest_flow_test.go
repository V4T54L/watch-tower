package integration

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	ingestorURL = "http://localhost:8080/ingest"
	postgresDSN = "postgres://testuser:testpassword@localhost:5432/testdb?sslmode=disable"
)

// TestMain manages the lifecycle of the docker-compose environment for integration tests.
func TestMain(m *testing.M) {
	// Start docker-compose
	cmd := exec.Command("docker-compose", "-f", "../../docker-compose.yml", "up", "-d", "--build")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to start docker-compose: %v\n", err)
		os.Exit(1)
	}

	// Wait for services to be healthy
	if !waitForPostgres() {
		fmt.Println("PostgreSQL did not become healthy in time")
		shutdown()
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Shutdown docker-compose
	shutdown()

	os.Exit(code)
}

func shutdown() {
	cmd := exec.Command("docker-compose", "-f", "../../docker-compose.yml", "down", "-v")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to stop docker-compose: %v\n", err)
	}
}

func waitForPostgres() bool {
	for i := 0; i < 30; i++ {
		db, err := sql.Open("postgres", postgresDSN)
		if err == nil {
			defer db.Close()
			if err = db.Ping(); err == nil {
				return true
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func countLogsInDB(t *testing.T) int {
	db, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query log count: %v", err)
	}
	return count
}

func TestIngestionFlow(t *testing.T) {
	// Give consumer a moment to start up and connect
	time.Sleep(5 * time.Second)

	// 1. Initial state check
	initialCount := countLogsInDB(t)
	if initialCount != 0 {
		t.Fatalf("Expected initial log count to be 0, got %d", initialCount)
	}

	// 2. Ingest a batch of unique events
	batchSize := 100
	var ndjsonBody bytes.Buffer
	for i := 0; i < batchSize; i++ {
		eventID := uuid.NewString()
		logLine := fmt.Sprintf(`{"event_id": "%s", "message": "integration test event %d"}`, eventID, i)
		ndjsonBody.WriteString(logLine + "\n")
	}

	req, _ := http.NewRequest(http.MethodPost, ingestorURL, &ndjsonBody)
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-API-Key", "supersecretkey") // From init.sql

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send ingest request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected status 202 Accepted, got %d", resp.StatusCode)
	}

	// 3. Verify events are processed and stored
	var finalCount int
	// Retry logic to allow for processing time
	for i := 0; i < 10; i++ {
		finalCount = countLogsInDB(t)
		if finalCount == batchSize {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if finalCount != batchSize {
		t.Fatalf("Expected %d logs in DB after ingest, got %d", batchSize, finalCount)
	}

	// 4. Ingest the *same* batch again to test idempotency
	req, _ = http.NewRequest(http.MethodPost, ingestorURL, &ndjsonBody)
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-API-Key", "supersecretkey")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send second ingest request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected status 202 Accepted on second request, got %d", resp.StatusCode)
	}

	// 5. Verify no new rows were added
	time.Sleep(5 * time.Second) // Allow time for processing
	idempotentCount := countLogsInDB(t)
	if idempotentCount != batchSize {
		t.Fatalf("Idempotency test failed: expected count to remain %d, but got %d", batchSize, idempotentCount)
	}
}

