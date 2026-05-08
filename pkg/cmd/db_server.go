package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(newDBStartCmd())
	rootCmd.AddCommand(newDBStopCmd())
}

// lsofPidsFn returns the PIDs (as strings) of processes listening on the
// given TCP port. Default impl shells out to `lsof -t -i:<port>`. Tests
// override this to inject deterministic PIDs without touching the real
// lsof binary or the network.
//
// Returns nil + nil error when no process is listening (lsof exits non-zero
// with empty output in that case).
var lsofPidsFn = func(port int) ([]string, error) {
	out, err := exec.Command("lsof", "-t", fmt.Sprintf("-i:%d", port)).Output()
	if err != nil {
		// lsof returns non-zero when no match — treat as empty, not error.
		// Distinguishing "no match" from "lsof not installed" would require
		// inspecting the ExitError, but for db-stop the caller treats both
		// the same way: nothing to kill.
		if len(out) == 0 {
			return nil, nil
		}
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	// strings.Fields splits on any whitespace including newlines, so
	// multi-PID output like "12345\n67890\n" produces ["12345","67890"].
	return strings.Fields(strings.TrimSpace(string(out))), nil
}

// killPidFn sends SIGTERM to the given PID. Default impl shells out to
// `kill -TERM <pid>`. Tests override this to record kill targets.
var killPidFn = func(pid string) error {
	return exec.Command("kill", "-TERM", pid).Run()
}

// inFlightCountFn returns the number of non-ephemeral issues currently
// status=in_progress. Used by db-stop to refuse teardown while /ship runs
// are mid-flight. Returns (count, true) on success, (0, false) when the
// DB is unreachable (best-effort: if Dolt is already down, the check is
// skipped because there's nothing to corrupt).
//
// Defined as a function variable so tests can simulate "claim happened
// between the early check and the kill" (TOCTOU window) by returning
// different counts on successive calls.
var inFlightCountFn = func() (int, bool) {
	store, err := connectDBFn()
	if err != nil {
		return 0, false
	}
	defer store.Close() //nolint:errcheck
	var n int
	row := store.QueryRow(
		"SELECT COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'in_progress'",
	)
	if scanErr := row.Scan(&n); scanErr != nil {
		return 0, false
	}
	return n, true
}

func newDBStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-start",
		Short: "Start the Dolt SQL server",
		Long: `Start the Dolt SQL server for the current Grava repository.
It uses the repository in .grava/dolt and the port specified in .grava.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			gravaDir, err := grava.ResolveGravaDir()
			if err != nil {
				return err
			}
			cwd, _ := os.Getwd()
			doltBin, err := utils.ResolveDoltBinary(cwd)
			if err != nil {
				return fmt.Errorf("dolt binary not found: %w", err)
			}

			doltRepoDir := filepath.Join(gravaDir, "dolt")

			// Resolve port
			dbURL := viper.GetString("db_url")
			if dbURL == "" {
				dbURL = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
			}

			// Extract port from URL if possible, otherwise use 3306
			port := 3306
			fmt.Sscanf(dbURL, "root@tcp(127.0.0.1:%d)", &port)

			// Check if already running
			testStore, err := dolt.NewClient(dbURL)
			if err == nil {
				testStore.Close()
				cmd.Printf("✅ Dolt server is already running on port %d\n", port)
				return nil
			}

			cmd.Printf("🚀 Starting Dolt server on port %d...\n", port)
			serverCmd := exec.Command(doltBin, "sql-server", "--port", fmt.Sprintf("%d", port), "--host", "0.0.0.0")
			serverCmd.Dir = doltRepoDir

			logFile, err := os.OpenFile(filepath.Join(gravaDir, "dolt.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				serverCmd.Stdout = logFile
				serverCmd.Stderr = logFile
			}

			if err := serverCmd.Start(); err != nil {
				return fmt.Errorf("failed to start dolt server: %w", err)
			}

			// Wait for server ready
			for i := 0; i < 10; i++ {
				initStore, err := dolt.NewClient(dbURL)
				if err == nil {
					initStore.Close()
					break
				}
				time.Sleep(500 * time.Millisecond)
				if i == 9 {
					return fmt.Errorf("dolt server did not become ready")
				}
			}

			cmd.Printf("✅ Dolt server started successfully.\n")
			return nil
		},
	}
}

// runDBStop is the testable core of db-stop, decoupled from cobra flag
// parsing and viper config resolution. It enforces the in-flight guard,
// re-checks the guard immediately before issuing kills, then SIGTERMs
// every PID listening on the port.
//
// Concurrency note (TOCTOU): the guard reads `issues WHERE status =
// 'in_progress'` twice — once at entry, and once again immediately
// before the first kill. Without DB-level locking we cannot make the
// check-then-kill sequence strictly atomic with `grava claim`, but the
// re-check shrinks the window from "however long lsof + flag parsing
// takes" to "the time between QueryRow returning and exec.Command.Run()
// dispatching" — millisecond-scale rather than second-scale. A claim
// landing inside that residual window will still see Dolt killed
// mid-write; pass --force only when you accept that risk, otherwise
// wait for /ship runs to finish.
func runDBStop(cmd *cobra.Command, port int, force bool) error {
	// Concurrency guard (concurrency-matrix #4): refuse to stop while
	// any /ship run might be writing wisps. Best-effort — if the DB is
	// unreachable (already down), skip the check.
	if !force {
		if n, ok := inFlightCountFn(); ok && n > 0 {
			return fmt.Errorf(
				"refusing to stop: %d issue(s) are status=in_progress "+
					"(active /ship runs would lose state). "+
					"Wait for them to finish, or pass --force to override",
				n,
			)
		}
	}

	cmd.Printf("🛑 Stopping Dolt server on port %d...\n", port)

	pids, err := lsofPidsFn(port)
	if err != nil {
		return fmt.Errorf("failed to list processes on port %d: %w", port, err)
	}
	if len(pids) == 0 {
		cmd.Printf("No process found listening on port %d.\n", port)
		return nil
	}

	// TOCTOU re-check: a /ship run may have called `grava claim`
	// between the entry check above and now (lsof took some real time).
	// Re-confirm the in-flight count is still zero before we send any
	// signals. This narrows but does not eliminate the race — see the
	// runDBStop doc comment for the residual window.
	if !force {
		if n, ok := inFlightCountFn(); ok && n > 0 {
			return fmt.Errorf(
				"refusing to stop: %d issue(s) became status=in_progress "+
					"during teardown (a /ship run claimed mid-flight). "+
					"Wait for them to finish, or pass --force to override",
				n,
			)
		}
	}

	// Send SIGTERM to every PID. Continue past individual failures so
	// one stuck process does not leave others alive.
	var killErrs []string
	for _, pid := range pids {
		if err := killPidFn(pid); err != nil {
			cmd.Printf("⚠️  failed to kill pid %s: %v\n", pid, err)
			killErrs = append(killErrs, fmt.Sprintf("%s: %v", pid, err))
		}
	}

	// Wait for the port to clear. We poll lsof rather than tracking
	// individual PIDs because new dolt children may inherit the listen
	// socket; "port empty" is the real success signal.
	for i := 0; i < 10; i++ {
		remaining, _ := lsofPidsFn(port)
		if len(remaining) == 0 {
			cmd.Printf("✅ Dolt server stopped.\n")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	if len(killErrs) > 0 {
		return fmt.Errorf("failed to stop server within timeout (kill errors: %s)", strings.Join(killErrs, "; "))
	}
	return fmt.Errorf("failed to stop server within timeout")
}

func newDBStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db-stop",
		Short: "Stop the Dolt SQL server",
		Long: `Stop the Dolt SQL server running on the port configured for this repository.

Refuses to stop while any non-ephemeral issues are status=in_progress
(claimed by an active /ship run). Stopping mid-flight breaks the
pipeline: the orchestrator's next 'grava signal' write fails and the
issue is left in an inconsistent state. Pass --force to override.

The in-progress check runs twice — once at entry, and once again just
before SIGTERM is sent — to narrow the TOCTOU window where a 'grava
claim' could land between check and kill. The residual window is
millisecond-scale; if you need stricter atomicity, hold an external
lock around your db-stop invocation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			dbURL := viper.GetString("db_url")
			if dbURL == "" {
				dbURL = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
			}

			port := 3306
			fmt.Sscanf(dbURL, "root@tcp(127.0.0.1:%d)", &port)

			return runDBStop(cmd, port, force)
		},
	}
	cmd.Flags().Bool("force", false, "Stop even if issues are in_progress (DANGEROUS — breaks active /ship runs)")
	return cmd
}
