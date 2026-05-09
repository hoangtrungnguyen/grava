// Regression tests for grava-37c8: graph subcommands (visualize, stats, health,
// cycle) ignore --json flag. The fix (commit 3ed9b83) wired *d.OutputJSON checks
// into each RunE; these tests pin the global-flag pathway so future refactors
// can't silently regress to text-only output.
package cmdgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeGraphDeps wires a fresh sqlmock store + cmddeps.Deps with the requested
// global --json flag value. Each test owns its own DB so parallel runs are safe.
func makeGraphDeps(t *testing.T, outputJSON bool) (*cmddeps.Deps, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	var s dolt.Store = dolt.NewClientFromDB(db)
	actor := "test"
	model := ""
	jsonFlag := outputJSON
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &jsonFlag}
	return deps, mock, func() { _ = db.Close() }
}

// expectGraphLoad mocks the two queries graph.LoadGraphFromDB issues. Pass the
// rows for issues and dependencies; columns are pinned via issueCols/depCols.
func expectGraphLoad(mock sqlmock.Sqlmock, issues, deps *sqlmock.Rows) {
	mock.ExpectQuery(qVisualizeLoadIssues).WillReturnRows(issues)
	mock.ExpectQuery(qVisualizeLoadDeps).WillReturnRows(deps)
}

// ---- graph stats ----

// TestGraphStatsCmd_JSONOutput pins AC for grava-37c8: `grava graph stats --json`
// emits a parseable JSON object with node/edge counts.
func TestGraphStatsCmd_JSONOutput(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, true)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "B", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphStatsCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "stats --json must emit valid JSON, got: %s", buf.String())
	assert.EqualValues(t, 2, result["nodes"])
	assert.EqualValues(t, 1, result["edges"])
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGraphStatsCmd_TextOutputUnchanged regression-guards the human-readable
// text path so adding JSON didn't break the default output (AC: "When --json
// absent, behaviour unchanged").
func TestGraphStatsCmd_TextOutputUnchanged(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, false)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "B", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphStatsCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	out := buf.String()
	assert.Contains(t, out, "Nodes:")
	assert.Contains(t, out, "Edges:")
	assert.False(t, json.Valid(buf.Bytes()), "text output must not be valid JSON")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- graph cycle ----

// TestGraphCycleCmd_JSONOutput_NoCycle pins AC for grava-37c8: `grava graph
// cycle --json` returns a parseable {has_cycle, cycle} object on a clean graph.
func TestGraphCycleCmd_JSONOutput_NoCycle(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, true)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphCycleCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	var result struct {
		HasCycle bool     `json:"has_cycle"`
		Cycle    []string `json:"cycle"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "cycle --json must emit valid JSON, got: %s", buf.String())
	assert.False(t, result.HasCycle)
	assert.Empty(t, result.Cycle)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGraphCycleCmd_TextOutputUnchanged guards the default text output path.
func TestGraphCycleCmd_TextOutputUnchanged(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, false)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphCycleCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	out := buf.String()
	assert.Contains(t, out, "No cycles detected")
	assert.False(t, json.Valid(buf.Bytes()), "text output must not be valid JSON")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- graph health ----

// TestGraphHealthCmd_JSONOutput pins AC for grava-37c8: `grava graph health
// --json` emits a parseable {has_cycle, cycle, orphans} object.
func TestGraphHealthCmd_JSONOutput(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, true)
	defer cleanup()

	// Two connected tasks (no orphans, no cycles) plus one orphan.
	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "B", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-3", "Orphan", "task", "open", 3, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphHealthCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	var result struct {
		HasCycle bool     `json:"has_cycle"`
		Cycle    []string `json:"cycle"`
		Orphans  int      `json:"orphans"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "health --json must emit valid JSON, got: %s", buf.String())
	assert.False(t, result.HasCycle)
	assert.Equal(t, 1, result.Orphans)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGraphHealthCmd_TextOutputUnchanged guards the default text output path.
func TestGraphHealthCmd_TextOutputUnchanged(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, false)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphHealthCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	out := buf.String()
	assert.Contains(t, out, "Performing health check")
	assert.Contains(t, out, "Cycles:")
	assert.False(t, json.Valid(buf.Bytes()), "text output must not be valid JSON")
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- graph visualize ----

// TestGraphVisualizeCmd_GlobalJSONFlag pins AC for grava-37c8: the global
// --json flag (i.e. *d.OutputJSON=true) MUST override the default ascii format
// and produce parseable JSON, even when the user did not pass --format json.
// The pre-fix bug was: --json was accepted but ignored, so output stayed ascii.
// Existing TestGraphVisualizeCmd_FormatJSON only exercises the local --format
// flag, not the global pathway.
func TestGraphVisualizeCmd_GlobalJSONFlag(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, true)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil).
			AddRow("task-2", "B", "task", "open", 2, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()).AddRow("task-1", "task-2", "blocks", []byte("{}")),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphVisualizeCmd(deps)
	cmd.SetOut(buf)
	// Note: NO --format json flag — relying purely on the global --json plumbing.
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	require.True(t, json.Valid(buf.Bytes()),
		"visualize with global --json must emit valid JSON, got: %s", buf.String())

	var result struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.Edges, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGraphVisualizeCmd_DefaultStaysASCII regression-guards the no-flag case:
// without --json or --format, output must remain ascii (the default).
func TestGraphVisualizeCmd_DefaultStaysASCII(t *testing.T) {
	deps, mock, cleanup := makeGraphDeps(t, false)
	defer cleanup()

	expectGraphLoad(mock,
		sqlmock.NewRows(issueCols()).
			AddRow("task-1", "A", "task", "open", 1, time.Now(), time.Now(), nil, nil, 0, nil),
		sqlmock.NewRows(depCols()),
	)

	buf := &bytes.Buffer{}
	cmd := newGraphVisualizeCmd(deps)
	cmd.SetOut(buf)
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	assert.False(t, json.Valid(buf.Bytes()), "default visualize output must not be valid JSON")
	assert.Contains(t, buf.String(), "A")
}
