package cmd

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/idgen"
)

// setupBenchmarkDB creates a test database connection for benchmarks
func setupBenchmarkDB(b *testing.B) *dolt.Client {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
	}

	client, err := dolt.NewClient(dsn)
	if err != nil {
		b.Skipf("Skipping benchmark: failed to connect to database: %v", err)
	}

	return client
}

// BenchmarkCreateBaseIssue measures the performance of creating top-level issues
func BenchmarkCreateBaseIssue(b *testing.B) {
	client := setupBenchmarkDB(b)
	defer client.Close()

	generator := idgen.NewStandardGenerator(client)

	// Clean up benchmark data before starting
	_, _ = client.Exec("DELETE FROM issues WHERE id LIKE 'base%'")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use sequential IDs to avoid hash collisions during benchmarking
		id := fmt.Sprintf("base-%d-%d", time.Now().UnixNano(), i)

		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at)
                  VALUES (?, ?, ?, ?, ?, 'open', ?, ?)`

		_, err := client.Exec(query, id, "Benchmark Issue", "Benchmark Description", "task", 2, time.Now(), time.Now())
		if err != nil {
			b.Fatalf("Failed to insert issue %d: %v", i, err)
		}
	}
	b.StopTimer()

	// Cleanup
	_, _ = client.Exec("DELETE FROM issues WHERE id LIKE 'base%'")

	// Note: For reference, the real ID generator is used in BenchmarkBulkInsert1000
	_ = generator
}

// BenchmarkCreateSubtask measures the performance of creating hierarchical subtasks
func BenchmarkCreateSubtask(b *testing.B) {
	client := setupBenchmarkDB(b)
	defer client.Close()

	generator := idgen.NewStandardGenerator(client)

	// Create a parent issue first
	parentID := fmt.Sprintf("bench-p-%d", time.Now().Unix()%1000000)
	query := `INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at)
              VALUES (?, ?, ?, ?, ?, 'open', ?, ?)`
	_, err := client.Exec(query, parentID, "Parent Issue", "Parent Description", "epic", 1, time.Now(), time.Now())
	if err != nil {
		b.Fatalf("Failed to create parent issue: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Generate child ID
		childID, err := generator.GenerateChildID(parentID)
		if err != nil {
			b.Fatalf("Failed to generate child ID %d: %v", i, err)
		}

		_, err = client.Exec(query, childID, fmt.Sprintf("Subtask %d", i), "Subtask Description", "task", 2, time.Now(), time.Now())
		if err != nil {
			b.Fatalf("Failed to insert subtask %d: %v", i, err)
		}
	}
	b.StopTimer()

	// Cleanup
	_, _ = client.Exec("DELETE FROM issues WHERE id = ? OR id LIKE ?", parentID, parentID+".%")
}

// BenchmarkBulkInsert1000 measures bulk insert performance (1000 items)
// This benchmark fulfills the requirement: "Benchmark script inserting 1000 items"
func BenchmarkBulkInsert1000(b *testing.B) {
	client := setupBenchmarkDB(b)
	defer client.Close()

	generator := idgen.NewStandardGenerator(client)

	for n := 0; n < b.N; n++ {
		// Use a temporary parent for organizing our test data
		batchParent := fmt.Sprintf("bulk-%d", time.Now().Unix()%1000000)
		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at)
                  VALUES (?, ?, ?, ?, ?, 'open', ?, ?)`

		// Create parent
		_, err := client.Exec(query, batchParent, "Bulk Parent", "Parent for bulk test", "epic", 2, time.Now(), time.Now())
		if err != nil {
			b.Fatalf("Failed to create batch parent: %v", err)
		}

		b.StartTimer()
		// Insert 1000 items as subtasks
		for i := 0; i < 1000; i++ {
			childID, err := generator.GenerateChildID(batchParent)
			if err != nil {
				b.Fatalf("Failed to generate child ID for item %d: %v", i, err)
			}

			_, err = client.Exec(query, childID, fmt.Sprintf("Bulk Issue %d", i), "Bulk Description", "task", 2, time.Now(), time.Now())
			if err != nil {
				b.Fatalf("Failed to insert issue %d in iteration %d: %v", i, n, err)
			}
		}
		b.StopTimer()

		// Cleanup after each iteration
		_, _ = client.Exec("DELETE FROM issues WHERE id = ? OR id LIKE ?", batchParent, batchParent+".%")
	}
}

// BenchmarkMixedWorkload measures realistic mixed operations
func BenchmarkMixedWorkload(b *testing.B) {
	client := setupBenchmarkDB(b)
	defer client.Close()

	generator := idgen.NewStandardGenerator(client)

	for n := 0; n < b.N; n++ {
		b.StartTimer()

		// Create 10 parent issues
		parentIDs := make([]string, 10)
		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at)
                  VALUES (?, ?, ?, ?, ?, 'open', ?, ?)`

		for i := 0; i < 10; i++ {
			parentID := fmt.Sprintf("mix-p%d-%d", i, time.Now().Unix()%1000000)
			parentIDs[i] = parentID

			_, err := client.Exec(query, parentID, fmt.Sprintf("Parent %d", i), "Parent Description", "epic", 1, time.Now(), time.Now())
			if err != nil {
				b.Fatalf("Failed to insert parent %d: %v", i, err)
			}
		}

		// Create 5 subtasks for each parent (50 subtasks total)
		for _, parentID := range parentIDs {
			for j := 0; j < 5; j++ {
				childID, err := generator.GenerateChildID(parentID)
				if err != nil {
					b.Fatalf("Failed to generate child ID: %v", err)
				}

				_, err = client.Exec(query, childID, fmt.Sprintf("Subtask %d", j), "Subtask Description", "task", 2, time.Now(), time.Now())
				if err != nil {
					b.Fatalf("Failed to insert subtask: %v", err)
				}
			}
		}

		b.StopTimer()

		// Cleanup
		for _, parentID := range parentIDs {
			_, _ = client.Exec("DELETE FROM issues WHERE id = ? OR id LIKE ?", parentID, parentID+".%")
		}
	}
}

// BenchmarkSequentialInserts measures performance of sequential inserts with varying priorities
func BenchmarkSequentialInserts(b *testing.B) {
	client := setupBenchmarkDB(b)
	defer client.Close()

	priorities := []int{0, 1, 2, 3, 4}                                      // critical, high, medium, low, backlog
	types := []string{"task", "bug", "epic", "feature", "chore", "message"} // All valid types from schema

	// Clean up any existing test data
	_, _ = client.Exec("DELETE FROM issues WHERE id LIKE 'seq%'")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("seq-%d-%d", time.Now().UnixNano(), i)

		priority := priorities[i%len(priorities)]
		issueType := types[i%len(types)]

		query := `INSERT INTO issues (id, title, description, issue_type, priority, status, created_at, updated_at)
                  VALUES (?, ?, ?, ?, ?, 'open', ?, ?)`

		_, err := client.Exec(query, id, fmt.Sprintf("Issue %d", i), "Description", issueType, priority, time.Now(), time.Now())
		if err != nil {
			b.Fatalf("Failed to insert issue %d: %v", i, err)
		}
	}
	b.StopTimer()

	// Cleanup
	_, _ = client.Exec("DELETE FROM issues WHERE id LIKE 'seq%'")
}
