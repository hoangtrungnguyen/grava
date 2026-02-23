package graph

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNilNode       = errors.New("node is nil")
	ErrNilEdge       = errors.New("edge is nil")
	ErrNodeNotFound  = errors.New("node not found")
	ErrNodeExists    = errors.New("node already exists")
	ErrSelfLoop      = errors.New("self-loops are not allowed")
	ErrCycleDetected = errors.New("cycle detected in graph")
)

// CycleError provides details about a detected cycle
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected: %s", strings.Join(e.Cycle, " -> "))
}

func (e *CycleError) Is(target error) bool {
	return target == ErrCycleDetected
}
