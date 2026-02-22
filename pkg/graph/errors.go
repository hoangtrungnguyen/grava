package graph

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNilNode       = errors.New("node cannot be nil")
	ErrNodeExists    = errors.New("node already exists")
	ErrNodeNotFound  = errors.New("node not found")
	ErrNilEdge       = errors.New("edge cannot be nil")
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
