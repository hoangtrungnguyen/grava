package graph

import (
	"encoding/json"
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// LoadGraphFromDB loads the entire graph structure from the database
func LoadGraphFromDB(store dolt.Store) (*AdjacencyDAG, error) {
	dag := NewAdjacencyDAG(true) // Enable cache
	dag.store = store

	// Load all issues
	rows, err := store.Query("SELECT id, title, issue_type, status, priority, created_at, updated_at, await_type, await_id, ephemeral, metadata FROM issues WHERE status != 'tombstone'")
	if err != nil {
		return nil, fmt.Errorf("failed to query issues: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var node Node
		var prio int
		var awaitType, awaitID *string
		var metadataJSON []byte
		if err := rows.Scan(&node.ID, &node.Title, &node.Type, &node.Status, &prio, &node.CreatedAt, &node.UpdatedAt, &awaitType, &awaitID, &node.Ephemeral, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan issue: %w", err)
		}
		node.Priority = Priority(prio)
		if awaitType != nil {
			node.AwaitType = *awaitType
		}
		if awaitID != nil {
			node.AwaitID = *awaitID
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &node.Metadata); err != nil {
				// We log or ignore error? Description says "handles malformed JSON".
				// Let's ensure it doesn't crash.
				node.Metadata = make(map[string]interface{})
			}
		}
		if node.Metadata == nil {
			node.Metadata = make(map[string]interface{})
		}

		_ = dag.AddNode(&node)
	}

	// Load all dependencies
	rows, err = store.Query("SELECT from_id, to_id, type, metadata FROM dependencies")
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var edge Edge
		var depType string
		var metadataJSON []byte
		if err := rows.Scan(&edge.FromID, &edge.ToID, &depType, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}
		edge.Type = DependencyType(depType)

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &edge.Metadata); err != nil {
				edge.Metadata = make(map[string]interface{})
			}
		}
		if edge.Metadata == nil {
			edge.Metadata = make(map[string]interface{})
		}

		_ = dag.AddEdge(&edge)
	}

	return dag, nil
}
