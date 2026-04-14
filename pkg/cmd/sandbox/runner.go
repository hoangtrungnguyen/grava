// Package sandbox implements the `grava sandbox run` command for executing
// integration scenarios that validate system behavior under realistic conditions.
package sandbox

import (
	"context"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// Result is the structured output for a single scenario run.
type Result struct {
	Scenario   string   `json:"scenario"`
	Status     string   `json:"status"` // "pass" or "fail"
	DurationMs int64    `json:"duration_ms"`
	Details    []string `json:"details"`
	Error      string   `json:"error,omitempty"`
}

// Scenario describes an executable validation scenario.
type Scenario struct {
	// ID is the canonical identifier, e.g. "TS-01".
	ID string
	// Name is a short human-readable description.
	Name string
	// EpicGate is the minimum epic number that must be complete before this
	// scenario is meaningful. 0 means no gate.
	EpicGate int
	// Run executes the scenario against the provided store and returns a result.
	// Implementations must be self-contained: set up test data, run checks, clean up.
	Run func(ctx context.Context, store dolt.Store) Result
}

// registry holds all registered scenarios ordered by ID.
var registry []Scenario

// Register adds a scenario to the global registry.
func Register(s Scenario) {
	registry = append(registry, s)
}

// All returns all registered scenarios.
func All() []Scenario {
	return registry
}

// Find returns the scenario with the given ID, or (nil, false) if not found.
func Find(id string) (*Scenario, bool) {
	for i := range registry {
		if registry[i].ID == id {
			return &registry[i], true
		}
	}
	return nil, false
}

// Run executes a scenario and wraps timing around it.
func Run(ctx context.Context, store dolt.Store, s Scenario) Result {
	start := time.Now()
	r := s.Run(ctx, store)
	r.DurationMs = time.Since(start).Milliseconds()
	if r.Scenario == "" {
		r.Scenario = s.ID
	}
	return r
}

// pass returns a passing Result with the given detail messages.
func pass(scenario string, details ...string) Result {
	return Result{
		Scenario: scenario,
		Status:   "pass",
		Details:  details,
	}
}

// fail returns a failing Result with an error message and optional detail messages.
func fail(scenario, errMsg string, details ...string) Result {
	return Result{
		Scenario: scenario,
		Status:   "fail",
		Error:    errMsg,
		Details:  details,
	}
}
