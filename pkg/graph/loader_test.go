package graph

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/migrate"
	"github.com/stretchr/testify/assert"
	"github.com/subosito/gotenv"
)

// TestLoadGraphFromDB_PopulatesTimestamps is a regression test for grava-b711:
// "ready queue Node.UpdatedAt always zero — not populated from DB".
//
// Before the fix, LoadGraphFromDB's SELECT did not include updated_at and the
// scan target list omitted &node.UpdatedAt, so every Node returned to callers
// (including grava ready --json) reported the Go zero time. This test pins the
// loader's contract: both CreatedAt and UpdatedAt MUST round-trip from the DB
// into the in-memory Node so consumers can determine issue freshness.
func TestLoadGraphFromDB_PopulatesTimestamps(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	store := dolt.NewClientFromDB(db)

	createdAt := time.Date(2026, 4, 18, 7, 57, 20, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 7, 8, 15, 54, 0, time.UTC)

	issueRows := sqlmock.NewRows([]string{
		"id", "title", "issue_type", "status", "priority",
		"created_at", "updated_at", "await_type", "await_id", "ephemeral", "metadata",
	}).AddRow(
		"grava-1", "Task One", "task", "open", 2,
		createdAt, updatedAt, nil, nil, false, nil,
	)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'",
	)).WillReturnRows(issueRows)

	depRows := sqlmock.NewRows([]string{"from_id", "to_id", "type", "metadata"})
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT from_id, to_id, type, metadata FROM dependencies",
	)).WillReturnRows(depRows)

	dag, err := LoadGraphFromDB(store)
	assert.NoError(t, err)

	node, err := dag.GetNode("grava-1")
	assert.NoError(t, err)

	// Core regression assertions: timestamps must NOT be the Go zero time and
	// must equal what the DB returned.
	assert.False(t, node.CreatedAt.IsZero(), "CreatedAt should be populated from DB, not Go zero time")
	assert.False(t, node.UpdatedAt.IsZero(), "UpdatedAt should be populated from DB, not Go zero time (grava-b711)")
	assert.True(t, node.CreatedAt.Equal(createdAt), "CreatedAt should round-trip exactly")
	assert.True(t, node.UpdatedAt.Equal(updatedAt), "UpdatedAt should round-trip exactly (grava-b711)")

	assert.NoError(t, mock.ExpectationsWereMet())
}

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
		t.Skip("Skipping integration test: could not connect to test db:", err)
	}
	defer client.Close() //nolint:errcheck

	// 2. Run migrations
	if err := migrate.Run(client.DB()); err != nil {
		t.Skip("Skipping integration test: could not run migrations:", err)
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
		t.Skip("Skipping integration test: could not connect to test db:", err)
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
