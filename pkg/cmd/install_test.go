package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTempGitRepo creates a temporary directory with a git repo and returns
// a cleanup function that restores the original working directory.
func initTempGitRepo(t *testing.T) (dir string, cleanup func()) {
	t.Helper()
	dir = t.TempDir()

	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "setup command failed: %s", string(out))
	}

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	return dir, func() {
		_ = os.Chdir(orig)
	}
}

func gitConfigGetLocal(t *testing.T, dir, key string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "--local", "--get", key).Output()
	require.NoError(t, err, "git config --get %s failed", key)
	return strings.TrimRight(string(out), "\n")
}

func TestInstallCmd_RegistersMergeDriver(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	name := gitConfigGetLocal(t, dir, "merge.grava.name")
	assert.Equal(t, "Grava JSONL Merge Driver", name)

	driver := gitConfigGetLocal(t, dir, "merge.grava.driver")
	assert.Equal(t, mergeDriverCmd, driver)
}

func TestInstallCmd_IdempotentOnRerun(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	for i := 0; i < 2; i++ {
		rootCmd.SetArgs([]string{"install"})
		err := rootCmd.Execute()
		assert.NoError(t, err, "run %d should succeed", i+1)
	}

	// Config should still be correct after double install
	driver := gitConfigGetLocal(t, dir, "merge.grava.driver")
	assert.Equal(t, mergeDriverCmd, driver)
}

func TestInstallCmd_FailsOutsideGitRepo(t *testing.T) {
	dir := t.TempDir() // no git init
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	rootCmd.SetArgs([]string{"install"})
	err = rootCmd.Execute()
	assert.Error(t, err)
}

func TestIsInGitRepo(t *testing.T) {
	// Inside a git repo (the project repo itself)
	assert.True(t, isInGitRepo())

	// Outside a git repo
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(t.TempDir())
	assert.False(t, isInGitRepo())
}

func TestRegisterMergeDriver(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	err := registerMergeDriver(installCmd)
	assert.NoError(t, err)

	assert.Equal(t, "Grava JSONL Merge Driver",
		gitConfigGetLocal(t, dir, "merge.grava.name"))
	assert.Equal(t, mergeDriverCmd,
		gitConfigGetLocal(t, dir, "merge.grava.driver"))
}

func TestInstallCmd_OutputContainsSuccess(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	buf := new(strings.Builder)
	installCmd.SetOut(buf)
	defer installCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "✅")
}

func TestGitConfigHelpers(t *testing.T) {
	dir, cleanup := initTempGitRepo(t)
	defer cleanup()

	_ = dir

	err := gitConfigSet("test.key", "test-value")
	assert.NoError(t, err)

	val, ok := gitConfigGet("test.key")
	assert.True(t, ok)
	assert.Equal(t, "test-value", val)

	_, ok = gitConfigGet("nonexistent.key")
	assert.False(t, ok)
}

func TestInstallCmd_DriverUsesCorrectPlaceholders(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())

	// Verify git driver command uses git placeholders, not hardcoded paths
	dir, _ := os.Getwd()
	driver := gitConfigGetLocal(t, dir, "merge.grava.driver")
	assert.Contains(t, driver, "%O", "driver should reference ancestor placeholder %%O")
	assert.Contains(t, driver, "%A", "driver should reference current placeholder %%A")
	assert.Contains(t, driver, "%B", "driver should reference other placeholder %%B")
	assert.Contains(t, driver, "merge-slot")
}

// writeGitAttr is a placeholder test verifying the gitattributes helper pattern
// (actual implementation comes in story 6.3).
func TestInstallCmd_SharedFlagExists(t *testing.T) {
	f := installCmd.Flags().Lookup("shared")
	assert.NotNil(t, f, "install command should have --shared flag")
	assert.Equal(t, "false", f.DefValue)
}

// helperFilePath returns an absolute path relative to the test dir.
func helperFilePath(dir, rel string) string {
	return filepath.Join(dir, rel)
}
