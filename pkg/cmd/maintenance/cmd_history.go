package maintenance

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
	"github.com/spf13/cobra"
)

// CmdAuditEntry is a single row from cmd_audit_log.
type CmdAuditEntry struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Actor     string    `json:"actor"`
	ArgsJSON  string    `json:"args,omitempty"`
	ExitCode  int       `json:"exit_code"`
	CreatedAt time.Time `json:"timestamp"`
}

// generateCmdAuditID returns a short ID like "cal-a1b2c3".
func generateCmdAuditID() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		n = big.NewInt(time.Now().UnixNano() % 999_999)
	}
	input := fmt.Sprintf("cal-%d-%d", time.Now().UnixNano(), n.Int64())
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("cal-%s", fmt.Sprintf("%x", hash)[:6])
}

// RecordCommand inserts a row into cmd_audit_log.
// argsJSON should be the JSON-encoded CLI arguments (os.Args[1:]).
// exitCode is 0 for success, 1 for failure.
// Errors are intentionally swallowed — command recording must never abort a successful operation.
func RecordCommand(ctx context.Context, store dolt.Store, command, actor, argsJSON string, exitCode int) {
	id := generateCmdAuditID()
	const q = `INSERT INTO cmd_audit_log (id, command, actor, args_json, exit_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = store.ExecContext(ctx, q, id, command, actor, argsJSON, exitCode, time.Now().UTC())
}

// QueryCmdHistory reads rows from cmd_audit_log with optional actor filter.
func QueryCmdHistory(ctx context.Context, store dolt.Store, actor string, limit int) ([]CmdAuditEntry, error) {
	if limit < 1 {
		limit = 50
	}
	var (
		q    string
		args []any
	)
	if actor != "" {
		q = `SELECT id, command, actor, COALESCE(args_json,''), exit_code, created_at
			FROM cmd_audit_log
			WHERE actor = ?
			ORDER BY created_at DESC
			LIMIT ?`
		args = []any{actor, limit}
	} else {
		q = `SELECT id, command, actor, COALESCE(args_json,''), exit_code, created_at
			FROM cmd_audit_log
			ORDER BY created_at DESC
			LIMIT ?`
		args = []any{limit}
	}

	rows, err := store.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("cmd_history: query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var result []CmdAuditEntry
	for rows.Next() {
		var e CmdAuditEntry
		if err := rows.Scan(&e.ID, &e.Command, &e.Actor, &e.ArgsJSON, &e.ExitCode, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("cmd_history: scan: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func newCmdHistoryCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		flagLimit int
		flagActor string
	)
	cmd := &cobra.Command{
		Use:   "cmd_history",
		Short: "Show the ledger of executed Grava commands",
		Long: `Display an ordered audit trail of previously executed Grava commands.

Examples:
  grava cmd_history
  grava cmd_history --limit 20
  grava cmd_history --actor agent-01
  grava cmd_history --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			entries, err := QueryCmdHistory(ctx, *d.Store, flagActor, flagLimit)
			if err != nil {
				return err
			}

			if *d.OutputJSON {
				if entries == nil {
					entries = []CmdAuditEntry{}
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"entries": entries,
				})
			}

			if len(entries) == 0 {
				cmd.Println("No command history found.")
				return nil
			}

			cmd.Printf("%-12s  %-30s  %-20s  %-4s  %s\n",
				"ID", "COMMAND", "ACTOR", "EXIT", "TIMESTAMP")
			for _, e := range entries {
				cmd.Printf("%-12s  %-30s  %-20s  %-4d  %s\n",
					e.ID, e.Command, e.Actor, e.ExitCode,
					e.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of entries to return")
	cmd.Flags().StringVar(&flagActor, "actor", "", "Filter entries by actor identity")
	return cmd
}
