package graph

import (
	"container/heap"
)

// PriorityQueue implements heap.Interface for ReadyTask
type PriorityQueue []*ReadyTask

func (pq PriorityQueue) Len() int { return len(pq) }

// Less defines min-heap ordering: lower priority number = higher priority
// For ties, older tasks (longer age) come first
func (pq PriorityQueue) Less(i, j int) bool {
	// Compare effective priority (lower number = higher priority)
	if pq[i].EffectivePriority != pq[j].EffectivePriority {
		return pq[i].EffectivePriority < pq[j].EffectivePriority
	}

	// Tie-breaker: older tasks first
	return pq[i].Node.CreatedAt.Before(pq[j].Node.CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*ReadyTask)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // Avoid memory leak
	*pq = old[0 : n-1]
	return item
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(tasks []*ReadyTask) *PriorityQueue {
	pq := make(PriorityQueue, len(tasks))
	copy(pq, tasks)
	heap.Init(&pq)
	return &pq
}

// PushTask adds a task to the priority queue
func (pq *PriorityQueue) PushTask(task *ReadyTask) {
	heap.Push(pq, task)
}

// PopTask removes and returns the highest priority task
func (pq *PriorityQueue) PopTask() *ReadyTask {
	if pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq).(*ReadyTask)
}
