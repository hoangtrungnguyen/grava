package merge

import (
	"testing"
	"time"

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

// --- ExtractConflicts ---

func TestExtractConflicts_Empty(t *testing.T) {
	entries, err := ExtractConflicts("", time.Now())
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExtractConflicts_FieldLevelConflict(t *testing.T) {
	merged := `{"id":"issue-1","status":"open","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	entries, err := ExtractConflicts(merged, time.Now())
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "issue-1", entries[0].IssueID)
	assert.Equal(t, "title", entries[0].Field)
	assert.Equal(t, `"X"`, string(entries[0].Local))
	assert.Equal(t, `"Y"`, string(entries[0].Remote))
	assert.NotEmpty(t, entries[0].ID)
}

func TestExtractConflicts_MultipleFields(t *testing.T) {
	// Two conflicted fields on the same issue — both extracted.
	merged := `{"id":"issue-2","a":{"_conflict":true,"local":"A1","remote":"A2"},"b":{"_conflict":true,"local":"B1","remote":"B2"}}` + "\n"
	entries, err := ExtractConflicts(merged, time.Now())
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
	// Output is sorted by field name.
	assert.Equal(t, "a", entries[0].Field)
	assert.Equal(t, "b", entries[1].Field)
}

func TestExtractConflicts_IssueLevelDeleteConflict(t *testing.T) {
	// Top-level _conflict:true — whole-issue conflict.
	merged := `{"_conflict":true,"id":"issue-3","local":null,"remote":{"id":"issue-3","title":"changed"}}` + "\n"
	entries, err := ExtractConflicts(merged, time.Now())
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "issue-3", entries[0].IssueID)
	assert.Equal(t, "", entries[0].Field) // empty field = whole-issue
	assert.Equal(t, "null", string(entries[0].Local))
}

func TestExtractConflicts_MixedIssues(t *testing.T) {
	// One clean issue and one with a field conflict — only the conflict is returned.
	merged := `{"id":"issue-1","title":"clean"}` + "\n" +
		`{"id":"issue-2","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	entries, err := ExtractConflicts(merged, time.Now())
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "issue-2", entries[0].IssueID)
}

func TestExtractConflicts_SortedByIssueIDThenField(t *testing.T) {
	// Conflicts across multiple issues — output sorted by issue_id then field.
	merged := `{"id":"issue-b","z":{"_conflict":true,"local":1,"remote":2}}` + "\n" +
		`{"id":"issue-a","z":{"_conflict":true,"local":3,"remote":4},"a":{"_conflict":true,"local":5,"remote":6}}` + "\n"
	entries, err := ExtractConflicts(merged, time.Now())
	assert.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, "issue-a", entries[0].IssueID)
	assert.Equal(t, "a", entries[0].Field)
	assert.Equal(t, "issue-a", entries[1].IssueID)
	assert.Equal(t, "z", entries[1].Field)
	assert.Equal(t, "issue-b", entries[2].IssueID)
}

func TestExtractConflicts_IDIsDeterministic(t *testing.T) {
	// The conflict ID must be stable: same issue+field always yields same ID.
	merged := `{"id":"issue-1","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	e1, _ := ExtractConflicts(merged, time.Now())
	e2, _ := ExtractConflicts(merged, time.Now())
	assert.Equal(t, e1[0].ID, e2[0].ID)
}

// TestMarshalSorted verifies the internal helper directly.
func TestMarshalSorted_NestedMaps(t *testing.T) {
	input := map[string]interface{}{
		"z": map[string]interface{}{"b": 2, "a": 1},
		"a": "first",
	}
	b, err := MarshalSorted(input)
	assert.NoError(t, err)
	// Outer keys sorted: a, z. Inner keys sorted: a, b.
	assert.Equal(t, `{"a":"first","z":{"a":1,"b":2}}`, string(b))
}
