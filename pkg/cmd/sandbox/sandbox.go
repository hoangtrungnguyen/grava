package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/spf13/cobra"
)

// AddCommands registers the sandbox command tree with the root cobra.Command.
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	root.AddCommand(newSandboxCmd(d))
}

func newSandboxCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		flagScenario string
		flagAll      bool
		flagEpic     int
	)

	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Run integration validation scenarios",
		Long: `Execute sandbox validation scenarios to confirm system behaviour under
realistic conditions.

Examples:
  grava sandbox run --scenario=TS-01
  grava sandbox run --all
  grava sandbox run --epic=3`,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Execute one or more sandbox scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store := *d.Store
			outputJSON := *d.OutputJSON

			var toRun []Scenario

			switch {
			case flagScenario != "":
				s, ok := Find(strings.ToUpper(flagScenario))
				if !ok {
					return fmt.Errorf("sandbox: unknown scenario %q — valid IDs: %s",
						flagScenario, scenarioIDs())
				}
				toRun = []Scenario{*s}

			case flagEpic > 0:
				for _, s := range All() {
					if s.EpicGate <= flagEpic {
						toRun = append(toRun, s)
					}
				}
				if len(toRun) == 0 {
					return fmt.Errorf("sandbox: no scenarios gated at or below epic %d", flagEpic)
				}

			case flagAll:
				toRun = All()

			default:
				return fmt.Errorf("sandbox run: specify --scenario=<id>, --all, or --epic=<n>")
			}

			var results []Result
			anyFail := false

			for _, s := range toRun {
				r := Run(ctx, store, s)
				results = append(results, r)
				if r.Status != "pass" {
					anyFail = true
				}
			}

			if outputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"scenarios": len(results),
					"results":   results,
				})
			}

			// Human-readable output.
			for _, r := range results {
				icon := "✅"
				if r.Status != "pass" {
					icon = "❌"
				}
				cmd.Printf("%s %s (%dms)\n", icon, r.Scenario, r.DurationMs)
				for _, d := range r.Details {
					cmd.Printf("   • %s\n", d)
				}
				if r.Error != "" {
					cmd.Printf("   ERROR: %s\n", r.Error)
				}
			}

			if anyFail {
				return fmt.Errorf("sandbox: %d scenario(s) failed", countFailed(results))
			}
			cmd.Printf("\n✅ All %d scenario(s) passed.\n", len(results))
			return nil
		},
	}

	runCmd.Flags().StringVar(&flagScenario, "scenario", "", "Run a specific scenario by ID (e.g. TS-01)")
	runCmd.Flags().BoolVar(&flagAll, "all", false, "Run all registered scenarios")
	runCmd.Flags().IntVar(&flagEpic, "epic", 0, "Run scenarios gated at or below this epic number")

	cmd.AddCommand(runCmd)
	return cmd
}

func scenarioIDs() string {
	ids := make([]string, 0, len(registry))
	for _, s := range registry {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ", ")
}

func countFailed(results []Result) int {
	n := 0
	for _, r := range results {
		if r.Status != "pass" {
			n++
		}
	}
	return n
}
