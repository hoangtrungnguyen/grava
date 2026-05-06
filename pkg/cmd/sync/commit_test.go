package synccmd

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitCmd_LeavesCleanDoltStatus verifies the FR-AUDIT-COMMIT contract:
// after `grava commit` returns, all writes (including the audit row that
// `commit` itself produces) must be folded into Dolt history. Concretely:
//
//  1. RunE issues the user's commit: DOLT_ADD('-A') + DOLT_COMMIT('-m', userMsg).
//  2. RunE then records its own audit row via INSERT INTO cmd_audit_log.
//  3. RunE issues a follow-up DOLT_ADD('cmd_audit_log') + DOLT_COMMIT to fold
//     the audit row into history, leaving cmd_audit_log clean.
//
// Without (3), bug grava-ff4b leaves cmd_audit_log perpetually dirty after
// every grava command, which breaks `grava import`'s Dual-Safety Check.
func TestCommitCmd_LeavesCleanDoltStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// Step 1: user's commit.
	mock.ExpectExec(regexp.QuoteMeta("CALL DOLT_ADD('-A')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("CALL DOLT_COMMIT('-m', ?)")).
		WithArgs("test message").
		WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow("user-commit-hash"))

	// Step 2: self-audit insert (the commit records its own audit row).
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO cmd_audit_log")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 3: follow-up audit-only commit that folds cmd_audit_log into history.
	mock.ExpectExec(regexp.QuoteMeta("CALL DOLT_ADD('cmd_audit_log')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("CALL DOLT_COMMIT('-m', ?)")).
		WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow("audit-commit-hash"))

	store := dolt.NewClientFromDB(db)
	d := makeDeps(store)
	cmd := newCommitCmd(d)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.Flags().Set("message", "test message"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "user-commit-hash")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCommitCmd_AuditFollowUpCommit_FailureDoesNotBreakUserCommit verifies
// that a failure in the follow-up audit-only DOLT_COMMIT does not surface as
// a user-facing error. The user's commit already succeeded; an audit hiccup
// must not make the user think their commit failed.
func TestCommitCmd_AuditFollowUpCommit_FailureDoesNotBreakUserCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// User's commit succeeds.
	mock.ExpectExec(regexp.QuoteMeta("CALL DOLT_ADD('-A')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("CALL DOLT_COMMIT('-m', ?)")).
		WithArgs("user msg").
		WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow("user-hash"))

	// Self-audit insert succeeds.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO cmd_audit_log")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Follow-up audit DOLT_ADD succeeds, but the audit DOLT_COMMIT fails.
	mock.ExpectExec(regexp.QuoteMeta("CALL DOLT_ADD('cmd_audit_log')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("CALL DOLT_COMMIT('-m', ?)")).
		WillReturnError(assert.AnError)

	store := dolt.NewClientFromDB(db)
	d := makeDeps(store)
	cmd := newCommitCmd(d)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.Flags().Set("message", "user msg"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	// Must not return an error to the user.
	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, out.String(), "user-hash")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCommitCmd_UserCommitFailure_ReturnsError verifies that a failure in
// the user's commit (DOLT_COMMIT) is surfaced as an error and skips the
// self-audit + follow-up commit (no point auditing a failed commit).
func TestCommitCmd_UserCommitFailure_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectExec(regexp.QuoteMeta("CALL DOLT_ADD('-A')")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("CALL DOLT_COMMIT('-m', ?)")).
		WillReturnError(assert.AnError)

	// No self-audit insert, no follow-up commit expected.

	store := dolt.NewClientFromDB(db)
	d := makeDeps(store)
	cmd := newCommitCmd(d)
	cmd.SetContext(context.Background())
	require.NoError(t, cmd.Flags().Set("message", "user msg"))

	var out bytes.Buffer
	cmd.SetOut(&out)

	require.Error(t, cmd.RunE(cmd, nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}
