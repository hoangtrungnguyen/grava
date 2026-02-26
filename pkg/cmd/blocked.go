package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

var (
	blockedDepth int
)

// blockedCmd represents the blocked command
var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show tasks that are currently blocked",
	Long: `Blocked lists all open issues that cannot be started because they depend on other open issues or gates.

Use --depth to see transitive blockers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return fmt.Errorf("failed to load graph: %w", err)
		}

		// Find blocked nodes
		blockedResults := []blockedInfo{}

		for _, node := range dag.GetAllNodes() {
			if node.Status != graph.StatusOpen {
				continue
			}

			// Check if blocked
			blockers, _ := dag.GetTransitiveBlockers(node.ID, blockedDepth)

			// Check if blocked by gates
			gateBlocked := false
			if node.AwaitType != "" {
				ge := graph.NewDefaultGateEvaluator()
				open, _ := ge.IsGateOpen(node)
				if !open {
					gateBlocked = true
				}
			}

			if len(blockers) > 0 || gateBlocked {
				blockedResults = append(blockedResults, blockedInfo{
					ID:          node.ID,
					Title:       node.Title,
					Blockers:    blockers,
					GateBlocked: gateBlocked,
					AwaitType:   node.AwaitType,
					Ephemeral:   node.Ephemeral,
				})
			}
		}

		if outputJSON {
			b, _ := json.MarshalIndent(blockedResults, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTitle\tBlocked By\tGate") //nolint:errcheck

		for _, info := range blockedResults {
			blockerStr := "-"
			if len(info.Blockers) > 0 {
				blockerStr = fmt.Sprintf("%v", info.Blockers)
			}
			gateStr := "-"
			if info.GateBlocked {
				gateStr = info.AwaitType
			}

			title := info.Title
			if info.Ephemeral {
				title = "👻 " + title
			}
			if len(title) > 40 {
				title = title[:37] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", //nolint:errcheck
				info.ID,
				title,
				blockerStr,
				gateStr,
			)
		}
		w.Flush() //nolint:errcheck

		if len(blockedResults) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No blocked tasks found.") //nolint:errcheck
		}

		return nil
	},
}

type blockedInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Blockers    []string `json:"blockers"`
	GateBlocked bool     `json:"gate_blocked"`
	AwaitType   string   `json:"await_type,omitempty"`
	Ephemeral   bool     `json:"ephemeral"`
}

func init() {
	rootCmd.AddCommand(blockedCmd)
	blockedCmd.Flags().IntVarP(&blockedDepth, "depth", "d", 1, "Depth of transitive blockers to show")
}
