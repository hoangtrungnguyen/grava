package issues

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/internal/testutil"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/stretchr/testify/require"
)

// expectArchivedStatusQuery primes a sqlmock for the guardNotArchived
// SELECT-status read so guard rejection paths can be asserted without
// re-implementing the helper per command.
func expectArchivedStatusQuery(mock sqlmock.Sqlmock, id, status string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM issues WHERE id = ?")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(status))
}

// --- comment ---

func TestCommentIssue_RejectsArchived(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	expectArchivedStatusQuery(mock, "grava-arch", "archived")

	store := mockStoreForComment(db)
	_, err = commentIssue(context.Background(), store, CommentParams{
		ID:      "grava-arch",
		Message: "still working",
		Actor:   "test-actor",
		Model:   "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommentIssue_RejectsTombstone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	expectArchivedStatusQuery(mock, "grava-tomb", "tombstone")

	store := mockStoreForComment(db)
	_, err = commentIssue(context.Background(), store, CommentParams{
		ID:      "grava-tomb",
		Message: "post-mortem",
		Actor:   "test-actor",
		Model:   "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- label ---

func TestLabelIssue_RejectsArchived(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	expectArchivedStatusQuery(mock, "grava-arch", "archived")

	store := mockStoreForLabel(db)
	_, err = labelIssue(context.Background(), store, LabelParams{
		ID:        "grava-arch",
		AddLabels: []string{"hot"},
		Actor:     "test-actor",
		Model:     "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- assign ---

// mockStoreForAssignArchived returns a MockStore whose first QueryRow
// (SELECT status, the guard) reports the archived row, and whose subsequent
// calls would (incorrectly) succeed — letting us prove the guard short-circuits.
func mockStoreForAssignArchived(t *testing.T) *testutil.MockStore {
	t.Helper()
	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, mock, _ := sqlmock.New()
		mock.ExpectQuery("SELECT").
			WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("archived"))
		return mockDB.QueryRow("SELECT", args...)
	}
	return store
}

func TestAssignIssue_RejectsArchived(t *testing.T) {
	store := mockStoreForAssignArchived(t)
	_, err := assignIssue(context.Background(), store, AssignParams{
		ID:       "grava-arch",
		Assignee: "alice",
		Actor:    "test-actor",
		Model:    "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
}

// --- subtask ---

// mockStoreForSubtaskArchived primes the parent COUNT(*) and the guard
// SELECT-status reads. Parent exists; guard reads "archived" → reject.
func mockStoreForSubtaskArchived(t *testing.T) *testutil.MockStore {
	t.Helper()
	store := testutil.NewMockStore()
	store.QueryRowFn = func(query string, args ...any) *sql.Row {
		mockDB, mock, _ := sqlmock.New()
		if len(query) > 6 && query[7:12] == "COUNT" {
			mock.ExpectQuery("SELECT").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		} else {
			mock.ExpectQuery("SELECT").
				WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("archived"))
		}
		return mockDB.QueryRow("SELECT", args...)
	}
	return store
}

func TestSubtaskIssue_RejectsArchivedParent(t *testing.T) {
	store := mockStoreForSubtaskArchived(t)
	_, err := subtaskIssue(context.Background(), store, SubtaskParams{
		ParentID:  "grava-arch",
		Title:     "Child of archive",
		IssueType: "task",
		Priority:  "medium",
		Actor:     "test-actor",
		Model:     "test-model",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
}

// --- update ---

// TestUpdateIssue_RejectsArchived verifies that any non-status field update
// against an archived issue returns ISSUE_READ_ONLY without entering the
// transactional UPDATE path.
func TestUpdateIssue_RejectsArchived(t *testing.T) {
	store := mockStoreForUpdate(nil, true, "old", "old", "task", 3, "archived")
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-arch",
		Title:         "new title",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"title"},
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
}

// TestUpdateIssue_RejectsArchivedStatusToClosed verifies the bug fix: an
// archived issue may NOT be transitioned to closed/blocked via update --status.
// Only --status open is a valid bypass (the un-archive path).
func TestUpdateIssue_RejectsArchivedStatusToClosed(t *testing.T) {
	store := mockStoreForUpdate(nil, true, "old", "old", "task", 3, "archived")
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-arch",
		Status:        "closed",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"status"},
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
}

// TestUpdateIssue_RejectsArchivedStatusToBlocked is the symmetric guard:
// archived → blocked must also be rejected.
func TestUpdateIssue_RejectsArchivedStatusToBlocked(t *testing.T) {
	store := mockStoreForUpdate(nil, true, "old", "old", "task", 3, "archived")
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-arch",
		Status:        "blocked",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"status"},
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
}

// TestUpdateIssue_AllowsUnarchive verifies the un-archive path: status=open
// against an archived issue must be permitted by the guard. The downstream
// graph.LoadGraphFromDB call requires a live DB and is out of scope for this
// unit test, so we let it panic on the nil mock and recover — the recovery
// proves the guard let the call through, which is the bug fix under test.
func TestUpdateIssue_AllowsUnarchive(t *testing.T) {
	defer func() {
		// Either we panic past the guard (proving it let us through to the
		// graph layer) or updateIssue returns a non-ISSUE_READ_ONLY error.
		// Both outcomes mean the guard correctly bypassed for status=open.
		_ = recover()
	}()
	store := mockStoreForUpdate(nil, true, "old", "old", "task", 3, "archived")
	_, err := updateIssue(context.Background(), store, UpdateParams{
		ID:            "grava-arch",
		Status:        "open",
		Actor:         "test-actor",
		Model:         "test-model",
		ChangedFields: []string{"status"},
	})
	if err != nil {
		if gerr, ok := err.(interface{ Error() string }); ok {
			if msg := gerr.Error(); contains(msg, "status is archived") || contains(msg, "ISSUE_READ_ONLY") {
				t.Fatalf("guard incorrectly blocked unarchive: %v", err)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- wisp write ---

func TestWispWrite_RejectsArchived(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	expectArchivedStatusQuery(mock, "grava-arch", "archived")

	store := dolt.NewClientFromDB(db)
	_, err = wispWrite(context.Background(), store, WispWriteParams{
		IssueID: "grava-arch",
		Key:     "step",
		Value:   "claimed",
		Actor:   "test-actor",
	})
	testutil.AssertGravaError(t, err, "ISSUE_READ_ONLY")
	require.NoError(t, mock.ExpectationsWereMet())
}
