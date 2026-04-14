package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/merge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConflictsFile writes a conflicts.json into a .grava dir under the CWD.
func writeConflictsFile(t *testing.T, entries []merge.ConflictEntry) string {
	t.Helper()
	gravaDir := ".grava"
	require.NoError(t, os.MkdirAll(gravaDir, 0755))
	path := filepath.Join(gravaDir, "conflicts.json")
	b, err := json.MarshalIndent(entries, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0644))
	return path
}

func sampleConflicts(t *testing.T) []merge.ConflictEntry {
	t.Helper()
	local, _ := json.Marshal("X")
	remote, _ := json.Marshal("Y")
	return []merge.ConflictEntry{
		{
			ID:         "aabbccdd",
			IssueID:    "issue-1",
			Field:      "title",
			Local:      json.RawMessage(local),
			Remote:     json.RawMessage(remote),
			DetectedAt: time.Now().UTC(),
			Resolved:   false,
		},
	}
}

// --- grava resolve list ---

func TestResolveListCmd_NoConflictsFile(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// .grava dir must exist for ResolveGravaDir; no conflicts.json inside.
	require.NoError(t, os.MkdirAll(".grava", 0755))

	var buf bytes.Buffer
	resolveListCmd.SetOut(&buf)
	defer resolveListCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"resolve", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No pending conflicts")
}

func TestResolveListCmd_WithConflicts(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	entries := sampleConflicts(t)
	writeConflictsFile(t, entries)

	var buf bytes.Buffer
	resolveListCmd.SetOut(&buf)
	defer resolveListCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"resolve", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "aabbccdd")
	assert.Contains(t, out, "issue-1")
	assert.Contains(t, out, "title")
}

func TestResolveListCmd_ResolvedConflictsHidden(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	local, _ := json.Marshal("X")
	remote, _ := json.Marshal("Y")
	entries := []merge.ConflictEntry{
		{
			ID:       "resolved11",
			IssueID:  "issue-2",
			Field:    "status",
			Local:    json.RawMessage(local),
			Remote:   json.RawMessage(remote),
			Resolved: true,
		},
	}
	writeConflictsFile(t, entries)

	var buf bytes.Buffer
	resolveListCmd.SetOut(&buf)
	defer resolveListCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"resolve", "list"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No pending conflicts")
}

// --- grava resolve pick ---

func TestResolvePickCmd_MissingChoiceFlag(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflicts(t))

	rootCmd.SetArgs([]string{"resolve", "pick", "aabbccdd"})
	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestResolvePickCmd_InvalidChoice(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflicts(t))

	rootCmd.SetArgs([]string{"resolve", "pick", "aabbccdd", "--choice", "both"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--choice must be")
}

func TestResolvePickCmd_ConflictNotFound(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeConflictsFile(t, sampleConflicts(t))

	rootCmd.SetArgs([]string{"resolve", "pick", "nonexistent", "--choice", "local"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePickCmd_AlreadyResolved(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	local, _ := json.Marshal("X")
	remote, _ := json.Marshal("Y")
	entries := []merge.ConflictEntry{
		{
			ID:       "done1111",
			IssueID:  "issue-3",
			Field:    "title",
			Local:    json.RawMessage(local),
			Remote:   json.RawMessage(remote),
			Resolved: true,
		},
	}
	writeConflictsFile(t, entries)
	// issues.jsonl not needed since conflict is already resolved
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(`{"id":"issue-3","title":"X"}`+"\n"), 0644))

	var buf bytes.Buffer
	resolvePickCmd.SetOut(&buf)
	defer resolvePickCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"resolve", "pick", "done1111", "--choice", "local"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "already resolved")
}

func TestResolvePickCmd_PickLocal(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	entries := sampleConflicts(t) // local="X", remote="Y"
	writeConflictsFile(t, entries)

	issuesLine := `{"id":"issue-1","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesLine), 0644))

	var buf bytes.Buffer
	resolvePickCmd.SetOut(&buf)
	defer resolvePickCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"resolve", "pick", "aabbccdd", "--choice", "local"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// issues.jsonl should now have "title":"X"
	result, err := os.ReadFile("issues.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(result), `"title":"X"`)
	assert.NotContains(t, string(result), `"_conflict"`)

	// conflicts.json should mark the entry as resolved
	b, err := os.ReadFile(".grava/conflicts.json")
	require.NoError(t, err)
	var updated []merge.ConflictEntry
	require.NoError(t, json.Unmarshal(b, &updated))
	assert.True(t, updated[0].Resolved)
}

func TestResolvePickCmd_PickRemote(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	entries := sampleConflicts(t) // local="X", remote="Y"
	writeConflictsFile(t, entries)

	issuesLine := `{"id":"issue-1","title":{"_conflict":true,"local":"X","remote":"Y"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(issuesLine), 0644))

	rootCmd.SetArgs([]string{"resolve", "pick", "aabbccdd", "--choice", "remote"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile("issues.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(result), `"title":"Y"`)
	assert.NotContains(t, string(result), `"_conflict"`)
}

func TestResolvePickCmd_WholeIssueConflict_Local(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	localIssue := map[string]interface{}{"id": "issue-4", "title": "local version"}
	remoteIssue := map[string]interface{}{"id": "issue-4", "title": "remote version"}
	localBytes, _ := json.Marshal(localIssue)
	remoteBytes, _ := json.Marshal(remoteIssue)
	entries := []merge.ConflictEntry{
		{
			ID:      "whole001",
			IssueID: "issue-4",
			Field:   "", // whole-issue conflict
			Local:   json.RawMessage(localBytes),
			Remote:  json.RawMessage(remoteBytes),
		},
	}
	writeConflictsFile(t, entries)

	conflictLine := `{"_conflict":true,"id":"issue-4","local":{"id":"issue-4","title":"local version"},"remote":{"id":"issue-4","title":"remote version"}}` + "\n"
	require.NoError(t, os.WriteFile("issues.jsonl", []byte(conflictLine), 0644))

	rootCmd.SetArgs([]string{"resolve", "pick", "whole001", "--choice", "local"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	result, err := os.ReadFile("issues.jsonl")
	require.NoError(t, err)
	assert.Contains(t, string(result), "local version")
}
