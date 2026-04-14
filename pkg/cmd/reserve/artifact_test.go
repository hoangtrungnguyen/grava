package reserve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var qGetReservation = regexp.QuoteMeta(
	`SELECT id, project_id, agent_id, path_pattern, exclusive, COALESCE(reason,''), created_ts, expires_ts, released_ts`,
)

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
