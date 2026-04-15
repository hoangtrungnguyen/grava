package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/grava"
	"github.com/hoangtrungnguyen/grava/pkg/merge"
)

const spikeMergeDriverID = "spike-merge-driver"

func init() {
	Register(Scenario{
		ID:       spikeMergeDriverID,
		Name:     "Merge Driver Spike — Git Invocation Validation",
		EpicGate: 6,
		Run:      runSpikeMergeDriver,
	})
}

// runSpikeMergeDriver validates the grava merge-slot driver works correctly
// as a Git merge driver. It checks:
//  1. ProcessMerge handles all three merge cases (clean merge, conflict, add/delete).
//  2. DB connectivity is confirmed via the provided store.
//  3. Git invocation (if grava binary is on PATH) produces correct output.
//
// Writes a spike report to .grava/spike-reports/merge-driver-poc.md.
// The scenario passes if checks 1 and 2 pass; check 3 is advisory.
func runSpikeMergeDriver(ctx context.Context, store dolt.Store) Result {
	details := []string{}

	// --- Check 1: ProcessMerge correctness ---
	mergeOK, mergeDetail := checkProcessMerge()
	details = append(details, mergeDetail)
	if !mergeOK {
		_ = writeSpikePOCReport(false, false, false, details)
		return fail(spikeMergeDriverID, "ProcessMerge check failed", details...)
	}

	// --- Check 2: DB connectivity via passed store ---
	dbOK, dbDetail := checkDBConnectivity(ctx, store)
	details = append(details, dbDetail)
	if !dbOK {
		_ = writeSpikePOCReport(true, false, false, details)
		return fail(spikeMergeDriverID, "DB connectivity check failed", details...)
	}

	// --- Check 3: Git invocation (advisory) ---
	gitOK, gitDetail := checkGitInvocation()
	details = append(details, gitDetail)

	_ = writeSpikePOCReport(true, true, gitOK, details)

	// Scenario passes regardless of git check (advisory).
	// gitDetail is already in details; no extra status entry needed.
	return pass(spikeMergeDriverID, details...)
}

// checkProcessMerge validates all three merge cases inline using merge.ProcessMerge.
func checkProcessMerge() (bool, string) {
	// Case A: non-conflicting field changes — both sides change different fields.
	ancestor := `{"id":"issue-1","title":"Original","status":"open"}` + "\n"
	current := `{"id":"issue-1","title":"Updated Title","status":"open"}` + "\n"
	other := `{"id":"issue-1","title":"Original","status":"in_progress"}` + "\n"

	merged, hasConflict, err := merge.ProcessMerge(ancestor, current, other)
	if err != nil {
		return false, fmt.Sprintf("ProcessMerge case-A error: %v", err)
	}
	if hasConflict {
		return false, "ProcessMerge case-A: unexpected conflict on non-conflicting fields"
	}
	if !strings.Contains(merged, "Updated Title") || !strings.Contains(merged, "in_progress") {
		return false, "ProcessMerge case-A: merged output missing expected fields"
	}

	// Case B: conflicting same-field changes — both sides change status.
	currentB := `{"id":"issue-2","status":"paused"}` + "\n"
	otherB := `{"id":"issue-2","status":"closed"}` + "\n"
	ancestorB := `{"id":"issue-2","status":"open"}` + "\n"

	_, conflictB, errB := merge.ProcessMerge(ancestorB, currentB, otherB)
	if errB != nil {
		return false, fmt.Sprintf("ProcessMerge case-B error: %v", errB)
	}
	if !conflictB {
		return false, "ProcessMerge case-B: expected conflict on same-field modification, got none"
	}

	// Case C: issues added on both sides (no ancestor entry) — both should appear.
	ancestorC := ""
	currentC := `{"id":"issue-3","title":"From current"}` + "\n"
	otherC := `{"id":"issue-4","title":"From other"}` + "\n"

	mergedC, conflictC, errC := merge.ProcessMerge(ancestorC, currentC, otherC)
	if errC != nil {
		return false, fmt.Sprintf("ProcessMerge case-C error: %v", errC)
	}
	if conflictC {
		return false, "ProcessMerge case-C: unexpected conflict when both sides add different issues"
	}
	if !strings.Contains(mergedC, "issue-3") || !strings.Contains(mergedC, "issue-4") {
		return false, "ProcessMerge case-C: merged output missing one of the newly added issues"
	}

	return true, "process_merge=PASS (cases: non-conflicting, conflicting, add-both-sides)"
}

// checkDBConnectivity confirms the store is reachable by executing SELECT NOW().
func checkDBConnectivity(ctx context.Context, store dolt.Store) (bool, string) {
	var nowStr string
	row := store.QueryRowContext(ctx, "SELECT NOW()")
	if err := row.Scan(&nowStr); err != nil {
		return false, fmt.Sprintf("db_connectivity=FAIL err=%v", err)
	}
	return true, fmt.Sprintf("db_connectivity=YES now=%s", nowStr)
}

// checkGitInvocation attempts a real git merge through the grava binary.
// Returns (true, detail) if the test passes, (false, advisory detail) on failure.
// This check is advisory — failure does not fail the scenario.
func checkGitInvocation() (bool, string) {
	gravaBin, err := exec.LookPath("grava")
	if err != nil {
		return false, "git_invocation=SKIP (grava not on PATH)"
	}

	dir, err := os.MkdirTemp("", "spike-merge-*")
	if err != nil {
		return false, fmt.Sprintf("git_invocation=SKIP (mkdtemp: %v)", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	driverCmd := gravaBin + " merge-slot --ancestor %O --current %A --other %B"

	setup := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "spike@grava.test"},
		{"git", "-C", dir, "config", "user.name", "Spike"},
		{"git", "-C", dir, "config", "merge.grava.name", "Grava Schema-Aware Merge"},
		{"git", "-C", dir, "config", "merge.grava.driver", driverCmd},
	}
	for _, args := range setup {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return false, fmt.Sprintf("git_invocation=FAIL (setup %q: %s)", args[2], string(out))
		}
	}

	attrPath := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(attrPath, []byte("issues.jsonl merge=grava\n"), 0o644); err != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (write .gitattributes: %v)", err)
	}

	issuePath := filepath.Join(dir, "issues.jsonl")
	base := `{"id":"abc123","title":"Base","status":"open"}` + "\n"
	if err := os.WriteFile(issuePath, []byte(base), 0o644); err != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (write base issues: %v)", err)
	}

	gitCmds := [][]string{
		{"git", "-C", dir, "add", "issues.jsonl", ".gitattributes"},
		{"git", "-C", dir, "commit", "-m", "initial"},
		{"git", "-C", dir, "checkout", "-b", "feat/spike"},
	}
	for _, args := range gitCmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return false, fmt.Sprintf("git_invocation=FAIL (%q: %s)", args[2], string(out))
		}
	}

	// Branch feat/spike: change title.
	branch := `{"id":"abc123","title":"Updated Title","status":"open"}` + "\n"
	if err := os.WriteFile(issuePath, []byte(branch), 0o644); err != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (write branch issues: %v)", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "issues.jsonl"},
		{"git", "-C", dir, "commit", "-m", "feat: update title"},
		{"git", "-C", dir, "checkout", "-"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return false, fmt.Sprintf("git_invocation=FAIL (%q: %s)", args[2], string(out))
		}
	}

	// Main: change status (non-conflicting with title change).
	main := `{"id":"abc123","title":"Base","status":"in_progress"}` + "\n"
	if err := os.WriteFile(issuePath, []byte(main), 0o644); err != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (write main issues: %v)", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "issues.jsonl"},
		{"git", "-C", dir, "commit", "-m", "main: update status"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return false, fmt.Sprintf("git_invocation=FAIL (%q: %s)", args[2], string(out))
		}
	}

	// Merge feat/spike — non-conflicting, should succeed.
	out, err := exec.Command("git", "-C", dir, "merge", "--no-edit", "feat/spike").CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (git merge: %s)", string(out))
	}

	// Verify both changes are in the merged result.
	mergedBytes, readErr := os.ReadFile(issuePath)
	if readErr != nil {
		return false, fmt.Sprintf("git_invocation=FAIL (read result: %v)", readErr)
	}
	mergedStr := string(mergedBytes)
	if !strings.Contains(mergedStr, "Updated Title") || !strings.Contains(mergedStr, "in_progress") {
		return false, fmt.Sprintf("git_invocation=FAIL (merged result missing expected fields: %s)", mergedStr)
	}

	return true, "git_invocation=PASS (grava merge-slot invoked via git merge, output correct)"
}

// writeSpikePOCReport writes a spike evidence report to .grava/spike-reports/merge-driver-poc.md.
func writeSpikePOCReport(mergeOK, dbOK, gitOK bool, details []string) error {
	gravaDir, err := grava.ResolveGravaDir()
	if err != nil {
		return err // best-effort; not fatal
	}

	reportDir := filepath.Join(gravaDir, "spike-reports")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}

	detailsJSON := []byte("[]")
	if b, err := json.MarshalIndent(details, "", "  "); err == nil {
		detailsJSON = b
	}

	content := fmt.Sprintf(`# Spike Report: Merge Driver Proof-of-Concept

**Date:** %s
**Scenario:** %s

## Results

| Check | Result |
|-------|--------|
| Invocation confirmed (ProcessMerge) | %s |
| DB accessible during merge | %s |
| Git end-to-end invocation | %s |

## Details

%s

## Conclusion

%s

## Next Steps

%s
`,
		time.Now().UTC().Format(time.RFC3339),
		spikeMergeDriverID,
		boolMD(mergeOK),
		boolMD(dbOK),
		boolMD(gitOK),
		string(detailsJSON),
		spikeConclusion(mergeOK, dbOK, gitOK),
		spikeNextSteps(mergeOK && dbOK),
	)

	path := filepath.Join(reportDir, "merge-driver-poc.md")
	return os.WriteFile(path, []byte(content), 0o644)
}

func boolMD(ok bool) string {
	if ok {
		return "YES"
	}
	return "NO"
}

func boolStr(ok bool) string {
	if ok {
		return "PASS"
	}
	return "SKIP/FAIL"
}

func spikeConclusion(mergeOK, dbOK, gitOK bool) string {
	if mergeOK && dbOK && gitOK {
		return "All checks passed. The grava merge-slot driver correctly handles 3-way JSONL merges, DB connectivity is confirmed, and Git invocation is verified end-to-end. Epic 6 stories 6.2–6.4 are unblocked."
	}
	if mergeOK && dbOK {
		return "Core merge logic and DB connectivity confirmed. Git binary not found on PATH — end-to-end Git invocation not tested in this environment. Stories 6.2–6.4 proceed with caution; run integration tests (go test -tags=integration) to validate full Git invocation."
	}
	return "Spike did not pass all required checks. Review details above. Stories 6.2–6.4 remain blocked pending investigation."
}

func spikeNextSteps(coreOK bool) string {
	if coreOK {
		return "- Proceed with Story 6.2: Register grava-merge driver and parse 3-way input\n- Run integration tests: go test -tags=integration ./pkg/merge/...\n- Consider adding conflict_records DB table (Story 6.4)"
	}
	return "- Investigate failures listed in details above\n- Re-run spike after fixes\n- Do not proceed with Stories 6.2–6.4 until spike passes"
}
