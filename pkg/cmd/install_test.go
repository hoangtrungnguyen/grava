package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/gitconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTempGitRepo creates a temporary git repository and cds into it.
// Returns the dir path and a cleanup func that restores the original wd.
func initTempGitRepo(t *testing.T) (dir string, cleanup func()) {
	t.Helper()
	dir = t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "git setup failed: %s", string(out))
	}
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	return dir, func() { _ = os.Chdir(orig) }
}

func TestInstallCmd_RegistersMergeDriver(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())

	assert.True(t, gitconfig.IsRegistered())
}

func TestInstallCmd_IdempotentOnRerun(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	for i := 0; i < 2; i++ {
		rootCmd.SetArgs([]string{"install"})
		assert.NoError(t, rootCmd.Execute(), "run %d should succeed", i+1)
	}

	assert.True(t, gitconfig.IsRegistered())
}

func TestInstallCmd_FailsOutsideGitRepo(t *testing.T) {
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(t.TempDir()))

	rootCmd.SetArgs([]string{"install"})
	assert.Error(t, rootCmd.Execute())
}

func TestInstallCmd_OutputFreshInstall(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	var buf strings.Builder
	installCmd.SetOut(&buf)
	defer installCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "✅")
	assert.Contains(t, buf.String(), "registered")
}

func TestInstallCmd_OutputAlreadyConfigured(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	// First install
	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())

	// Second install — should report already configured
	var buf strings.Builder
	installCmd.SetOut(&buf)
	defer installCmd.SetOut(nil)

	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "already configured")
}

func TestInstallCmd_DriverHasCorrectPlaceholders(t *testing.T) {
	_, cleanup := initTempGitRepo(t)
	defer cleanup()

	rootCmd.SetArgs([]string{"install"})
	require.NoError(t, rootCmd.Execute())

	cfg, ok := gitconfig.Get()
	require.True(t, ok)
	assert.Contains(t, cfg.Driver, "%O")
	assert.Contains(t, cfg.Driver, "%A")
	assert.Contains(t, cfg.Driver, "%B")
	assert.Contains(t, cfg.Driver, "merge-slot")
}

func TestInstallCmd_SharedFlagExists(t *testing.T) {
	f := installCmd.Flags().Lookup("shared")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}
