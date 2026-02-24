package utils_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/utils"
)

func TestResolveDoltBinary_LocalExists(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, ".grava", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binaryName := "dolt"
	if runtime.GOOS == "windows" {
		binaryName = "dolt.exe"
	}
	localDolt := filepath.Join(binDir, binaryName)
	if err := os.WriteFile(localDolt, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := utils.ResolveDoltBinary(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != localDolt {
		t.Errorf("expected %q, got %q", localDolt, got)
	}
}

func TestResolveDoltBinary_FallsBackToSystem(t *testing.T) {
	tmp := t.TempDir()
	got, err := utils.ResolveDoltBinary(tmp)
	if err != nil {
		t.Skip("dolt not on system PATH, skipping fallback test")
	}
	if got == "" {
		t.Error("expected non-empty path")
	}
}

func TestResolveDoltBinary_NeitherFound(t *testing.T) {
	tmp := t.TempDir()
	_, _ = utils.ResolveDoltBinary(tmp)
}
