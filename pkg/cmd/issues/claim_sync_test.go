package issues

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaimCmd_SyncsClaudeSettings verifies that after a successful claim and
// worktree provisioning, .claude/settings.json from the main repo is copied
// into the new worktree (AC5, grava-4136.2).
//
// This test is deliberately failing until grava-4136.2 wires the
// SyncClaudeSettings call in newClaimCmd.
func TestClaimCmd_SyncsClaudeSettings(t *testing.T) {
	mainRepo := t.TempDir()
	setupGitRepo(t, mainRepo) // defined in close_test.go (same package)

	// Place settings.json in the main repo's .claude dir
	settingsDir := filepath.Join(mainRepo, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0755))
	settingsContent := `{"enabledPlugins":{}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsContent), 0644))

	// Change cwd to the main repo so os.Getwd() returns it inside newClaimCmd
	t.Chdir(mainRepo)

	// Mock DB for the claim transaction
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "test-sync-abc1"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).
			AddRow("open", nil, nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("test-actor", "", "test-actor", issueID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test-actor"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	cmd := newClaimCmd(deps)
	cmd.SetArgs([]string{issueID})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err, "claim command should succeed")
	require.NoError(t, mock.ExpectationsWereMet())

	// Verify settings.json was synced into the provisioned worktree
	worktreeSettings := filepath.Join(mainRepo, ".worktree", issueID, ".claude", "settings.json")
	if assert.FileExists(t, worktreeSettings, "settings.json must be synced into the worktree") {
		got, _ := os.ReadFile(worktreeSettings)
		assert.Equal(t, settingsContent, string(got))
	}
}

// TestClaimCmd_SyncsClaudeSettings_AbsentSource verifies that claim succeeds
// even when the main repo has no .claude/settings.json (non-fatal, AC5).
// This variant is expected to pass now; the primary TestClaimCmd_SyncsClaudeSettings
// is the deliberately-failing TDD test (passes after grava-4136.2 wires the call).
func TestClaimCmd_SyncsClaudeSettings_AbsentSource(t *testing.T) {
	mainRepo := t.TempDir()
	setupGitRepo(t, mainRepo) // defined in close_test.go (same package)
	// No settings.json placed — source is absent

	t.Chdir(mainRepo)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	issueID := "test-sync-nosrc1"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, assignee, wisp_heartbeat_at FROM issues WHERE id").
		WithArgs(issueID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "assignee", "wisp_heartbeat_at"}).
			AddRow("open", nil, nil))
	mock.ExpectExec("UPDATE issues SET").
		WithArgs("test-actor", "", "test-actor", issueID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO events").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	var s dolt.Store = dolt.NewClientFromDB(db)
	outputJSON := false
	actor := "test-actor"
	model := ""
	deps := &cmddeps.Deps{Store: &s, Actor: &actor, AgentModel: &model, OutputJSON: &outputJSON}

	cmd := newClaimCmd(deps)
	cmd.SetArgs([]string{issueID})
	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err, "claim should succeed even when settings.json is absent")
	require.NoError(t, mock.ExpectationsWereMet())
}
