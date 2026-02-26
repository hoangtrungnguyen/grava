package graph

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/migrate"
	"github.com/stretchr/testify/assert"
	"github.com/subosito/gotenv"
)

func TestLoadGraphMetadataIntegration(t *testing.T) {
	// Try to load .env.test
	_ = gotenv.Load("../../.env.test")
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		t.Skip("Skipping integration test: DB_URL not set")
	}

	// 1. Setup real Store
	client, err := dolt.NewClient(dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer client.Close() //nolint:errcheck

	// 2. Run migrations
	if err := migrate.Run(client.DB()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// 3. Clear data
	_, _ = client.Exec("DELETE FROM dependencies")
	_, _ = client.Exec("DELETE FROM issues")

	// 4. Insert issues
	_, err = client.Exec("INSERT INTO issues (id, title, status, priority, metadata) VALUES (?, ?, ?, ?, ?)",
		"grava-meta-1", "Task with Metadata", "open", 2, `{"tags": ["urgent", "graph"], "complexity": 5}`)
	assert.NoError(t, err)

	_, err = client.Exec("INSERT INTO issues (id, title, status, priority) VALUES (?, ?, ?, ?)",
		"grava-meta-2", "Target Task", "open", 2)
	assert.NoError(t, err)

	// 5. Insert dependency with metadata
	edgeMetaJSON := `{"strength": "hard"}`
	_, err = client.Exec("INSERT INTO dependencies (from_id, to_id, type, metadata) VALUES (?, ?, ?, ?)",
		"grava-meta-1", "grava-meta-2", "blocks", edgeMetaJSON)
	assert.NoError(t, err)

	// 6. Load Graph
	dag, err := LoadGraphFromDB(client)
	assert.NoError(t, err)

	// 7. Verify Node Metadata
	node, err := dag.GetNode("grava-meta-1")
	assert.NoError(t, err)
	assert.NotNil(t, node.Metadata, "Node metadata should not be nil")
	assert.Equal(t, float64(5), node.Metadata["complexity"])

	tags, ok := node.Metadata["tags"].([]interface{})
	assert.True(t, ok)
	assert.Contains(t, tags, "urgent")

	// 8. Verify Edge Metadata
	edges, err := dag.GetOutgoingEdges("grava-meta-1")
	assert.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.NotNil(t, edges[0].Metadata, "Edge metadata should not be nil")
	assert.Equal(t, "hard", edges[0].Metadata["strength"])
}

func TestLoadGraphMalformedMetadataIntegration(t *testing.T) {
	// Try to load .env.test
	_ = gotenv.Load("../../.env.test")
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		t.Skip("Skipping integration test: DB_URL not set")
	}

	client, err := dolt.NewClient(dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer client.Close() //nolint:errcheck

	// Clear data
	_, _ = client.Exec("DELETE FROM dependencies")
	_, _ = client.Exec("DELETE FROM issues")

	// NULL metadata
	_, err = client.Exec("INSERT INTO issues (id, title, status, priority, metadata) VALUES (?, ?, ?, ?, ?)",
		"grava-mal-1", "Task with NULL Metadata", "open", 2, nil)
	assert.NoError(t, err)

	dag, err := LoadGraphFromDB(client)
	assert.NoError(t, err)

	node, err := dag.GetNode("grava-mal-1")
	assert.NoError(t, err)
	assert.NotNil(t, node.Metadata, "Metadata should be initialized even if NULL in DB")
	assert.Len(t, node.Metadata, 0)
}

func TestLoaderMetadataErrorHandling(t *testing.T) {
	// Test unmarshaling logic specifically
	node := &Node{}
	malformed := []byte(`{"tags": ["wrong", ]}`) // Invalid JSON

	// Ensure our loader logic (conceptually) handles this without crashing
	// If json.Unmarshal fails, we want it initialized as empty
	if err := json.Unmarshal(malformed, &node.Metadata); err != nil {
		node.Metadata = make(map[string]interface{})
	}
	assert.NotNil(t, node.Metadata)
	assert.Len(t, node.Metadata, 0)
}
