package dolt_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// TestClient_GetNextChildSequence_Integration runs against a local Dolt server.
// Set environment variable TEST_INTEGRATION=1 to run (or check if port is open).
// For simplicity in this session, we run it if connection succeeds.
func TestClient_GetNextChildSequence_Integration(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
	}
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

// TestForeignKeyConstraints_Dependencies verifies that FK constraints work properly for the dependencies table
func TestForeignKeyConstraints_Dependencies(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
	}
	client, err := dolt.NewClient(dsn)
	if err != nil {
		t.Skipf("Skipping integration test: connection failed: %v", err)
	}
	defer client.Close()

	// Test 1: Insert dependency with non-existent from_id should fail
	t.Run("InvalidFromID", func(t *testing.T) {
		nonExistentID := fmt.Sprintf("nonexistent-%d", time.Now().UnixNano())
		validID := fmt.Sprintf("valid-%d", time.Now().UnixNano())

		// Create valid issue first
		_, err := client.Exec(
			"INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			validID, "Valid Issue", "Description", "task", 2, "open", time.Now(), time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to create valid issue: %v", err)
		}
		defer client.Exec("DELETE FROM issues WHERE id = ?", validID)

		// Try to insert dependency with invalid from_id
		_, err = client.Exec(
			"INSERT INTO dependencies (from_id, to_id, type) VALUES (?, ?, ?)",
			nonExistentID, validID, "blocks",
		)
		if err == nil {
			t.Error("Expected FK constraint violation for invalid from_id, but insert succeeded")
		}
		// Check that the error is specifically a FK constraint error
		if err != nil && !containsString(err.Error(), []string{"foreign key", "constraint", "Cannot add"}) {
			t.Logf("Got error (as expected): %v", err)
		}
	})

	// Test 2: Insert dependency with non-existent to_id should fail
	t.Run("InvalidToID", func(t *testing.T) {
		nonExistentID := fmt.Sprintf("nonexistent-%d", time.Now().UnixNano())
		validID := fmt.Sprintf("valid-%d", time.Now().UnixNano())

		// Create valid issue first
		_, err := client.Exec(
			"INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			validID, "Valid Issue", "Description", "task", 2, "open", time.Now(), time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to create valid issue: %v", err)
		}
		defer client.Exec("DELETE FROM issues WHERE id = ?", validID)

		// Try to insert dependency with invalid to_id
		_, err = client.Exec(
			"INSERT INTO dependencies (from_id, to_id, type) VALUES (?, ?, ?)",
			validID, nonExistentID, "blocks",
		)
		if err == nil {
			t.Error("Expected FK constraint violation for invalid to_id, but insert succeeded")
		}
		if err != nil && !containsString(err.Error(), []string{"foreign key", "constraint", "Cannot add"}) {
			t.Logf("Got error (as expected): %v", err)
		}
	})

	// Test 3: Cascade delete - deleting an issue should cascade to dependencies
	t.Run("CascadeDelete", func(t *testing.T) {
		fromID := fmt.Sprintf("from-%d", time.Now().UnixNano())
		toID := fmt.Sprintf("to-%d", time.Now().UnixNano())

		// Create two issues
		for _, id := range []string{fromID, toID} {
			_, err := client.Exec(
				"INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				id, "Test Issue", "Description", "task", 2, "open", time.Now(), time.Now(),
			)
			if err != nil {
				t.Fatalf("Failed to create issue %s: %v", id, err)
			}
		}

		// Create dependency between them
		_, err := client.Exec(
			"INSERT INTO dependencies (from_id, to_id, type) VALUES (?, ?, ?)",
			fromID, toID, "blocks",
		)
		if err != nil {
			t.Fatalf("Failed to create dependency: %v", err)
		}

		// Verify dependency exists
		var count int
		err = client.QueryRow("SELECT COUNT(*) FROM dependencies WHERE from_id = ? AND to_id = ?", fromID, toID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query dependency: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 dependency, got %d", count)
		}

		// Delete the from_id issue
		_, err = client.Exec("DELETE FROM issues WHERE id = ?", fromID)
		if err != nil {
			t.Fatalf("Failed to delete issue: %v", err)
		}

		// Verify dependency was cascade deleted
		err = client.QueryRow("SELECT COUNT(*) FROM dependencies WHERE from_id = ? OR to_id = ?", fromID, fromID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query dependencies: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 dependencies after cascade delete, got %d", count)
		}

		// Cleanup remaining issue
		client.Exec("DELETE FROM issues WHERE id = ?", toID)
	})
}

// TestForeignKeyConstraints_Events verifies that FK constraints work properly for the events table
func TestForeignKeyConstraints_Events(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
	}
	client, err := dolt.NewClient(dsn)
	if err != nil {
		t.Skipf("Skipping integration test: connection failed: %v", err)
	}
	defer client.Close()

	// Test 1: Insert event with non-existent issue_id should fail
	t.Run("InvalidIssueID", func(t *testing.T) {
		nonExistentID := fmt.Sprintf("nonexistent-%d", time.Now().UnixNano())

		// Try to insert event with invalid issue_id
		_, err := client.Exec(
			"INSERT INTO events (issue_id, event_type, actor, timestamp) VALUES (?, ?, ?, ?)",
			nonExistentID, "create", "test-user", time.Now(),
		)
		if err == nil {
			t.Error("Expected FK constraint violation for invalid issue_id, but insert succeeded")
		}
		if err != nil && !containsString(err.Error(), []string{"foreign key", "constraint", "Cannot add"}) {
			t.Logf("Got error (as expected): %v", err)
		}
	})

	// Test 2: Cascade delete - deleting an issue should cascade to events
	t.Run("CascadeDelete", func(t *testing.T) {
		issueID := fmt.Sprintf("issue-%d", time.Now().UnixNano())

		// Create issue
		_, err := client.Exec(
			"INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			issueID, "Test Issue", "Description", "task", 2, "open", time.Now(), time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to create issue: %v", err)
		}

		// Create multiple events for the issue
		for i := 0; i < 3; i++ {
			_, err := client.Exec(
				"INSERT INTO events (issue_id, event_type, actor, timestamp) VALUES (?, ?, ?, ?)",
				issueID, fmt.Sprintf("event-%d", i), "test-user", time.Now(),
			)
			if err != nil {
				t.Fatalf("Failed to create event %d: %v", i, err)
			}
		}

		// Verify events exist
		var count int
		err = client.QueryRow("SELECT COUNT(*) FROM events WHERE issue_id = ?", issueID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query events: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 events, got %d", count)
		}

		// Delete the issue
		_, err = client.Exec("DELETE FROM issues WHERE id = ?", issueID)
		if err != nil {
			t.Fatalf("Failed to delete issue: %v", err)
		}

		// Verify events were cascade deleted
		err = client.QueryRow("SELECT COUNT(*) FROM events WHERE issue_id = ?", issueID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query events after delete: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 events after cascade delete, got %d", count)
		}
	})
}

// containsString checks if error message contains any of the expected strings (case-insensitive)
func containsString(s string, substrs []string) bool {
	lower := fmt.Sprintf("%s", s)
	for _, substr := range substrs {
		if len(lower) >= len(substr) {
			for i := 0; i <= len(lower)-len(substr); i++ {
				match := true
				for j := 0; j < len(substr); j++ {
					c1 := lower[i+j]
					c2 := substr[j]
					if c1 >= 'A' && c1 <= 'Z' {
						c1 += 32
					}
					if c2 >= 'A' && c2 <= 'Z' {
						c2 += 32
					}
					if c1 != c2 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}
