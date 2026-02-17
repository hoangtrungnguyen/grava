package idgen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

func TestStandardGenerator_GenerateBaseID(t *testing.T) {
	// BaseID generation doesn't use the store, so nil is fine if implementation allows,
	// but let's pass a mock for correctness.
	mockStore := dolt.NewMockStore()
	gen := NewStandardGenerator(mockStore)

	// Test 1: Verify format
	id := gen.GenerateBaseID()
	if !strings.HasPrefix(id, "grava-") {
		t.Errorf("ID %s does not start with expected prefix grava-", id)
	}

	// Expecting "grava-" followed by 4 hex characters
	// regex: ^grava-[0-9a-f]{4}$
	matched, err := regexp.MatchString(`^grava-[0-9a-f]{4}$`, id)
	if err != nil {
		t.Fatalf("Regex error: %v", err)
	}
	if !matched {
		t.Errorf("ID %s does not match expected format grava-XXXX", id)
	}
}

func TestStandardGenerator_GenerateChildID(t *testing.T) {
	mockStore := dolt.NewMockStore()
	gen := NewStandardGenerator(mockStore)
	parent := "grava-a1b2"

	// Mock behavior: first call returns 1, second 2

	child1, err := gen.GenerateChildID(parent)
	if err != nil {
		t.Fatalf("GenerateChildID failed: %v", err)
	}
	if child1 != "grava-a1b2.1" {
		t.Errorf("Expected grava-a1b2.1, got %s", child1)
	}

	child2, err := gen.GenerateChildID(parent)
	if err != nil {
		t.Fatalf("GenerateChildID failed: %v", err)
	}
	if child2 != "grava-a1b2.2" {
		t.Errorf("Expected grava-a1b2.2, got %s", child2)
	}
}
