// Package reserve implements the `grava reserve` command for declaring,
// listing, and releasing advisory file-path leases (FR-ECS-1a).
package reserve

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

const defaultProjectID = "default"
const defaultTTLMinutes = 30

// Reservation is the JSON-serialisable view of a file_reservations row.
type Reservation struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	AgentID     string     `json:"agent_id"`
	PathPattern string     `json:"path_pattern"`
	Exclusive   bool       `json:"exclusive"`
	Reason      string     `json:"reason,omitempty"`
	CreatedTS   time.Time  `json:"created_ts"`
	ExpiresTS   time.Time  `json:"expires_ts"`
	ReleasedTS  *time.Time `json:"released_ts,omitempty"`
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

// DeclareReservation creates a new file lease in the DB.
// Returns FILE_RESERVATION_CONFLICT if an active exclusive lease from a
// different agent already covers the same path_pattern.
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
	if p.TTLMinutes <= 0 {
		p.TTLMinutes = defaultTTLMinutes
	}

	// Check for conflicting exclusive lease from a different agent.
	if p.Exclusive {
		conflict, conflictAgent, conflictExpires, err := findConflict(ctx, store, p.ProjectID, p.PathPattern, p.AgentID)
		if err != nil {
			return DeclareResult{}, fmt.Errorf("reserve: check conflict: %w", err)
		}
		if conflict {
			msg := fmt.Sprintf("path %q is exclusively reserved by %s until %s — release or wait",
				p.PathPattern, conflictAgent, conflictExpires.Format(time.RFC3339))
			return DeclareResult{}, gravaerrors.New("FILE_RESERVATION_CONFLICT", msg, nil)
		}
	}

	id := generateReservationID()
	now := time.Now().UTC()
	expires := now.Add(time.Duration(p.TTLMinutes) * time.Minute)

	const q = `INSERT INTO file_reservations
		(id, project_id, agent_id, path_pattern, exclusive, reason, created_ts, expires_ts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := store.ExecContext(ctx, q,
		id, p.ProjectID, p.AgentID, p.PathPattern, p.Exclusive, p.Reason, now, expires,
	); err != nil {
		return DeclareResult{}, fmt.Errorf("reserve: insert: %w", err)
	}

	r := Reservation{
		ID:          id,
		ProjectID:   p.ProjectID,
		AgentID:     p.AgentID,
		PathPattern: p.PathPattern,
		Exclusive:   p.Exclusive,
		Reason:      p.Reason,
		CreatedTS:   now,
		ExpiresTS:   expires,
	}
	return DeclareResult{Reservation: r}, nil
}

// findConflict checks whether an active exclusive lease from a different agent
// exists for the given path_pattern. Returns (true, conflictAgent, expiresTS) if found.
func findConflict(ctx context.Context, store dolt.Store, projectID, pathPattern, requestingAgent string) (bool, string, time.Time, error) {
	const q = `SELECT agent_id, expires_ts FROM file_reservations
		WHERE project_id = ?
		  AND path_pattern = ?
		  AND exclusive = TRUE
		  AND agent_id != ?
		  AND released_ts IS NULL
		  AND expires_ts > NOW()
		LIMIT 1`
	row := store.QueryRowContext(ctx, q, projectID, pathPattern, requestingAgent)
	var agentID string
	var expiresTS time.Time
	if err := row.Scan(&agentID, &expiresTS); err != nil {
		// sql.ErrNoRows means no conflict.
		return false, "", time.Time{}, nil
	}
	return true, agentID, expiresTS, nil
}

// ListReservations returns all active (non-expired, non-released) reservations.
func ListReservations(ctx context.Context, store dolt.Store, projectID string) ([]Reservation, error) {
	if projectID == "" {
		projectID = defaultProjectID
	}
	const q = `SELECT id, project_id, agent_id, path_pattern, exclusive, COALESCE(reason,''), created_ts, expires_ts
		FROM file_reservations
		WHERE project_id = ?
		  AND released_ts IS NULL
		  AND expires_ts > NOW()
		ORDER BY created_ts ASC`
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
			&r.Exclusive, &r.Reason, &r.CreatedTS, &r.ExpiresTS,
		); err != nil {
			return nil, fmt.Errorf("reserve list: scan: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ReleaseReservation sets released_ts on the given reservation row.
// Returns ISSUE_NOT_FOUND if the reservation ID does not exist or is already released/expired.
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
  grava reserve 'src/cmd/issues/*.go' --ttl=30 --exclusive
  grava reserve --list
  grava reserve --release res-a1b2c3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store := *d.Store
			agent := *d.Actor
			outputJSON := *d.OutputJSON

			// --list
			if flagList {
				reservations, err := ListReservations(ctx, store, defaultProjectID)
				if err != nil {
					return err
				}
				if outputJSON {
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
					eta := time.Until(r.ExpiresTS).Round(time.Second)
					cmd.Printf("%-12s  %-20s  %-10s  %-9s  %s\n",
						r.ID, r.AgentID, exc, eta, r.PathPattern)
				}
				return nil
			}

			// --release <id>
			if flagRelease != "" {
				if err := ReleaseReservation(ctx, store, flagRelease); err != nil {
					return err
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
