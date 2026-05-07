package reserve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var qGetReservation = `SELECT id, project_id, agent_id, path_pattern, .exclusive., COALESCE`

// tmpGravaDir creates a temporary directory that acts as .grava/ for a test.
func tmpGravaDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// --- WriteReservationArtifact ---

func TestWriteReservationArtifact_CreatesFile(t *testing.T) {
	gravaDir := tmpGravaDir(t)
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Minute)

	r := Reservation{
		ID:          "res-abc123",
		ProjectID:   "default",
		AgentID:     "agent-01",
		PathPattern: "src/cmd/issues/*.go",
		Exclusive:   true,
		CreatedTS:   now,
		ExpiresTS:   exp,
	}

	require.NoError(t, WriteReservationArtifact(gravaDir, r))

	// File must exist at the expected path.
	path := artifactPath(gravaDir, r.PathPattern)
	b, err := os.ReadFile(path)
	require.NoError(t, err)

	var got Reservation
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, r.AgentID, got.AgentID)
	assert.Equal(t, r.PathPattern, got.PathPattern)
	assert.True(t, got.Exclusive)
	assert.Nil(t, got.ReleasedTS, "released_ts must be absent for active reservation")
}

func TestWriteReservationArtifact_UpdatesReleasedTS(t *testing.T) {
	gravaDir := tmpGravaDir(t)
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Minute)

	r := Reservation{
		ID:          "res-abc123",
		ProjectID:   "default",
		AgentID:     "agent-01",
		PathPattern: "src/cmd/issues/*.go",
		Exclusive:   true,
		CreatedTS:   now,
		ExpiresTS:   exp,
	}

	// Write initial artifact (no released_ts).
	require.NoError(t, WriteReservationArtifact(gravaDir, r))

	// Simulate release: set released_ts and overwrite.
	released := now.Add(5 * time.Minute)
	r.ReleasedTS = &released
	require.NoError(t, WriteReservationArtifact(gravaDir, r))

	b, err := os.ReadFile(artifactPath(gravaDir, r.PathPattern))
	require.NoError(t, err)

	var got Reservation
	require.NoError(t, json.Unmarshal(b, &got))
	require.NotNil(t, got.ReleasedTS, "released_ts must be set after release")
	assert.True(t, got.ReleasedTS.Equal(released))
}

func TestWriteReservationArtifact_DifferentPathPatternsDifferentFiles(t *testing.T) {
	gravaDir := tmpGravaDir(t)
	now := time.Now().UTC()
	exp := now.Add(30 * time.Minute)

	r1 := Reservation{ID: "res-1", AgentID: "a", PathPattern: "src/foo.go", CreatedTS: now, ExpiresTS: exp}
	r2 := Reservation{ID: "res-2", AgentID: "a", PathPattern: "src/bar.go", CreatedTS: now, ExpiresTS: exp}

	require.NoError(t, WriteReservationArtifact(gravaDir, r1))
	require.NoError(t, WriteReservationArtifact(gravaDir, r2))

	path1 := artifactPath(gravaDir, r1.PathPattern)
	path2 := artifactPath(gravaDir, r2.PathPattern)
	assert.NotEqual(t, path1, path2, "different path patterns must produce different artifact paths")

	// Both files must exist.
	_, err1 := os.Stat(path1)
	_, err2 := os.Stat(path2)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

// --- GetReservation ---

func TestGetReservation_Found(t *testing.T) {
	store, mock := newMock(t)
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Minute)

	mock.ExpectQuery(qGetReservation).
		WithArgs("res-xyz").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
			"created_ts", "expires_ts", "released_ts",
		}).AddRow("res-xyz", "default", "agent-01", "src/*.go", true, "", now, exp, nil))

	r, err := GetReservation(context.Background(), store, "res-xyz")
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "res-xyz", r.ID)
	assert.Equal(t, "agent-01", r.AgentID)
	assert.Nil(t, r.ReleasedTS)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReservation_NotFound(t *testing.T) {
	store, mock := newMock(t)
	mock.ExpectQuery(qGetReservation).
		WithArgs("res-missing").
		WillReturnRows(sqlmock.NewRows([]string{}))

	_, err := GetReservation(context.Background(), store, "res-missing")
	require.Error(t, err)
	var gerr *gravaerrors.GravaError
	require.True(t, errors.As(err, &gerr), "error must be a GravaError")
	assert.Equal(t, "RESERVATION_NOT_FOUND", gerr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- integration-style: full create → write artifact → release → update artifact ---

func TestArtifact_IntegrationFlow(t *testing.T) {
	gravaDir := tmpGravaDir(t)
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Minute)

	// Step 1: declare creates a reservation in DB.
	r := Reservation{
		ID:          "res-integ-01",
		ProjectID:   "default",
		AgentID:     "agent-test",
		PathPattern: "pkg/cmd/**/*.go",
		Exclusive:   false,
		CreatedTS:   now,
		ExpiresTS:   exp,
	}

	// Write artifact (simulates what RunE does after DeclareReservation).
	require.NoError(t, WriteReservationArtifact(gravaDir, r))

	// Step 2: inspect artifact — no released_ts.
	path := artifactPath(gravaDir, r.PathPattern)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var got1 Reservation
	require.NoError(t, json.Unmarshal(b, &got1))
	assert.Equal(t, "res-integ-01", got1.ID)
	assert.Nil(t, got1.ReleasedTS, "artifact must not have released_ts before release")

	// Step 3: release — update artifact with released_ts.
	releasedAt := now.Add(10 * time.Minute)
	r.ReleasedTS = &releasedAt
	require.NoError(t, WriteReservationArtifact(gravaDir, r))

	// Step 4: verify released_ts in artifact.
	b2, err := os.ReadFile(path)
	require.NoError(t, err)
	var got2 Reservation
	require.NoError(t, json.Unmarshal(b2, &got2))
	require.NotNil(t, got2.ReleasedTS, "released_ts must be set in artifact after release")
	assert.True(t, got2.ReleasedTS.Equal(releasedAt), "released_ts value must match")

	// Step 5: ensure artifact directory exists under gravaDir.
	info, err := os.Stat(filepath.Join(gravaDir, "file_reservations"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// --- cmd-level test for --release artifact update ---

// buildReleaseCmd creates a cobra command wired to the given store and gravaDir.
func buildReleaseCmd(t *testing.T, store dolt.Store, gravaDir string) *cobra.Command {
	t.Helper()
	outputJSON := false
	actor := "test-actor"
	agentModel := ""
	d := &cmddeps.Deps{Store: &store, OutputJSON: &outputJSON, Actor: &actor, AgentModel: &agentModel}
	root := &cobra.Command{Use: "grava"}
	AddCommands(root, d)
	// Point GRAVA_DIR to our temp dir so ResolveGravaDir returns it.
	t.Setenv("GRAVA_DIR", gravaDir)
	return root
}

// reserveColumns returns the sqlmock column set for file_reservations SELECT.
func reserveColumns() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "project_id", "agent_id", "path_pattern", "exclusive", "reason",
		"created_ts", "expires_ts", "released_ts",
	})
}

// TestReleaseCmd_WritesArtifactWithReleasedTS exercises the full --release RunE path:
// GetReservation (pre-release) → ReleaseReservation UPDATE → GetReservation (re-fetch)
// → WriteReservationArtifact. Verifies the artifact has released_ts set after the command.
func TestReleaseCmd_WritesArtifactWithReleasedTS(t *testing.T) {
	store, mock := newMock(t)
	gravaDir := tmpGravaDir(t)

	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(30 * time.Minute)
	releasedAt := now.Add(5 * time.Minute)
	pathPattern := "src/cmd/reserve/*.go"

	// Pre-release fetch: released_ts is NULL.
	mock.ExpectQuery(qGetReservation).
		WithArgs("res-cmd-test").
		WillReturnRows(reserveColumns().
			AddRow("res-cmd-test", "default", "agent-01", pathPattern, false, "", now, exp, nil))

	// ReleaseReservation UPDATE.
	mock.ExpectExec(qReleaseQuery).
		WithArgs("res-cmd-test").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Re-fetch after release: released_ts is now set.
	mock.ExpectQuery(qGetReservation).
		WithArgs("res-cmd-test").
		WillReturnRows(reserveColumns().
			AddRow("res-cmd-test", "default", "agent-01", pathPattern, false, "", now, exp, releasedAt))

	root := buildReleaseCmd(t, store, gravaDir)
	root.SetArgs([]string{"reserve", "--release", "res-cmd-test"})
	require.NoError(t, root.Execute())

	// Artifact must exist with released_ts matching the re-fetched DB value.
	b, err := os.ReadFile(artifactPath(gravaDir, pathPattern))
	require.NoError(t, err)

	var got Reservation
	require.NoError(t, json.Unmarshal(b, &got))
	require.NotNil(t, got.ReleasedTS, "released_ts must be set in artifact after --release")
	assert.True(t, got.ReleasedTS.Equal(releasedAt), "artifact released_ts must match re-fetched DB value")

	assert.NoError(t, mock.ExpectationsWereMet())
}
