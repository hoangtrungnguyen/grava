package reserve

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// Conflict describes a staged path that overlaps with an active exclusive lease.
type Conflict struct {
	Path          string    `json:"path"`
	ReservationID string    `json:"reservation_id"`
	AgentID       string    `json:"agent_id"`
	PathPattern   string    `json:"path_pattern"`
	ExpiresTS     time.Time `json:"expires_ts"`
}

// CheckStagedConflicts checks whether any of the given staged file paths
// overlap with active exclusive leases held by a different agent.
// Returns a slice of conflicts (empty if none). Only active, exclusive,
// non-expired, non-released leases from OTHER agents are considered.
func CheckStagedConflicts(ctx context.Context, store dolt.Store, stagedPaths []string, actor string) ([]Conflict, error) {
	if len(stagedPaths) == 0 {
		return nil, nil
	}

	// Fetch all active exclusive leases from other agents.
	const q = `SELECT id, agent_id, path_pattern, expires_ts
		FROM file_reservations
		WHERE project_id = ?
		  AND exclusive = TRUE
		  AND released_ts IS NULL
		  AND expires_ts > NOW()
		  AND agent_id != ?`
	rows, err := store.QueryContext(ctx, q, defaultProjectID, actor)
	if err != nil {
		return nil, fmt.Errorf("check staged conflicts: query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	type lease struct {
		id          string
		agentID     string
		pathPattern string
		expiresTS   time.Time
	}
	var leases []lease
	for rows.Next() {
		var l lease
		if err := rows.Scan(&l.id, &l.agentID, &l.pathPattern, &l.expiresTS); err != nil {
			return nil, fmt.Errorf("check staged conflicts: scan: %w", err)
		}
		leases = append(leases, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("check staged conflicts: rows: %w", err)
	}

	if len(leases) == 0 {
		return nil, nil
	}

	// Match each staged path against each lease pattern.
	var conflicts []Conflict
	for _, path := range stagedPaths {
		for _, l := range leases {
			if matchPattern(l.pathPattern, path) {
				conflicts = append(conflicts, Conflict{
					Path:          path,
					ReservationID: l.id,
					AgentID:       l.agentID,
					PathPattern:   l.pathPattern,
					ExpiresTS:     l.expiresTS,
				})
				break // one conflict per path is enough
			}
		}
	}
	return conflicts, nil
}

// matchPattern checks if a file path matches a reservation pattern.
// Supports filepath.Match glob syntax (e.g. "src/cmd/*.go").
// Falls back to exact match if the pattern is invalid.
func matchPattern(pattern, path string) bool {
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		// Invalid pattern — fall back to exact string match
		return pattern == path
	}
	return matched
}
