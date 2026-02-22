package graph

import (
	"testing"
)

func TestGraphCache(t *testing.T) {
	dag := NewAdjacencyDAG(true)
	cache := dag.cache

	// Test Indegree Cache
	cache.SetIndegree("A", 5)
	val, ok := cache.GetIndegree("A")
	if !ok || val != 5 {
		t.Errorf("expected 5, got %d", val)
	}

	cache.InvalidateIndegree("A")
	_, ok = cache.GetIndegree("A")
	if ok {
		t.Errorf("expected A to be invalidated")
	}

	// Test InvalidateAll
	cache.SetIndegree("B", 3)
	cache.InvalidateAll()
	_, ok = cache.GetIndegree("B")
	if ok {
		t.Errorf("expected all to be invalidated")
	}
}
