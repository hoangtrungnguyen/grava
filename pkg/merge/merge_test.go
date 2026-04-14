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

func TestProcessMerge_SameIDAddedInBothWithConflict(t *testing.T) {
	// Both branches add the same new issue ID with different field values.
	// ancestor has no issue "5"; current and other both add it with differing titles.
	ancestor := ``
	current := `{"id":"5","title":"From current","status":"open"}`
	other := `{"id":"5","title":"From other","status":"open"}`

	merged, hasConflict, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	assert.True(t, hasConflict)
	assert.Contains(t, merged, `"_conflict":true`)
	assert.Contains(t, merged, `"local":"From current"`)
	assert.Contains(t, merged, `"remote":"From other"`)
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

func TestProcessMerge_OutputIsSortedAlphabetically(t *testing.T) {
	// Fields appear in reverse alphabetical order in the input to ensure we
	// test that the output is sorted, not just passthrough.
	ancestor := `{"id":"1","zzz":"z","aaa":"a","mmm":"m"}`
	current := ancestor
	other := ancestor

	merged, _, err := ProcessMerge(ancestor, current, other)
	assert.NoError(t, err)
	// Expect keys to appear in sorted order: aaa, id, mmm, zzz
	assert.Equal(t, `{"aaa":"a","id":"1","mmm":"m","zzz":"z"}`+"\n", merged)
}

func TestProcessMerge_DeterministicAcrossRuns(t *testing.T) {
	// Run ProcessMerge multiple times on the same input to verify output is
	// byte-for-byte identical regardless of map iteration order.
	ancestor := `{"id":"1","alpha":"a","beta":"b","gamma":"g","delta":"d"}`
	current := `{"id":"1","alpha":"a","beta":"b","gamma":"g","delta":"d","epsilon":"e"}`
	other := ancestor

	const runs = 20
	var first string
	for i := 0; i < runs; i++ {
		merged, _, err := ProcessMerge(ancestor, current, other)
		assert.NoError(t, err)
		if i == 0 {
			first = merged
		} else {
			assert.Equal(t, first, merged, "ProcessMerge output must be identical on run %d", i+1)
		}
	}
}

// TestMarshalSorted verifies the internal helper directly.
func TestMarshalSorted_NestedMaps(t *testing.T) {
	input := map[string]interface{}{
		"z": map[string]interface{}{"b": 2, "a": 1},
		"a": "first",
	}
	b, err := marshalSorted(input)
	assert.NoError(t, err)
	// Outer keys sorted: a, z. Inner keys sorted: a, b.
	assert.Equal(t, `{"a":"first","z":{"a":1,"b":2}}`, string(b))
}
