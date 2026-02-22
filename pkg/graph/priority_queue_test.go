package graph

import (
	"testing"
	"time"
)

func TestPriorityQueue(t *testing.T) {
	now := time.Now()
	tasks := []*ReadyTask{
		{Node: &Node{ID: "Medium", CreatedAt: now}, EffectivePriority: PriorityMedium},
		{Node: &Node{ID: "Critical", CreatedAt: now}, EffectivePriority: PriorityCritical},
		{Node: &Node{ID: "High", CreatedAt: now}, EffectivePriority: PriorityHigh},
	}

	pq := NewPriorityQueue(tasks)

	if pq.Len() != 3 {
		t.Errorf("expected length 3, got %d", pq.Len())
	}

	// Should pop in order: Critical, High, Medium
	t1 := pq.PopTask()
	if t1.Node.ID != "Critical" {
		t.Errorf("expected Critical, got %s", t1.Node.ID)
	}

	t2 := pq.PopTask()
	if t2.Node.ID != "High" {
		t.Errorf("expected High, got %s", t2.Node.ID)
	}

	t3 := pq.PopTask()
	if t3.Node.ID != "Medium" {
		t.Errorf("expected Medium, got %s", t3.Node.ID)
	}
}

func TestPriorityQueue_TieBreak(t *testing.T) {
	now := time.Now()
	tasks := []*ReadyTask{
		{Node: &Node{ID: "New", CreatedAt: now}, EffectivePriority: PriorityHigh},
		{Node: &Node{ID: "Old", CreatedAt: now.Add(-1 * time.Hour)}, EffectivePriority: PriorityHigh},
	}

	pq := NewPriorityQueue(tasks)

	// Older should come first
	t1 := pq.PopTask()
	if t1.Node.ID != "Old" {
		t.Errorf("expected Old, got %s", t1.Node.ID)
	}
}
