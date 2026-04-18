// Package reserve implements the `grava reserve` command for declaring,
// listing, and releasing advisory file-path leases (FR-ECS-1a).
package reserve

import (
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // sha1 used for filename derivation, not cryptographic security
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/spf13/cobra"
)

const defaultProjectID = "default"
const defaultTTLMinutes = 30

// Reservation is the JSON-serialisable view of a file_reservations row.
type Reservation struct {
	ID               string        `json:"id"`
	ProjectID        string        `json:"project_id"`
	AgentID          string        `json:"agent_id"`
	PathPattern      string        `json:"path_pattern"`
	Exclusive        bool          `json:"exclusive"`
	Reason           string        `json:"reason,omitempty"`
	CreatedTS        time.Time     `json:"created_ts"`
	ExpiresTS        time.Time     `json:"expires_ts"`
	ReleasedTS       *time.Time    `json:"released_ts,omitempty"`
	RemainingSeconds int64         `json:"remaining_seconds,omitempty"` // server-side computed
}

// generateReservationID returns a short ID like "res-a1b2c3".
func generateReservationID() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		n = big.NewInt(time.Now().UnixNano() % 999_999)
	}
	input := fmt.Sprintf("%d-%d", time.Now().UnixNano(), n.Int64())
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("res-%s", fmt.Sprintf("%x", hash)[:6])
}

// DeclareParams holds the inputs for declaring a lease.
type DeclareParams struct {
	PathPattern string
	AgentID     string
	ProjectID   string
	Exclusive   bool
	Reason      string
	TTLMinutes  int
}

// DeclareResult is returned by DeclareReservation on success.
type DeclareResult struct {
	Reservation Reservation `json:"reservation"`
}

// DeclareReservation creates a new advisory file-path lease in the database.
// It validates the request, checks for conflicting exclusive leases from other
// agents using glob-based overlap detection, and inserts the row inside a
// transaction to prevent TOCTOU races. Missing fields in p are filled with
// defaults (AgentID="unknown", ProjectID="default", TTLMinutes=30).
// Returns FILE_RESERVATION_CONFLICT if an active exclusive lease from a
// different agent already covers the same path_pattern, or MISSING_REQUIRED_FIELD
// if PathPattern is empty.
func DeclareReservation(ctx context.Context, store dolt.Store, p DeclareParams) (DeclareResult, error) {
	if p.PathPattern == "" {
		return DeclareResult{}, gravaerrors.New("MISSING_REQUIRED_FIELD", "path pattern is required", nil)
	}
	if p.AgentID == "" {
		p.AgentID = "unknown"
	}
	if p.ProjectID == "" {
		p.ProjectID = defaultProjectID
	}
	if p.TTLMinutes < 1 {
		p.TTLMinutes = defaultTTLMinutes
	}

	// Check for conflicting exclusive lease from a different agent.
	// Always check — both exclusive AND shared requests must respect existing exclusive leases.
	conflict, conflictAgent, conflictExpires, err := findConflict(ctx, store, p.ProjectID, p.PathPattern, p.AgentID)
	if err != nil {
		return DeclareResult{}, fmt.Errorf("reserve: check conflict: %w", err)
	}
	if conflict {
		msg := fmt.Sprintf("path %q is exclusively reserved by %s until %s — release or wait",
			p.PathPattern, conflictAgent, conflictExpires.Format(time.RFC3339))
		return DeclareResult{}, gravaerrors.New("FILE_RESERVATION_CONFLICT", msg, nil)
	}

	id := generateReservationID()

	// Use SQL NOW() + INTERVAL for timestamps to stay consistent with Dolt's clock.
	// Wrap check+insert in a transaction to prevent TOCTOU race.
	tx, txErr := store.BeginTx(ctx, nil)
	if txErr != nil {
		return DeclareResult{}, fmt.Errorf("reserve: begin tx: %w", txErr)
	}
	defer tx.Rollback() //nolint:errcheck

	const q = "INSERT INTO file_reservations" +
		" (id, project_id, agent_id, path_pattern, `exclusive`, reason, created_ts, expires_ts)" +
		" VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW() + INTERVAL ? MINUTE)"
	if _, err := tx.ExecContext(ctx, q,
		id, p.ProjectID, p.AgentID, p.PathPattern, p.Exclusive, p.Reason, p.TTLMinutes,
	); err != nil {
		return DeclareResult{}, fmt.Errorf("reserve: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return DeclareResult{}, fmt.Errorf("reserve: commit: %w", err)
	}

	// Re-read the row to get server-computed timestamps.
	inserted, err := GetReservation(ctx, store, id)
	if err != nil {
		return DeclareResult{}, fmt.Errorf("reserve: read-back: %w", err)
	}
	return DeclareResult{Reservation: *inserted}, nil
}

// findConflict checks whether an active exclusive lease from a different agent
// overlaps with the requested path_pattern using glob matching.
// Returns (true, conflictAgent, expiresTS) if an overlapping lease is found.
func findConflict(ctx context.Context, store dolt.Store, projectID, pathPattern, requestingAgent string) (bool, string, time.Time, error) {
	// Fetch ALL active exclusive leases from other agents, then match with globs.
	// Exact SQL match is insufficient — patterns like src/**/*.go and src/cmd/*.go overlap.
	const q = "SELECT agent_id, path_pattern, expires_ts FROM file_reservations" +
		" WHERE project_id = ?" +
		" AND `exclusive` = TRUE" +
		" AND agent_id != ?" +
		" AND released_ts IS NULL" +
		" AND expires_ts > NOW()"
	rows, err := store.QueryContext(ctx, q, projectID, requestingAgent)
	if err != nil {
		return false, "", time.Time{}, fmt.Errorf("findConflict query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var agentID, existingPattern string
		var expiresTS time.Time
		if err := rows.Scan(&agentID, &existingPattern, &expiresTS); err != nil {
			return false, "", time.Time{}, fmt.Errorf("findConflict scan: %w", err)
		}
		// Check overlap in both directions: does existing cover requested, or vice versa?
		if patternsOverlap(existingPattern, pathPattern) {
			return true, agentID, expiresTS, nil
		}
	}
	return false, "", time.Time{}, rows.Err()
}

// patternsOverlap checks if two glob patterns could match the same file.
// Tests both directions: does pattern A match a representative of B, and vice versa.
// For exact patterns, also checks if either matches the other directly.
func patternsOverlap(a, b string) bool {
	// Direct match in either direction
	if matchPattern(a, b) || matchPattern(b, a) {
		return true
	}
	// If patterns are identical strings, they obviously overlap
	return a == b
}

// ListReservations returns all active (non-expired, non-released) reservations
// for the given project. It queries the file_reservations table, filtering out
// rows that have been released or whose TTL has expired, and orders results
// by creation time ascending. If projectID is empty, the default project is used.
func ListReservations(ctx context.Context, store dolt.Store, projectID string) ([]Reservation, error) {
	if projectID == "" {
		projectID = defaultProjectID
	}
	const q = "SELECT id, project_id, agent_id, path_pattern, `exclusive`, COALESCE(reason,''), created_ts, expires_ts," +
		" TIMESTAMPDIFF(SECOND, NOW(), expires_ts) AS remaining_seconds" +
		" FROM file_reservations" +
		" WHERE project_id = ?" +
		" AND released_ts IS NULL" +
		" AND expires_ts > NOW()" +
		" ORDER BY created_ts ASC"
	rows, err := store.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("reserve list: query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var result []Reservation
	for rows.Next() {
		var r Reservation
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.AgentID, &r.PathPattern,
			&r.Exclusive, &r.Reason, &r.CreatedTS, &r.ExpiresTS, &r.RemainingSeconds,
		); err != nil {
			return nil, fmt.Errorf("reserve list: scan: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ReleaseReservation marks a lease as released by setting released_ts to the
// current server time. Once released, the reservation no longer blocks other
// agents from acquiring exclusive leases on the same path pattern.
// Returns RESERVATION_NOT_FOUND if the reservation ID does not exist or is
// already released.
func ReleaseReservation(ctx context.Context, store dolt.Store, reservationID string) error {
	const q = `UPDATE file_reservations
		SET released_ts = NOW()
		WHERE id = ? AND released_ts IS NULL`
	result, err := store.ExecContext(ctx, q, reservationID)
	if err != nil {
		return fmt.Errorf("reserve release: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reserve release rows affected: %w", err)
	}
	if n == 0 {
		return gravaerrors.New("RESERVATION_NOT_FOUND",
			fmt.Sprintf("reservation %s not found or already released", reservationID), nil)
	}
	return nil
}

// artifactDir returns the path to the .grava/file_reservations/ directory.
func artifactDir(gravaDir string) string {
	return filepath.Join(gravaDir, "file_reservations")
}

// artifactPath returns the full path of the JSON artifact for a given path_pattern.
// Filename is the hex-encoded SHA-1 of path_pattern (collision-free for all practical
// path strings; SHA-1 is used only for deterministic filename derivation, not security).
func artifactPath(gravaDir, pathPattern string) string {
	h := sha1.Sum([]byte(pathPattern)) //nolint:gosec
	return filepath.Join(artifactDir(gravaDir), fmt.Sprintf("%x.json", h))
}

// WriteReservationArtifact writes or overwrites the JSON artifact for r under gravaDir.
// Creates the file_reservations/ directory if it does not exist.
func WriteReservationArtifact(gravaDir string, r Reservation) error {
	dir := artifactDir(gravaDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("reserve artifact: mkdir %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("reserve artifact: marshal: %w", err)
	}
	path := artifactPath(gravaDir, r.PathPattern)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("reserve artifact: write %s: %w", path, err)
	}
	return nil
}

// GetReservation fetches a single reservation by ID (including released rows).
func GetReservation(ctx context.Context, store dolt.Store, reservationID string) (*Reservation, error) {
	const q = "SELECT id, project_id, agent_id, path_pattern, `exclusive`, COALESCE(reason,'')," +
		" created_ts, expires_ts, released_ts" +
		" FROM file_reservations WHERE id = ?"
	row := store.QueryRowContext(ctx, q, reservationID)
	var r Reservation
	var releasedTS sql.NullTime
	if err := row.Scan(
		&r.ID, &r.ProjectID, &r.AgentID, &r.PathPattern,
		&r.Exclusive, &r.Reason, &r.CreatedTS, &r.ExpiresTS, &releasedTS,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gravaerrors.New("RESERVATION_NOT_FOUND",
				fmt.Sprintf("reservation %s not found", reservationID), nil)
		}
		return nil, fmt.Errorf("get reservation: scan: %w", err)
	}
	if releasedTS.Valid {
		r.ReleasedTS = &releasedTS.Time
	}
	return &r, nil
}

// AddCommands registers the reserve command tree on the root command.
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	var (
		flagExclusive bool
		flagTTL       int
		flagReason    string
		flagList      bool
		flagRelease   string
	)

	cmd := &cobra.Command{
		Use:   "reserve [path-pattern]",
		Short: "Declare, list, or release advisory file leases",
		Long: `Declare an advisory lease on file paths before modifying them.

Examples:
  grava reserve 'src/cmd/issues/*.go' --ttl=30 --exclusive  (--ttl is integer minutes, not a duration string)
  grava reserve --list
  grava reserve --release res-a1b2c3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			store := *d.Store
			agent := *d.Actor
			outputJSON := *d.OutputJSON

			// Resolve .grava/ directory for artifact writes (best-effort; non-fatal if missing).
			gravaDir, _ := grava.ResolveGravaDir()

			// --list
			if flagList {
				reservations, err := ListReservations(ctx, store, defaultProjectID)
				if err != nil {
					return err
				}
				if outputJSON {
					if reservations == nil {
						reservations = []Reservation{}
					}
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"reservations": reservations,
					})
				}
				if len(reservations) == 0 {
					cmd.Println("No active reservations.")
					return nil
				}
				cmd.Printf("%-12s  %-20s  %-10s  %-9s  %s\n",
					"ID", "AGENT", "EXCLUSIVE", "EXPIRES_IN", "PATH")
				for _, r := range reservations {
					exc := "shared"
					if r.Exclusive {
						exc = "exclusive"
					}
					eta := time.Duration(r.RemainingSeconds) * time.Second
					cmd.Printf("%-12s  %-20s  %-10s  %-9s  %s\n",
						r.ID, r.AgentID, exc, eta, r.PathPattern)
				}
				return nil
			}

			// --release <id>
			if flagRelease != "" {
				// Fetch reservation before releasing so we have path_pattern for artifact update.
				existing, fetchErr := GetReservation(ctx, store, flagRelease)
				if fetchErr != nil {
					return fetchErr
				}
				if err := ReleaseReservation(ctx, store, flagRelease); err != nil {
					return err
				}
				// Update the artifact to reflect the actual released_ts written by the DB.
				// Re-fetch after release so the artifact timestamp matches the DB's server-side NOW().
				if gravaDir != "" && existing != nil {
					if updated, refetchErr := GetReservation(ctx, store, flagRelease); refetchErr == nil {
						existing = updated
					}
					if werr := WriteReservationArtifact(gravaDir, *existing); werr != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not write reservation artifact: %v\n", werr)
					}
				}
				if outputJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
						"status": "released",
						"id":     flagRelease,
					})
				}
				cmd.Printf("✅ Released reservation %s\n", flagRelease)
				return nil
			}

			// Declare
			if len(args) == 0 {
				return gravaerrors.New("MISSING_REQUIRED_FIELD", "path pattern is required (or use --list / --release)", nil)
			}
			p := DeclareParams{
				PathPattern: args[0],
				AgentID:     agent,
				ProjectID:   defaultProjectID,
				Exclusive:   flagExclusive,
				Reason:      flagReason,
				TTLMinutes:  flagTTL,
			}
			result, err := DeclareReservation(ctx, store, p)
			if err != nil {
				return err
			}
			// Write Git-tracked artifact (best-effort; non-fatal if .grava/ is unavailable).
			if gravaDir != "" {
				if werr := WriteReservationArtifact(gravaDir, result.Reservation); werr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not write reservation artifact: %v\n", werr)
				}
			}
			if outputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			r := result.Reservation
			exc := "shared"
			if r.Exclusive {
				exc = "exclusive"
			}
			cmd.Printf("🔒 Reserved %s (%s) by %s until %s — ID: %s\n",
				r.PathPattern, exc, r.AgentID,
				r.ExpiresTS.Format("15:04:05 UTC"), r.ID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagExclusive, "exclusive", false, "Declare an exclusive (write) lease; blocks other agents")
	cmd.Flags().IntVar(&flagTTL, "ttl", defaultTTLMinutes, "Lease duration in minutes")
	cmd.Flags().StringVar(&flagReason, "reason", "", "Human-readable reason for the reservation")
	cmd.Flags().BoolVar(&flagList, "list", false, "List all active reservations")
	cmd.Flags().StringVar(&flagRelease, "release", "", "Release a reservation by ID")

	root.AddCommand(cmd)
}
