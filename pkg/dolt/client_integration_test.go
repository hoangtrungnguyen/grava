package dolt_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// TestClient_GetNextChildSequence_Integration runs against a local Dolt server.
// Set environment variable TEST_INTEGRATION=1 to run (or check if port is open).
// For simplicity in this session, we run it if connection succeeds.
func TestClient_GetNextChildSequence_Integration(t *testing.T) {
	dsn := "root@tcp(127.0.0.1:3306)/dolt" // Using 'dolt' database as verified earlier
	client, err := dolt.NewClient(dsn)
	if err != nil {
		t.Skipf("Skipping integration test: connection failed: %v", err)
	}
	defer client.Close()

	parentID := fmt.Sprintf("test-parent-%d", time.Now().UnixNano())

	// 1. Sequential Test
	seq, err := client.GetNextChildSequence(parentID)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if seq != 1 {
		t.Errorf("Expected 1, got %d", seq)
	}

	seq, err = client.GetNextChildSequence(parentID)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if seq != 2 {
		t.Errorf("Expected 2, got %d", seq)
	}

	// 2. Concurrency Test
	concurrency := 10
	results := make(chan int, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			s, err := client.GetNextChildSequence(parentID)
			if err != nil {
				errors <- err
				return
			}
			results <- s
		}()
	}

	seen := make(map[int]bool)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-errors:
			t.Fatalf("Concurrent error: %v", err)
		case val := <-results:
			if seen[val] {
				t.Fatalf("Duplicate sequence returned: %d", val)
			}
			seen[val] = true
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out waiting for concurrent results")
		}
	}
}
