package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

var (
	readyLimit    int
	readyPriority int
	showInherited bool
)

// readyCmd represents the ready command
var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show tasks that are ready to be worked on",
	Long: `Ready computes tasks that are not blocked by any open dependencies or gates.
Tasks are sorted by their effective priority (highest first) and age.

Priority levels:
  0 = critical
  1 = high
  2 = medium (default for creation)
  3 = low
  4 = backlog`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dag, err := graph.LoadGraphFromDB(Store)
		if err != nil {
			return fmt.Errorf("failed to load graph: %w", err)
		}

		engine := graph.NewReadyEngine(dag, graph.DefaultReadyEngineConfig())
		tasks, err := engine.ComputeReady(readyLimit)
		if err != nil {
			return fmt.Errorf("failed to compute ready tasks: %w", err)
		}

		// Filter by priority if requested
		if readyPriority != -1 {
			filtered := []*graph.ReadyTask{}
			for _, t := range tasks {
				if int(t.EffectivePriority) == readyPriority {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}

		if outputJSON {
			b, err := json.MarshalIndent(tasks, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTitle\tPriority\tAge\tStatus")

		for _, task := range tasks {
			prioStr := fmt.Sprintf("%d", task.EffectivePriority)
			if task.PriorityBoosted && showInherited {
				prioStr = fmt.Sprintf("%d*", task.EffectivePriority)
			}

			title := task.Node.Title
			if task.Node.Ephemeral {
				title = "👻 " + title
			}
			if len(title) > 50 {
				title = title[:47] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				task.Node.ID,
				title,
				prioStr,
				formatAge(task.Age),
				task.Node.Status,
			)
		}
		w.Flush() //nolint:errcheck

		if len(tasks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No ready tasks found.")
		}

		return nil
	},
}

func formatAge(d time.Duration) string {
	if d < 24*time.Hour {
		return d.Round(time.Minute).String()
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

func init() {
	rootCmd.AddCommand(readyCmd)
	readyCmd.Flags().IntVarP(&readyLimit, "limit", "l", 20, "Limit number of results")
	readyCmd.Flags().IntVarP(&readyPriority, "priority", "p", -1, "Filter by priority level")
	readyCmd.Flags().BoolVar(&showInherited, "show-inherited", false, "Show if priority was inherited or boosted (indicated by *)")
}
