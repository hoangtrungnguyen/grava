package grava

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGravaDir(t *testing.T) {
	t.Run("GRAVA_DIR env var - valid directory", func(t *testing.T) {
		dir := t.TempDir()
		gravaDir := filepath.Join(dir, ".grava")
		require.NoError(t, os.MkdirAll(gravaDir, 0755))

		t.Setenv("GRAVA_DIR", gravaDir)
		got, err := ResolveGravaDir()
		require.NoError(t, err)

		wantResolved, _ := filepath.EvalSymlinks(gravaDir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		assert.Equal(t, wantResolved, gotResolved)
	})

	t.Run("GRAVA_DIR env var - nonexistent path returns NOT_INITIALIZED", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "/nonexistent/path/does/not/exist/.grava")
		_, err := ResolveGravaDir()
		require.Error(t, err)
		var gravaErr *gravaerrors.GravaError
		require.True(t, errors.As(err, &gravaErr))
		assert.Equal(t, "NOT_INITIALIZED", gravaErr.Code)
	})

	t.Run("redirect file - valid target", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "") // ensure env var is unset

		base := t.TempDir()
		gravaSubDir := filepath.Join(base, ".grava")
		require.NoError(t, os.MkdirAll(gravaSubDir, 0755))

		// Create real .grava target
		realGrava := filepath.Join(base, "real-grava")
		require.NoError(t, os.MkdirAll(realGrava, 0755))

		// Write redirect file with relative path
		require.NoError(t, os.WriteFile(
			filepath.Join(gravaSubDir, "redirect"),
			[]byte("../real-grava"),
			0644,
		))

		// Change working directory to base
		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(base))

		got, err := ResolveGravaDir()
		require.NoError(t, err)

		wantResolved, _ := filepath.EvalSymlinks(realGrava)
		gotResolved, _ := filepath.EvalSymlinks(got)
		assert.Equal(t, wantResolved, gotResolved)
	})

	t.Run("redirect file - stale target returns REDIRECT_STALE", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "")

		base := t.TempDir()
		gravaSubDir := filepath.Join(base, ".grava")
		require.NoError(t, os.MkdirAll(gravaSubDir, 0755))

		// Write redirect pointing to nonexistent target
		require.NoError(t, os.WriteFile(
			filepath.Join(gravaSubDir, "redirect"),
			[]byte("/nonexistent/stale/path"),
			0644,
		))

		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(base))

		_, err := ResolveGravaDir()
		require.Error(t, err)
		var gravaErr *gravaerrors.GravaError
		require.True(t, errors.As(err, &gravaErr))
		assert.Equal(t, "REDIRECT_STALE", gravaErr.Code)
	})

	t.Run("CWD walk - finds .grava in current dir", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "")

		base := t.TempDir()
		gravaDir := filepath.Join(base, ".grava")
		require.NoError(t, os.MkdirAll(gravaDir, 0755))

		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(base))

		got, err := ResolveGravaDir()
		require.NoError(t, err)

		wantResolved, _ := filepath.EvalSymlinks(gravaDir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		assert.Equal(t, wantResolved, gotResolved)
	})

	t.Run("CWD walk - finds .grava in parent dir", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "")

		base := t.TempDir()
		gravaDir := filepath.Join(base, ".grava")
		require.NoError(t, os.MkdirAll(gravaDir, 0755))

		subDir := filepath.Join(base, "subdir", "nested")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(subDir))

		got, err := ResolveGravaDir()
		require.NoError(t, err)

		wantResolved, _ := filepath.EvalSymlinks(gravaDir)
		gotResolved, _ := filepath.EvalSymlinks(got)
		assert.Equal(t, wantResolved, gotResolved)
	})

	t.Run("no .grava directory found returns NOT_INITIALIZED", func(t *testing.T) {
		t.Setenv("GRAVA_DIR", "")

		base := t.TempDir()

		// Skip if any ancestor of the temp dir already has a .grava/ — that would
		// make it impossible to test the NOT_INITIALIZED case from this path.
		dir := base
		for {
			if _, statErr := os.Stat(filepath.Join(dir, ".grava")); statErr == nil {
				t.Skip("skipping: .grava/ found in ancestry of temp dir; cannot isolate NOT_INITIALIZED case")
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}

		origDir, _ := os.Getwd()
		t.Cleanup(func() { _ = os.Chdir(origDir) })
		require.NoError(t, os.Chdir(base))

		_, err := ResolveGravaDir()
		require.Error(t, err, "expected NOT_INITIALIZED error when no .grava/ exists")
		var gravaErr *gravaerrors.GravaError
		require.True(t, errors.As(err, &gravaErr))
		assert.Equal(t, "NOT_INITIALIZED", gravaErr.Code)
	})
}
