package graph

import (
	"testing"
	"time"
)

func TestDefaultGateEvaluator_TimerGate(t *testing.T) {
	ge := NewDefaultGateEvaluator()

	now := time.Now()
	past := now.Add(-1 * time.Hour).Format(time.RFC3339)
	future := now.Add(1 * time.Hour).Format(time.RFC3339)

	nodePast := &Node{AwaitType: "timer", AwaitID: past}
	nodeFuture := &Node{AwaitType: "timer", AwaitID: future}

	open, err := ge.IsGateOpen(nodePast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !open {
		t.Errorf("expected past timer gate to be open")
	}

	open, err = ge.IsGateOpen(nodeFuture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if open {
		t.Errorf("expected future timer gate to be closed")
	}
}

func TestDefaultGateEvaluator_NoGate(t *testing.T) {
	ge := NewDefaultGateEvaluator()
	node := &Node{AwaitType: ""}
	open, _ := ge.IsGateOpen(node)
	if !open {
		t.Errorf("expected no gate to be open")
	}
}
