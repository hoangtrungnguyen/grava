package synccmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


// --- flat JSONL export/import tests ---

func TestExportFlatJSONL_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery("SELECT .* FROM issues WHERE").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "issue_type", "priority", "status",
			"metadata", "created_at", "updated_at", "created_by", "updated_by",
			"agent_model", "affected_files", "ephemeral",
		}))

	store := dolt.NewClientFromDB(db)
	var buf bytes.Buffer
	count, err := exportFlatJSONL(context.Background(), store, &buf, false)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, buf.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExportFlatJSONL_SingleIssue_NoRelations(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)

	issueRows := sqlmock.NewRows([]string{
		"id", "title", "description", "issue_type", "priority", "status",
		"metadata", "created_at", "updated_at", "created_by", "updated_by",
		"agent_model", "affected_files", "ephemeral",
	}).AddRow("grava-abc1", "Test", "desc", "task", 2, "open",
		[]byte(`{}`), now, now, "actor1", "actor1", nil, []byte(`[]`), false)

	mock.ExpectQuery("SELECT .* FROM issues WHERE").WillReturnRows(issueRows)
	// labels, comments, deps, wisps queries — return empty
	mock.ExpectQuery("SELECT issue_id, label FROM issue_labels").
		WillReturnRows(sqlmock.NewRows([]string{"issue_id", "label"}))
	mock.ExpectQuery("SELECT issue_id, id, message").
		WillReturnRows(sqlmock.NewRows([]string{"issue_id", "id", "message", "actor", "agent_model", "created_at"}))
	mock.ExpectQuery("SELECT from_id, to_id").
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "created_by", "updated_by", "agent_model"}))
	mock.ExpectQuery("SELECT issue_id, key_name").
		WillReturnRows(sqlmock.NewRows([]string{"issue_id", "key_name", "value", "written_by", "written_at"}))

	store := dolt.NewClientFromDB(db)
	var buf bytes.Buffer
	count, err := exportFlatJSONL(context.Background(), store, &buf, false)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var rec IssueJSONLRecord
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	assert.Equal(t, "grava-abc1", rec.ID)
	assert.Equal(t, "Test", rec.Title)
	assert.Empty(t, rec.Labels)
	assert.Empty(t, rec.Comments)
	assert.Empty(t, rec.Dependencies)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExportFlatJSONL_WithLabelsAndComments(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)
	actor := "dev1"

	issueRows := sqlmock.NewRows([]string{
		"id", "title", "description", "issue_type", "priority", "status",
		"metadata", "created_at", "updated_at", "created_by", "updated_by",
		"agent_model", "affected_files", "ephemeral",
	}).AddRow("grava-xyz2", "Labeled Issue", "desc", "bug", 1, "open",
		[]byte(`{}`), now, now, "actor1", "actor1", nil, []byte(`[]`), false)

	labelRows := sqlmock.NewRows([]string{"issue_id", "label"}).
		AddRow("grava-xyz2", "bug").
		AddRow("grava-xyz2", "high")

	commentRows := sqlmock.NewRows([]string{"issue_id", "id", "message", "actor", "agent_model", "created_at"}).
		AddRow("grava-xyz2", 1, "First comment", &actor, nil, now)

	mock.ExpectQuery("SELECT .* FROM issues WHERE").WillReturnRows(issueRows)
	mock.ExpectQuery("SELECT issue_id, label FROM issue_labels").WillReturnRows(labelRows)
	mock.ExpectQuery("SELECT issue_id, id, message").WillReturnRows(commentRows)
	mock.ExpectQuery("SELECT from_id, to_id").
		WillReturnRows(sqlmock.NewRows([]string{"from_id", "to_id", "type", "created_by", "updated_by", "agent_model"}))
	mock.ExpectQuery("SELECT issue_id, key_name").
		WillReturnRows(sqlmock.NewRows([]string{"issue_id", "key_name", "value", "written_by", "written_at"}))

	store := dolt.NewClientFromDB(db)
	var buf bytes.Buffer
	count, err := exportFlatJSONL(context.Background(), store, &buf, false)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var rec IssueJSONLRecord
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	assert.Equal(t, []string{"bug", "high"}, rec.Labels)
	require.Len(t, rec.Comments, 1)
	assert.Equal(t, "First comment", rec.Comments[0].Message)
	assert.Equal(t, "dev1", rec.Comments[0].Actor)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFlatJSONL_SingleIssue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)
	line := `{"id":"grava-f001","title":"Flat Issue","description":"desc","type":"task","priority":2,"status":"open","created_at":"` +
		now.Format(time.RFC3339) + `","updated_at":"` + now.Format(time.RFC3339) + `","created_by":"dev1","updated_by":"dev1"}` + "\n"

	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importFlatJSONL(context.Background(), store, strings.NewReader(line), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFlatJSONL_WithLabelsAndComments(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)
	rec := IssueJSONLRecord{
		ID: "grava-f002", Title: "With Relations", Description: "d", Type: "bug",
		Priority: 1, Status: "open", CreatedAt: now, UpdatedAt: now,
		CreatedBy: "dev1", UpdatedBy: "dev1",
		Labels:   []string{"bug", "high"},
		Comments: []CommentRecord{{ID: 5, Message: "hello", Actor: "dev1", CreatedAt: now}},
	}
	b, _ := json.Marshal(rec)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT IGNORE INTO issue_labels").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT IGNORE INTO issue_labels").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO issue_comments").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importFlatJSONL(context.Background(), store, strings.NewReader(string(b)+"\n"), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFlatJSONL_Overwrite_UpdatesExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)
	line := `{"id":"grava-f003","title":"Updated","description":"d","type":"task","priority":2,"status":"closed","created_at":"` +
		now.Format(time.RFC3339) + `","updated_at":"` + now.Format(time.RFC3339) + `","created_by":"dev1","updated_by":"dev1"}` + "\n"

	mock.ExpectBegin()
	// ON DUPLICATE KEY UPDATE returns RowsAffected=2 for updated rows
	mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(1, 2))
	// overwrite=true clears stale related data
	mock.ExpectExec("DELETE FROM issue_labels").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM issue_comments").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM dependencies").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importFlatJSONL(context.Background(), store, strings.NewReader(line), true)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Imported)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFlatJSONL_WithDependencies(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	now := time.Now().UTC().Truncate(time.Second)
	rec := IssueJSONLRecord{
		ID: "grava-f004", Title: "With Deps", Description: "d", Type: "epic",
		Priority: 1, Status: "open", CreatedAt: now, UpdatedAt: now,
		CreatedBy: "dev1", UpdatedBy: "dev1",
		Dependencies: []DepRecord{
			{FromID: "grava-f004", ToID: "grava-f005", Type: "blocks", CreatedBy: "dev1"},
		},
	}
	b, _ := json.Marshal(rec)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO dependencies").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store := dolt.NewClientFromDB(db)
	result, err := importFlatJSONL(context.Background(), store, strings.NewReader(string(b)+"\n"), false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateJSONL_ValidFlatFormat(t *testing.T) {
	lines := `{"id":"g-001","title":"T1","type":"task","status":"open","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","created_by":"x","updated_by":"x"}` + "\n" +
		`{"id":"g-002","title":"T2","type":"bug","status":"open","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","created_by":"x","updated_by":"x"}` + "\n"
	err := ValidateJSONL(strings.NewReader(lines))
	assert.NoError(t, err)
}

func TestValidateJSONL_MissingID(t *testing.T) {
	line := `{"title":"No ID","type":"task","status":"open"}` + "\n"
	err := ValidateJSONL(strings.NewReader(line))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required field 'id'")
}

func TestValidateJSONL_LegacyWrappedFormat(t *testing.T) {
	line := `{"type":"issue","data":{"id":"grava-001","title":"Test"}}` + "\n"
	err := ValidateJSONL(strings.NewReader(line))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "legacy wrapped format")
	assert.Contains(t, err.Error(), "grava export")
}

func TestValidateJSONL_LegacyWrappedDependency(t *testing.T) {
	line := `{"type":"dependency","data":{"from_id":"a","to_id":"b"}}` + "\n"
	err := ValidateJSONL(strings.NewReader(line))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "legacy wrapped format")
}

func TestValidateJSONL_InvalidJSON(t *testing.T) {
	line := `{not valid json}` + "\n"
	err := ValidateJSONL(strings.NewReader(line))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "line 1")
}

func TestValidateJSONL_EmptyLines(t *testing.T) {
	err := ValidateJSONL(strings.NewReader("\n\n"))
	assert.NoError(t, err)
}

// --- doltHasUncommittedChanges ---

func TestDoltHasUncommittedChanges_TrueWhenRowsExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(3))

	store := dolt.NewClientFromDB(db)
	assert.True(t, doltHasUncommittedChanges(store))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltHasUncommittedChanges_FalseWhenNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

	store := dolt.NewClientFromDB(db)
	assert.False(t, doltHasUncommittedChanges(store))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltHasUncommittedChanges_FalseOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnError(assert.AnError)

	store := dolt.NewClientFromDB(db)
	assert.False(t, doltHasUncommittedChanges(store))
}

// --- newImportCmd command-level tests ---

// makeDeps creates a cmddeps.Deps wired to a mock store for use in command tests.
func makeDeps(store dolt.Store) *cmddeps.Deps {
	actor := "test"
	model := ""
	outputJSON := false
	return &cmddeps.Deps{
		Store:      &store,
		Actor:      &actor,
		AgentModel: &model,
		OutputJSON: &outputJSON,
	}
}

func TestImportCmd_FileNotFound(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	d := makeDeps(dolt.NewClientFromDB(db))
	cmd := newImportCmd(d)
	runErr := cmd.RunE(cmd, []string{"/tmp/does-not-exist-grava-test.jsonl"})
	require.Error(t, runErr)
	assert.ErrorIs(t, runErr, gravaerrors.New("FILE_NOT_FOUND", "", nil))
}

func TestImportCmd_ImportConflict_DoltHasChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// dolt_status returns rows → uncommitted changes present.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))

	f, writeErr := os.CreateTemp(t.TempDir(), "issues-*.jsonl")
	require.NoError(t, writeErr)
	_, _ = f.WriteString(`{"id":"x","title":"T","type":"task","status":"open","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","created_by":"a","updated_by":"a"}` + "\n")
	require.NoError(t, f.Close())

	d := makeDeps(dolt.NewClientFromDB(db))
	cmd := newImportCmd(d)
	cmd.SetContext(context.Background())
	runErr := cmd.RunE(cmd, []string{f.Name()})
	require.Error(t, runErr)
	assert.ErrorIs(t, runErr, gravaerrors.New("IMPORT_CONFLICT", "", nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportCmd_JSONOutput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close() //nolint:errcheck

	// dolt_status returns 0 → no uncommitted changes.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM dolt_status")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

	// importFlatJSONL (overwrite=true): BEGIN, upsert issue, clear stale related data, COMMIT.
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO issues").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM issue_labels").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM issue_comments").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM dependencies").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// Auto-export after import: exportFlatJSONL queries issues, labels, comments, deps, wisps.
	mock.ExpectQuery("SELECT .* FROM issues WHERE").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "issue_type", "priority", "status",
			"metadata", "created_at", "updated_at", "created_by", "updated_by",
			"agent_model", "affected_files", "ephemeral",
		}))

	f, writeErr := os.CreateTemp(t.TempDir(), "issues-*.jsonl")
	require.NoError(t, writeErr)
	_, _ = f.WriteString(`{"id":"x","title":"T","type":"task","status":"open","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","created_by":"a","updated_by":"a"}` + "\n")
	require.NoError(t, f.Close())

	d := makeDeps(dolt.NewClientFromDB(db))
	*d.OutputJSON = true

	var outBuf bytes.Buffer
	cmd := newImportCmd(d)
	cmd.SetContext(context.Background())
	cmd.SetOut(&outBuf)
	runErr := cmd.RunE(cmd, []string{f.Name()})
	require.NoError(t, runErr)

	var out map[string]int
	require.NoError(t, json.Unmarshal(outBuf.Bytes(), &out))
	assert.Equal(t, 1, out["imported"])
	assert.Equal(t, 0, out["updated"])
	assert.Equal(t, 0, out["skipped"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

