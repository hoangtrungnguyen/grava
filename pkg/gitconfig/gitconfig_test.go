package gitconfig_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/gitconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tempGitRepo creates a temp directory with a git repo and cds into it.
// Returns the dir path and a cleanup func that restores the original wd.
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

func TestRegisterMergeDriver_Fresh(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	cfg := gitconfig.DefaultDriverConfig()
	var buf strings.Builder
	alreadySet, err := gitconfig.RegisterMergeDriver(cfg, &buf, &buf)
	require.NoError(t, err)
	assert.False(t, alreadySet, "should not report already-set on first run")

	got, ok := gitconfig.Get()
	assert.True(t, ok)
	assert.Equal(t, cfg.Name, got.Name)
	assert.Equal(t, cfg.Driver, got.Driver)
}

func TestRegisterMergeDriver_Idempotent(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	cfg := gitconfig.DefaultDriverConfig()
	var buf strings.Builder

	_, err := gitconfig.RegisterMergeDriver(cfg, &buf, &buf)
	require.NoError(t, err)

	alreadySet, err := gitconfig.RegisterMergeDriver(cfg, &buf, &buf)
	require.NoError(t, err)
	assert.True(t, alreadySet, "second call should report already-set")
}

func TestRegisterMergeDriver_UpdatesStaleConfig(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	// Write an old config first
	var buf strings.Builder
	require.NoError(t, gitconfig.Set(
		"merge.grava.driver", "old-grava merge %O %A %B", &buf, &buf,
	))

	cfg := gitconfig.DefaultDriverConfig()
	alreadySet, err := gitconfig.RegisterMergeDriver(cfg, &buf, &buf)
	require.NoError(t, err)
	assert.False(t, alreadySet, "stale config should trigger a re-write")

	got, ok := gitconfig.Get()
	assert.True(t, ok)
	assert.Equal(t, gitconfig.DriverCmd, got.Driver)
}

func TestIsRegistered_AfterRegister(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	assert.False(t, gitconfig.IsRegistered())

	cfg := gitconfig.DefaultDriverConfig()
	var buf strings.Builder
	_, err := gitconfig.RegisterMergeDriver(cfg, &buf, &buf)
	require.NoError(t, err)

	assert.True(t, gitconfig.IsRegistered())
}

func TestGet_NotSet(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	_, ok := gitconfig.Get()
	assert.False(t, ok)
}

func TestGetValue_RoundTrip(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	var buf strings.Builder
	require.NoError(t, gitconfig.Set("test.roundtrip", "hello-world", &buf, &buf))

	val, ok := gitconfig.GetValue("test.roundtrip")
	assert.True(t, ok)
	assert.Equal(t, "hello-world", val)
}

func TestGetValue_Missing(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	_, ok := gitconfig.GetValue("does.not.exist")
	assert.False(t, ok)
}

func TestIsInGitRepo(t *testing.T) {
	_, cleanup := tempGitRepo(t)
	defer cleanup()

	assert.True(t, gitconfig.IsInGitRepo())
}

func TestIsInGitRepo_Outside(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	assert.False(t, gitconfig.IsInGitRepo())
}

func TestDefaultDriverConfig(t *testing.T) {
	cfg := gitconfig.DefaultDriverConfig()
	assert.Equal(t, gitconfig.DriverHumanName, cfg.Name)
	assert.Equal(t, gitconfig.DriverCmd, cfg.Driver)
	assert.Contains(t, cfg.Driver, "%O")
	assert.Contains(t, cfg.Driver, "%A")
	assert.Contains(t, cfg.Driver, "%B")
	assert.Contains(t, cfg.Driver, "merge-slot")
}
