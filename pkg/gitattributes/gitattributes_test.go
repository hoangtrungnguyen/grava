package gitattributes_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/gitattributes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempGitRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "git setup: %s", string(out))
	}
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	return dir, func() { _ = os.Chdir(orig) }
}

func attrPath(dir string) string {
	return filepath.Join(dir, ".gitattributes")
}

func TestEnsureMergeAttr_CreatesFile(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	added, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, err := os.ReadFile(attrPath(dir))
	require.NoError(t, err)
	assert.Contains(t, string(content), gitattributes.MergeAttrLine)
}

func TestEnsureMergeAttr_Idempotent(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	_, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)

	added, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)
	assert.False(t, added, "second call should not add the line again")

	// Verify line appears exactly once
	content, err := os.ReadFile(attrPath(dir))
	require.NoError(t, err)
	count := strings.Count(string(content), gitattributes.MergeAttrLine)
	assert.Equal(t, 1, count, "line should appear exactly once")
}

func TestEnsureMergeAttr_AppendsToExistingFile(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	// Write existing content without a trailing newline
	existing := "*.go diff=golang"
	require.NoError(t, os.WriteFile(attrPath(dir), []byte(existing), 0644))

	added, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, err := os.ReadFile(attrPath(dir))
	require.NoError(t, err)
	body := string(content)
	assert.Contains(t, body, existing)
	assert.Contains(t, body, gitattributes.MergeAttrLine)
	// The grava line must start on its own line
	assert.Contains(t, body, "\n"+gitattributes.MergeAttrLine)
}

func TestEnsureMergeAttr_ExistingFileWithTrailingNewline(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	existing := "*.go diff=golang\n"
	require.NoError(t, os.WriteFile(attrPath(dir), []byte(existing), 0644))

	added, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)
	assert.True(t, added)

	content, err := os.ReadFile(attrPath(dir))
	require.NoError(t, err)
	// Should not have a blank line between existing content and new line
	assert.NotContains(t, string(content), "\n\n")
}

func TestHasMergeAttr_FalseWhenAbsent(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHasMergeAttr_FalseWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHasMergeAttr_TrueAfterEnsure(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	_, err := gitattributes.EnsureMergeAttr(dir)
	require.NoError(t, err)

	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestHasMergeAttr_IgnoresPartialMatch(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	// Line that contains MergeAttrLine as a substring but is not an exact match
	partial := "# " + gitattributes.MergeAttrLine
	require.NoError(t, os.WriteFile(attrPath(dir), []byte(partial+"\n"), 0644))

	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.False(t, ok, "commented-out line should not count as present")
}

func TestHasMergeAttr_LeadingSpaceNotMatched(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	// Git does not strip leading spaces, so an indented line is not equivalent.
	indented := "  " + gitattributes.MergeAttrLine
	require.NoError(t, os.WriteFile(attrPath(dir), []byte(indented+"\n"), 0644))

	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.False(t, ok, "leading-space variant should not count as present")
}

func TestHasMergeAttr_TrailingWhitespaceIgnored(t *testing.T) {
	dir, cleanup := tempGitRepo(t)
	defer cleanup()

	withTrailing := gitattributes.MergeAttrLine + "   \r"
	require.NoError(t, os.WriteFile(attrPath(dir), []byte(withTrailing+"\n"), 0644))

	ok, err := gitattributes.HasMergeAttr(dir)
	require.NoError(t, err)
	assert.True(t, ok, "trailing whitespace/CR should be ignored")
}

func TestRepoRoot(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	root, err := gitattributes.RepoRoot()
	require.NoError(t, err)
	assert.NotEmpty(t, root)
}

func TestRepoRoot_OutsideRepo(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(t.TempDir()))

	_, err := gitattributes.RepoRoot()
	assert.Error(t, err)
}
