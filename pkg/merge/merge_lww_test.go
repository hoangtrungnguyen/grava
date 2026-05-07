package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessMergeWithLWW_NoConflict verifies non-conflicting field changes produce
// a clean merge with no conflict records or git conflict.
func TestProcessMergeWithLWW_NoConflict(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"A","status":"closed"}`
	other := `{"id":"1","title":"B","status":"open"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict)
	assert.Empty(t, res.ConflictRecords)
	assert.Contains(t, res.Merged, `"title":"B"`)
	assert.Contains(t, res.Merged, `"status":"closed"`)
}

// TestProcessMergeWithLWW_LWW_NewerCurrentWins verifies that when current has a
// newer updated_at, it wins the field conflict.
func TestProcessMergeWithLWW_LWW_NewerCurrentWins(t *testing.T) {
	ancestor := `{"id":"1","title":"A","updated_at":"2026-01-01T10:00:00Z"}`
	current := `{"id":"1","title":"X","updated_at":"2026-01-01T12:00:00Z"}`
	other := `{"id":"1","title":"Y","updated_at":"2026-01-01T11:00:00Z"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict, "LWW should resolve conflict")
	assert.Empty(t, res.ConflictRecords)
	assert.Contains(t, res.Merged, `"title":"X"`, "current (newer) should win")
}

// TestProcessMergeWithLWW_LWW_NewerOtherWins verifies that when other has a
// newer updated_at, it wins the field conflict.
func TestProcessMergeWithLWW_LWW_NewerOtherWins(t *testing.T) {
	ancestor := `{"id":"1","title":"A","updated_at":"2026-01-01T10:00:00Z"}`
	current := `{"id":"1","title":"X","updated_at":"2026-01-01T11:00:00Z"}`
	other := `{"id":"1","title":"Y","updated_at":"2026-01-01T12:00:00Z"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict, "LWW should resolve conflict")
	assert.Empty(t, res.ConflictRecords)
	assert.Contains(t, res.Merged, `"title":"Y"`, "other (newer) should win")
}

// TestProcessMergeWithLWW_EqualTimestamps_WritesConflictRecord verifies that
// equal updated_at timestamps produce a git conflict and a ConflictRecord.
func TestProcessMergeWithLWW_EqualTimestamps_WritesConflictRecord(t *testing.T) {
	ancestor := `{"id":"1","title":"A","updated_at":"2026-01-01T10:00:00Z"}`
	current := `{"id":"1","title":"X","updated_at":"2026-01-01T10:00:00Z"}`
	other := `{"id":"1","title":"Y","updated_at":"2026-01-01T10:00:00Z"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.True(t, res.HasGitConflict, "equal timestamps should not resolve via LWW")
	require.Len(t, res.ConflictRecords, 1)
	assert.Equal(t, "1", res.ConflictRecords[0].IssueID)
	assert.Equal(t, "title", res.ConflictRecords[0].Field)
	assert.Contains(t, res.Merged, `"_conflict":true`)
}

// TestProcessMergeWithLWW_NoTimestamp_WritesConflictRecord verifies that missing
// updated_at fields fall back to conflict behavior.
func TestProcessMergeWithLWW_NoTimestamp_WritesConflictRecord(t *testing.T) {
	ancestor := `{"id":"1","title":"A"}`
	current := `{"id":"1","title":"X"}`
	other := `{"id":"1","title":"Y"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.True(t, res.HasGitConflict)
	assert.Len(t, res.ConflictRecords, 1)
	assert.Contains(t, res.Merged, `"_conflict":true`)
}

// TestProcessMergeWithLWW_DeleteWins_CurrentDeleted verifies that when current
// deletes an issue that other modified, the delete wins, no git conflict,
// but a ConflictRecord is written.
func TestProcessMergeWithLWW_DeleteWins_CurrentDeleted(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `` // deleted
	other := `{"id":"1","title":"A","status":"closed"}` // modified

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict, "delete-wins is deterministic, not a git conflict")
	require.Len(t, res.ConflictRecords, 1, "delete-wins should produce one conflict record")
	assert.Equal(t, "1", res.ConflictRecords[0].IssueID)
	assert.Equal(t, "", res.ConflictRecords[0].Field, "issue-level conflict has empty field")
	assert.Equal(t, "null", string(res.ConflictRecords[0].Local))
	assert.NotContains(t, res.Merged, `"id":"1"`, "issue should not appear in merged output")
}

// TestProcessMergeWithLWW_DeleteWins_OtherDeleted verifies the symmetric case:
// other deletes, current modified — delete wins.
func TestProcessMergeWithLWW_DeleteWins_OtherDeleted(t *testing.T) {
	ancestor := `{"id":"1","title":"A"}`
	current := `{"id":"1","title":"changed"}` // modified
	other := ``                                // deleted

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict)
	require.Len(t, res.ConflictRecords, 1)
	assert.Equal(t, "null", string(res.ConflictRecords[0].Remote))
	assert.NotContains(t, res.Merged, `"id":"1"`, "issue should not appear in merged output")
}

// TestProcessMergeWithLWW_CleanDeleteWins verifies that when the other side
// did not change the issue and current deletes it, no conflict record is produced.
func TestProcessMergeWithLWW_CleanDeleteWins(t *testing.T) {
	ancestor := `{"id":"1","title":"A"}`
	current := `` // deleted
	other := `{"id":"1","title":"A"}` // unchanged

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict)
	assert.Empty(t, res.ConflictRecords, "unchanged other side means clean delete, no conflict record")
	assert.NotContains(t, res.Merged, `"id":"1"`)
}

// TestProcessMergeWithLWW_AddedBothSides_SameField_NoTimestamp checks that two
// branches independently adding the same issue ID with conflicting field values
// results in a conflict record when timestamps are absent.
func TestProcessMergeWithLWW_AddedBothSides_SameField_NoTimestamp(t *testing.T) {
	ancestor := ``
	current := `{"id":"new","title":"From current"}`
	other := `{"id":"new","title":"From other"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.True(t, res.HasGitConflict)
	assert.Len(t, res.ConflictRecords, 1)
	assert.Contains(t, res.Merged, `"_conflict":true`)
}

// TestProcessMergeWithLWW_AddedBothSides_DifferentIDs verifies two branches
// each adding a distinct issue produces a clean merge with no conflicts.
func TestProcessMergeWithLWW_AddedBothSides_DifferentIDs(t *testing.T) {
	ancestor := ``
	current := `{"id":"issue-1","title":"First"}`
	other := `{"id":"issue-2","title":"Second"}`

	res, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	assert.False(t, res.HasGitConflict)
	assert.Empty(t, res.ConflictRecords)
	assert.Contains(t, res.Merged, `"id":"issue-1"`)
	assert.Contains(t, res.Merged, `"id":"issue-2"`)
}

// TestProcessMergeWithLWW_OneSidedTimestamp_SideWithTimestampWins verifies that
// when only one branch has an updated_at, that branch wins the field conflict.
func TestProcessMergeWithLWW_OneSidedTimestamp_SideWithTimestampWins(t *testing.T) {
	t.Run("only current has timestamp — current wins", func(t *testing.T) {
		ancestor := `{"id":"1","title":"A"}`
		current := `{"id":"1","title":"X","updated_at":"2026-01-01T12:00:00Z"}`
		other := `{"id":"1","title":"Y"}` // no timestamp

		res, err := ProcessMergeWithLWW(ancestor, current, other)
		require.NoError(t, err)
		assert.False(t, res.HasGitConflict, "one-sided timestamp: current wins")
		assert.Empty(t, res.ConflictRecords)
		assert.Contains(t, res.Merged, `"title":"X"`)
	})

	t.Run("only other has timestamp — other wins", func(t *testing.T) {
		ancestor := `{"id":"1","title":"A"}`
		current := `{"id":"1","title":"X"}` // no timestamp
		other := `{"id":"1","title":"Y","updated_at":"2026-01-01T12:00:00Z"}`

		res, err := ProcessMergeWithLWW(ancestor, current, other)
		require.NoError(t, err)
		assert.False(t, res.HasGitConflict, "one-sided timestamp: other wins")
		assert.Empty(t, res.ConflictRecords)
		assert.Contains(t, res.Merged, `"title":"Y"`)
	})
}

// TestProcessMergeWithLWW_ConflictRecordHasDeterministicID verifies that the
// conflict record ID is stable across calls with the same inputs.
func TestProcessMergeWithLWW_ConflictRecordHasDeterministicID(t *testing.T) {
	ancestor := `{"id":"1","title":"A"}`
	current := `{"id":"1","title":"X"}`
	other := `{"id":"1","title":"Y"}`

	res1, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	res2, err := ProcessMergeWithLWW(ancestor, current, other)
	require.NoError(t, err)
	require.Len(t, res1.ConflictRecords, 1)
	assert.Equal(t, res1.ConflictRecords[0].ID, res2.ConflictRecords[0].ID)
}
