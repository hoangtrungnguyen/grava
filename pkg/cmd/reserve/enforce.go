package reserve

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

	// Fetch active exclusive leases. When actor is empty (unconfigured identity),
	// check ALL leases — we can't distinguish own vs other.
	var args []interface{}
	q := "SELECT id, agent_id, path_pattern, expires_ts" +
		" FROM file_reservations" +
		" WHERE project_id = ?" +
		" AND `exclusive` = TRUE" +
		" AND released_ts IS NULL" +
		" AND expires_ts > NOW()"
	args = append(args, defaultProjectID)
	if actor != "" {
		q += ` AND agent_id != ?`
		args = append(args, actor)
	}
	rows, err := store.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("check staged conflicts: query: %w", err)
	}
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
// Supports filepath.Match glob syntax (e.g. "src/cmd/*.go") and additionally
// handles "**" as a recursive directory wildcard (matching zero or more path segments).
// Falls back to exact match if the pattern is invalid.
func matchPattern(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, path)
	}
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return pattern == path
	}
	return matched
}

// matchDoublestar handles patterns containing "**" by splitting on "**" and
// checking that the prefix matches the start of the path and the suffix matches
// the end, with any number of path segments in between.
func matchDoublestar(pattern, path string) bool {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0] // e.g. "src/cmd/" from "src/cmd/**/*.go"
	suffix := parts[1] // e.g. "/*.go" from "src/cmd/**/*.go"

	// Strip leading separator from suffix
	suffix = strings.TrimPrefix(suffix, "/")

	// Path must start with prefix
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	remainder := strings.TrimPrefix(path, prefix)

	// If no suffix, ** matches everything after prefix
	if suffix == "" {
		return true
	}

	// Try matching suffix against every possible tail of the remainder.
	// e.g. remainder="reserve/sub/enforce.go", suffix="*.go"
	// Try: "reserve/sub/enforce.go", "sub/enforce.go", "enforce.go"
	for {
		matched, err := filepath.Match(suffix, remainder)
		if err == nil && matched {
			return true
		}
		idx := strings.IndexByte(remainder, '/')
		if idx < 0 {
			break
		}
		remainder = remainder[idx+1:]
	}
	return false
}
