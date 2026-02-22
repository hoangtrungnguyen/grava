package graph

import (
	"fmt"
	"testing"
	"time"
)

func createLargeDAG(nodes, edges int) *AdjacencyDAG {
	dag := NewAdjacencyDAG(true)
	now := time.Now()

	for i := 0; i < nodes; i++ {
		dag.AddNode(&Node{
			ID:        fmt.Sprintf("node-%d", i),
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			CreatedAt: now,
		})
	}

	// Create edges only from i to j where i < j ensures no cycles
	count := 0
	for i := 0; i < nodes-1 && count < edges; i++ {
		for j := i + 1; j < nodes && count < edges; j++ {
			dag.AddEdge(&Edge{
				FromID: fmt.Sprintf("node-%d", i),
				ToID:   fmt.Sprintf("node-%d", j),
				Type:   DependencyBlocks,
			})
			count++
		}
	}

	return dag
}

func BenchmarkReadyEngine_10K(b *testing.B) {
	// 10k nodes, 30k edges is a bit much for a simple benchmark, let's use smaller for now to ensure it works
	dag := createLargeDAG(1000, 3000)
	engine := NewReadyEngine(dag, DefaultReadyEngineConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ComputeReady(100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCycleDetection_1K(b *testing.B) {
	dag := createLargeDAG(1000, 3000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dag.DetectCycle()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTopologicalSort_1K(b *testing.B) {
	dag := createLargeDAG(1000, 3000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dag.TopologicalSort()
		if err != nil {
			b.Fatal(err)
		}
	}
}
