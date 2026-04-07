package issues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
)

// ScenarioResult holds the result of a single scenario execution.
type ScenarioResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"` // pass, fail, skip
	Duration  string `json:"duration,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SandboxReport is the JSON output for the sandbox run command.
type SandboxReport struct {
	Timestamp time.Time        `json:"timestamp"`
	Total     int              `json:"total"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Skipped   int              `json:"skipped"`
	Status    string           `json:"status"` // pass, fail
	Results   []ScenarioResult `json:"results"`
}

// ScenarioFunc is a function that implements a sandbox scenario.
// It returns an error if the scenario fails validation.
type ScenarioFunc func(ctx context.Context, store dolt.Store) error

// scenarioRegistry maps scenario IDs to their implementations.
var scenarioRegistry = map[string]struct {
	Name string
	Func ScenarioFunc
	Epic int
}{
	"TS-01": {"Basic Issue Lifecycle", runTS01, 2},
	"TS-02": {"Concurrent Atomic Claim", runTS02, 3},
}

func newSandboxCmd(d *cmddeps.Deps) *cobra.Command {
	var (
		scenario string
		all      bool
		epic     int
	)

	cmd := &cobra.Command{
		Use:   "sandbox run",
		Short: "Run sandbox validation scenarios",
		Long: `Execute sandbox validation scenarios for CI gating.
Each scenario exercises a specific workflow and reports pass/fail.
Exit 0 = all passed, exit 1 = failures, exit 2 = scenario not found.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := SandboxReport{
				Timestamp: time.Now(),
				Results:   []ScenarioResult{},
			}

			scenarios := selectScenarios(scenario, all, epic)
			if len(scenarios) == 0 {
				return gravaerrors.New("SCENARIO_GATE_NOT_MET",
					"no matching scenarios found", nil)
			}

			ctx := cmd.Context()
			if _, ok := ctx.Deadline(); !ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
			}

			for _, id := range scenarios {
				entry, ok := scenarioRegistry[id]
				if !ok {
					report.Results = append(report.Results, ScenarioResult{
						ID:     id,
						Name:   fmt.Sprintf("Unknown scenario %s", id),
						Status: "skip",
						Error:  "scenario not implemented",
					})
					report.Skipped++
					report.Total++
					continue
				}

				start := time.Now()
				err := entry.Func(ctx, *d.Store)
				duration := time.Since(start)

				result := ScenarioResult{
					ID:       id,
					Name:     entry.Name,
					Duration: duration.Round(time.Millisecond).String(),
				}

				if err != nil {
					result.Status = "fail"
					result.Error = err.Error()
					report.Failed++
				} else {
					result.Status = "pass"
					report.Passed++
				}
				report.Total++
				report.Results = append(report.Results, result)
			}

			if report.Failed > 0 {
				report.Status = "fail"
			} else {
				report.Status = "pass"
			}

			if *d.OutputJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				_ = enc.Encode(report) //nolint:errcheck
			} else {
				cmd.Printf("Sandbox Validation Report\n")
				cmd.Printf("========================\n")
				cmd.Printf("Total: %d  Passed: %d  Failed: %d  Skipped: %d\n\n",
					report.Total, report.Passed, report.Failed, report.Skipped)
				for _, r := range report.Results {
					icon := "PASS"
					if r.Status == "fail" {
						icon = "FAIL"
					} else if r.Status == "skip" {
						icon = "SKIP"
					}
					cmd.Printf("  [%s] %s (%s) %s\n", icon, r.ID, r.Name, r.Duration)
					if r.Error != "" {
						cmd.Printf("         %s\n", r.Error)
					}
				}
				cmd.Printf("\n")
				if report.Failed > 0 {
					cmd.Printf("Status: FAIL\n")
				} else {
					cmd.Printf("Status: PASS\n")
				}
			}

			if report.Failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&scenario, "scenario", "", "Run a specific scenario (e.g., TS-01)")
	cmd.Flags().BoolVar(&all, "all", false, "Run all registered scenarios")
	cmd.Flags().IntVar(&epic, "epic", 0, "Run all scenarios for a specific epic number")

	return cmd
}

// selectScenarios returns scenario IDs based on the filter flags.
func selectScenarios(scenario string, all bool, epic int) []string {
	if scenario != "" {
		return []string{scenario}
	}
	if all {
		ids := []string{}
		for id := range scenarioRegistry {
			ids = append(ids, id)
		}
		return ids
	}
	if epic > 0 {
		ids := []string{}
		for id, entry := range scenarioRegistry {
			if entry.Epic == epic {
				ids = append(ids, id)
			}
		}
		return ids
	}
	// Default: run all
	ids := []string{}
	for id := range scenarioRegistry {
		ids = append(ids, id)
	}
	return ids
}

// runTS01 validates the basic issue lifecycle: create → claim → verify.
func runTS01(ctx context.Context, store dolt.Store) error {
	res, err := createIssue(ctx, store, CreateParams{
		Title:     "TS-01: Basic Issue Lifecycle",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "sandbox-agent",
		Model:     "sandbox",
	})
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}

	claimRes, err := claimIssue(ctx, store, res.ID, "sandbox-agent", "sandbox")
	if err != nil {
		return fmt.Errorf("claim failed: %w", err)
	}

	if claimRes.Status != "in_progress" {
		return fmt.Errorf("expected status in_progress, got %s", claimRes.Status)
	}
	if claimRes.Actor != "sandbox-agent" {
		return fmt.Errorf("expected actor sandbox-agent, got %s", claimRes.Actor)
	}

	var status, assignee string
	if err := store.QueryRowContext(ctx,
		"SELECT status, assignee FROM issues WHERE id = ?", res.ID,
	).Scan(&status, &assignee); err != nil {
		return fmt.Errorf("failed to query issue state: %w", err)
	}
	if status != "in_progress" {
		return fmt.Errorf("expected DB status in_progress, got %s", status)
	}
	if assignee != "sandbox-agent" {
		return fmt.Errorf("expected DB assignee sandbox-agent, got %s", assignee)
	}

	return nil
}

// runTS02 validates concurrent atomic claim: two agents race for the same issue.
// Exactly one must succeed; the other must receive ALREADY_CLAIMED.
func runTS02(ctx context.Context, store dolt.Store) error {
	res, err := createIssue(ctx, store, CreateParams{
		Title:     "TS-02: Concurrent Atomic Claim",
		IssueType: "task",
		Priority:  "high",
		Actor:     "sandbox-setup",
		Model:     "sandbox",
	})
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}

	type outcome struct {
		result ClaimResult
		err    error
	}
	ch := make(chan outcome, 2)

	for _, actor := range []string{"agent-alpha", "agent-beta"} {
		go func(a string) {
			r, e := claimIssue(ctx, store, res.ID, a, "sandbox")
			ch <- outcome{result: r, err: e}
		}(actor)
	}

	var successes, failures int
	for i := 0; i < 2; i++ {
		o := <-ch
		if o.err == nil {
			successes++
		} else {
			failures++
			var gravaErr *gravaerrors.GravaError
			if !errors.As(o.err, &gravaErr) || gravaErr.Code != "ALREADY_CLAIMED" {
				return fmt.Errorf("unexpected error: %v (expected ALREADY_CLAIMED)", o.err)
			}
		}
	}

	if successes != 1 {
		return fmt.Errorf("expected exactly 1 successful claim, got %d", successes)
	}
	if failures != 1 {
		return fmt.Errorf("expected exactly 1 failed claim, got %d", failures)
	}

	var status, assignee string
	if err := store.QueryRowContext(ctx,
		"SELECT status, assignee FROM issues WHERE id = ?", res.ID,
	).Scan(&status, &assignee); err != nil {
		return fmt.Errorf("failed to query issue state: %w", err)
	}
	if status != "in_progress" {
		return fmt.Errorf("expected DB status in_progress, got %s", status)
	}
	if assignee == "" {
		return fmt.Errorf("expected non-empty assignee")
	}

	return nil
}
