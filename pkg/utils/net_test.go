package utils

import (
	"os"
	"testing"
)

func TestAllocatePort(t *testing.T) {
	// Set the home directory to a temp dir so we don't pollute the real ports.json
	tmpDir, err := os.MkdirTemp("", "grava_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("HOME", tmpDir)

	port1, err := AllocatePort("/project1", 3306)
	if err != nil {
		t.Fatal(err)
	}

	port2, err := AllocatePort("/project2", 3306)
	if err != nil {
		t.Fatal(err)
	}

	if port1 == port2 {
		t.Fatalf("Expected different ports, got same: %d", port1)
	}

	port1_again, err := AllocatePort("/project1", 3306)
	if err != nil {
		t.Fatal(err)
	}

	if port1_again != port1 {
		t.Fatalf("Expected same port for same project, got %d and %d", port1, port1_again)
	}
}
