package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessMerge_NoConflict(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"A","status":"closed"}`
	other := `{"id":"1","title":"B","status":"open"}`

	// current changed status; other changed title — no overlap
	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.Contains(t, merged, `"title":"B"`)
	assert.Contains(t, merged, `"status":"closed"`)
}

func TestProcessMerge_Conflict(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"X","status":"open"}`
	other := `{"id":"1","title":"Y","status":"open"}`

	// both changed title — conflict
	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.True(t, hasConflict)
	assert.Contains(t, merged, `"_conflict":true`)
	assert.Contains(t, merged, `"local":"X"`)
	assert.Contains(t, merged, `"remote":"Y"`)
}

func TestProcessMerge_AddedInBoth(t *testing.T) {
	ancestor := ``
	current := `{"id":"2","title":"New from current"}`
	other := `{"id":"3","title":"New from other"}`

	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.Contains(t, merged, `"id":"2"`)
	assert.Contains(t, merged, `"id":"3"`)
}

func TestProcessMerge_DeletedInCurrent(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := ``
	other := `{"id":"1","title":"A","status":"open"}`

	// current deleted, other unchanged — delete wins
	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.NotContains(t, merged, `"id":"1"`)
}

func TestProcessMerge_DeleteConflict(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := ``
	other := `{"id":"1","title":"A","status":"closed"}`

	// current deleted, other modified — conflict
	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.True(t, hasConflict)
	assert.Contains(t, merged, `"_conflict":true`)
}

func TestProcessMerge_BothDeleted(t *testing.T) {
	ancestor := `{"id":"1","title":"A"}`
	current := ``
	other := ``

	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.Empty(t, merged)
}

func TestProcessMerge_SameChangesBothSides(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"A","status":"closed"}`
	other := `{"id":"1","title":"A","status":"closed"}`

	// same change both sides — no conflict
	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.Contains(t, merged, `"status":"closed"`)
}
