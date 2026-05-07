package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetConflictsFlags resets global flags that may leak between conflict tests.
func resetConflictsFlags(t *testing.T) {
	t.Helper()
	outputJSON = false
	t.Cleanup(func() { outputJSON = false })
}

// sampleConflictEntries returns two sample ConflictEntry values for testing.
func sampleConflictEntries(t *testing.T) []merge.ConflictEntry {
	t.Helper()
	localV, _ := json.Marshal("X")
	remoteV, _ := json.Marshal("Y")
	return []merge.ConflictEntry{
		{
			ID:         "aabb1122",
			IssueID:    "issue-42",
			Field:      "title",
			Local:      json.RawMessage(localV),
			Remote:     json.RawMessage(remoteV),
			DetectedAt: time.Now().UTC(),
			Resolved:   false,
		},
	}
}

// --- conflicts list ---

func TestConflictsListCmd_NoPendingConflicts(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	require.NoError(t, os.MkdirAll(".grava", 0755))

	var buf bytes.Buffer
	conflictsListCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsListCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No pending conflicts")
}

func TestConflictsListCmd_ShowsPendingEntries(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	var buf bytes.Buffer
	conflictsListCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsListCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "aabb1122")
	assert.Contains(t, out, "issue-42")
	assert.Contains(t, out, "title")
}

func TestConflictsListCmd_JSONOutput(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()
	t.Cleanup(func() { outputJSON = false })

	writeConflictsFile(t, sampleConflictEntries(t))

	var buf bytes.Buffer
	conflictsListCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsListCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "--json", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	var entries []merge.ConflictEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "aabb1122", entries[0].ID)
}

func TestConflictsListCmd_JSONOutput_EmptyIsArray(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()
	t.Cleanup(func() { outputJSON = false })

	require.NoError(t, os.MkdirAll(".grava", 0755))

	var buf bytes.Buffer
	conflictsListCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsListCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "--json", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	// Must be a valid JSON array, not null.
	trimmed := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(trimmed, "["), "empty JSON output must be an array")
}

// --- conflicts resolve ---

func TestConflictsResolveCmd_MissingAcceptFlag(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	rootCmd.SetArgs([]string{"conflicts", "resolve", "aabb1122"})
	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestConflictsResolveCmd_InvalidAccept(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	rootCmd.SetArgs([]string{"conflicts", "resolve", "aabb1122", "--accept", "both"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--accept must be")
}

func TestConflictsResolveCmd_ConflictNotFound(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	rootCmd.SetArgs([]string{"conflicts", "resolve", "doesnotexist", "--accept", "ours"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestConflictsResolveCmd_AcceptOurs(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	issuesLine := `{"id":"issue-42","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesLine), 0644))

	rootCmd.SetArgs([]string{"conflicts", "resolve", "aabb1122", "--accept", "ours"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile("issues.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(result), `"title":"X"`, "ours = local value")
	assert.NotContains(t, string(result), `"_conflict"`)
}

func TestConflictsResolveCmd_AcceptTheirs(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	issuesLine := `{"id":"issue-42","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesLine), 0644))

	rootCmd.SetArgs([]string{"conflicts", "resolve", "aabb1122", "--accept", "theirs"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile("issues.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(result), `"title":"Y"`, "theirs = remote value")
	assert.NotContains(t, string(result), `"_conflict"`)
}

func TestConflictsResolveCmd_JSONOutput(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()
	t.Cleanup(func() { outputJSON = false })

	writeConflictsFile(t, sampleConflictEntries(t))
	issuesLine := `{"id":"issue-42","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesLine), 0644))

	var buf bytes.Buffer
	conflictsResolveCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsResolveCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "--json", "resolve", "aabb1122", "--accept", "ours"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.Equal(t, "aabb1122", resp["id"])
	assert.Equal(t, "resolved", resp["status"])
	assert.Equal(t, "ours", resp["resolution"])
}

func TestConflictsResolveCmd_AlreadyResolved(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	localV, _ := json.Marshal("X")
	remoteV, _ := json.Marshal("Y")
	entries := []merge.ConflictEntry{{
		ID: "done9999", IssueID: "issue-99", Field: "title",
		Local: json.RawMessage(localV), Remote: json.RawMessage(remoteV),
		Resolved: true,
	}}
	writeConflictsFile(t, entries)

	var buf bytes.Buffer
	conflictsResolveCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsResolveCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "resolve", "done9999", "--accept", "ours"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "already resolved")
}

// --- conflicts dismiss ---

func TestConflictsDismissCmd_MarksResolved(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	rootCmd.SetArgs([]string{"conflicts", "dismiss", "aabb1122"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	b, err := os.ReadFile(".grava/conflicts.json")
	require.NoError(t, err)
	var updated []merge.ConflictEntry
	require.NoError(t, json.Unmarshal(b, &updated))
	assert.True(t, updated[0].Resolved, "dismissed entry must be marked resolved")
}

func TestConflictsDismissCmd_NotFound(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflictEntries(t))

	rootCmd.SetArgs([]string{"conflicts", "dismiss", "nonexistent"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestConflictsDismissCmd_JSONOutput(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()
	t.Cleanup(func() { outputJSON = false })

	writeConflictsFile(t, sampleConflictEntries(t))

	var buf bytes.Buffer
	conflictsDismissCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsDismissCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "--json", "dismiss", "aabb1122"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.Equal(t, "aabb1122", resp["id"])
	assert.Equal(t, "dismissed", resp["status"])
}

func TestConflictsDismissCmd_AlreadyDismissed(t *testing.T) {
	resetConflictsFlags(t)
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	localV, _ := json.Marshal("X")
	remoteV, _ := json.Marshal("Y")
	entries := []merge.ConflictEntry{{
		ID: "gone5555", IssueID: "issue-88", Field: "status",
		Local: json.RawMessage(localV), Remote: json.RawMessage(remoteV),
		Resolved: true,
	}}
	writeConflictsFile(t, entries)

	var buf bytes.Buffer
	conflictsDismissCmd.SetOut(&buf)
	t.Cleanup(func() { conflictsDismissCmd.SetOut(nil) })

	rootCmd.SetArgs([]string{"conflicts", "dismiss", "gone5555"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "already resolved")
}
