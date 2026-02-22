package graph

import (
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// LoadGraphFromDB loads the entire graph structure from the database
func LoadGraphFromDB(store dolt.Store) (*AdjacencyDAG, error) {
	dag := NewAdjacencyDAG(true) // Enable cache

	// Load all issues
	rows, err := store.Query("SELECT id, title, status, priority, created_at, await_type, await_id FROM issues")
	if err != nil {
		return nil, fmt.Errorf("failed to query issues: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var node Node
		var prio int
		var awaitType, awaitID *string
		if err := rows.Scan(&node.ID, &node.Title, &node.Status, &prio, &node.CreatedAt, &awaitType, &awaitID); err != nil {
			return nil, fmt.Errorf("failed to scan issue: %w", err)
		}
		node.Priority = Priority(prio)
		if awaitType != nil {
			node.AwaitType = *awaitType
		}
		if awaitID != nil {
			node.AwaitID = *awaitID
		}
		dag.AddNode(&node)
	}

	// Load all dependencies
	rows, err = store.Query("SELECT from_id, to_id, type FROM dependencies")
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var edge Edge
		var depType string
		if err := rows.Scan(&edge.FromID, &edge.ToID, &depType); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}
		edge.Type = DependencyType(depType)
		dag.AddEdge(&edge)
	}

	return dag, nil
}
