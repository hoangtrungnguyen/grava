package cmdgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStatsMock(t *testing.T, outputJSON bool) (*cmddeps.Deps, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	var s dolt.Store = dolt.NewClientFromDB(db)
	actor := "test"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}
	return deps, mock, func() { db.Close() } //nolint:errcheck
}

func expectStatsQueries(mock sqlmock.Sqlmock, blockedCount, staleCount int, avgCycle *float64) {
	// by-status query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY status`)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow("open", 5).
			AddRow("in_progress", 3).
			AddRow("blocked", blockedCount).
			AddRow("closed", 2))

	// blocked count
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'blocked'`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(blockedCount))

	// stale in_progress count
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'in_progress' AND COALESCE(wisp_heartbeat_at, updated_at) < DATE_SUB(NOW(), INTERVAL 1 HOUR)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(staleCount))

	// avg cycle time
	row := sqlmock.NewRows([]string{"avg"})
	if avgCycle != nil {
		row = row.AddRow(*avgCycle)
	} else {
		row = row.AddRow(nil)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT AVG(TIMESTAMPDIFF(MINUTE, started_at, stopped_at)) FROM issues WHERE ephemeral = 0 AND status = 'closed' AND started_at IS NOT NULL AND stopped_at IS NOT NULL`)).
		WillReturnRows(row)

	// by-priority query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT priority, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY priority`)).
		WillReturnRows(sqlmock.NewRows([]string{"priority", "count"}).
			AddRow(0, 1).AddRow(2, 4))

	// by-author query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT created_by, COUNT(*) FROM issues WHERE ephemeral = 0 GROUP BY created_by ORDER BY COUNT(*) DESC LIMIT 10`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_by", "count"}).
			AddRow("alice", 6))

	// by-assignee query
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT assignee, COUNT(*) FROM issues WHERE ephemeral = 0 AND assignee IS NOT NULL AND assignee != '' GROUP BY assignee ORDER BY COUNT(*) DESC LIMIT 10`)).
		WillReturnRows(sqlmock.NewRows([]string{"assignee", "count"}).
			AddRow("bob", 3))

	// created by date (default --days=7)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(`SELECT DATE_FORMAT(created_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND created_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC`, 7))).
		WillReturnRows(sqlmock.NewRows([]string{"day", "count"}).
			AddRow("2026-04-11", 2))

	// closed by date (default --days=7)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(`SELECT DATE_FORMAT(updated_at, '%%Y-%%m-%%d') as day, COUNT(*) FROM issues WHERE ephemeral = 0 AND status = 'closed' AND updated_at >= DATE_SUB(NOW(), INTERVAL %d DAY) GROUP BY day ORDER BY day DESC`, 7))).
		WillReturnRows(sqlmock.NewRows([]string{"day", "count"}).
			AddRow("2026-04-11", 1))
}

// TestStatsCmd_TextOutput tests the default human-readable output.
func TestStatsCmd_TextOutput(t *testing.T) {
	deps, mock, cleanup := setupStatsMock(t, false)
	defer cleanup()

	avg := 42.0
	expectStatsQueries(mock, 2, 1, &avg)

	buf := &bytes.Buffer{}
	cmd := newStatsCmd(deps)
	cmd.SetOut(buf)
	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Blocked Issues:")
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "Stale In-Progress:")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "Avg Cycle Time:")
	assert.Contains(t, out, "42 min")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestStatsCmd_JSONOutput verifies NFR5 JSON schema fields.
func TestStatsCmd_JSONOutput(t *testing.T) {
	deps, mock, cleanup := setupStatsMock(t, true)
	defer cleanup()

	avg := 120.5
	expectStatsQueries(mock, 3, 2, &avg)

	buf := &bytes.Buffer{}
	cmd := newStatsCmd(deps)
	cmd.SetOut(buf)
	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result StatsResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, 3, result.BlockedCount)
	assert.Equal(t, 2, result.StaleInProgressCount)
	assert.InDelta(t, 120.5, result.AvgCycleTimeMinutes, 0.01)
	assert.Greater(t, result.Total, 0)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestStatsCmd_ZeroBlockedAndStale tests output when no blocked or stale issues.
func TestStatsCmd_ZeroBlockedAndStale(t *testing.T) {
	deps, mock, cleanup := setupStatsMock(t, true)
	defer cleanup()

	expectStatsQueries(mock, 0, 0, nil)

	buf := &bytes.Buffer{}
	cmd := newStatsCmd(deps)
	cmd.SetOut(buf)
	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	var result StatsResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, 0, result.BlockedCount)
	assert.Equal(t, 0, result.StaleInProgressCount)
	assert.Equal(t, 0.0, result.AvgCycleTimeMinutes)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestStatsCmd_NoCycleTimeData tests when no closed issues have work session data.
func TestStatsCmd_NoCycleTimeData(t *testing.T) {
	deps, mock, cleanup := setupStatsMock(t, false)
	defer cleanup()

	expectStatsQueries(mock, 0, 0, nil)

	buf := &bytes.Buffer{}
	cmd := newStatsCmd(deps)
	cmd.SetOut(buf)
	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Avg Cycle Time line should not appear when there's no data
	assert.NotContains(t, buf.String(), "Avg Cycle Time:")
	require.NoError(t, mock.ExpectationsWereMet())
}
