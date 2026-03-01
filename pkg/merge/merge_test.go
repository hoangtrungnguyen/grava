package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessMerge(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"A","status":"closed"}`
	other := `{"id":"1","title":"B","status":"open"}`

	// Current changed status to closed. Other changed title to B.
	// Merged should have title B and status closed.

	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.False(t, hasConflict)
	assert.Contains(t, merged, `"title":"B"`)
	assert.Contains(t, merged, `"status":"closed"`)
}

func TestProcessMergeConflict(t *testing.T) {
	ancestor := `{"id":"1","title":"A","status":"open"}`
	current := `{"id":"1","title":"X","status":"open"}`
	other := `{"id":"1","title":"Y","status":"open"}`

	// Both changed title. Conflict!

	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.True(t, hasConflict)
	assert.Contains(t, merged, `"title":{"_conflict":true,"local":"X","remote":"Y"}`)
}
