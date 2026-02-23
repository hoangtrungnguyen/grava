package graph

import "time"

// DependencyType represents the semantic type of a dependency edge
type DependencyType string

const (
	// Blocking Types (Hard Dependencies)
	DependencyBlocks    DependencyType = "blocks"     // from_id blocks to_id
	DependencyBlockedBy DependencyType = "blocked-by" // Inverse of blocks

	// Soft Dependencies
	DependencyWaitsFor  DependencyType = "waits-for"  // Soft dependency
	DependencyDependsOn DependencyType = "depends-on" // General dependency

	// Hierarchical
	DependencyParentChild DependencyType = "parent-child" // Hierarchical decomposition
	DependencyChildOf     DependencyType = "child-of"     // Inverse
	DependencySubtaskOf   DependencyType = "subtask-of"   // Task breakdown
	DependencyHasSubtask  DependencyType = "has-subtask"  // Inverse

	// Semantic Relationships
	DependencyDuplicates   DependencyType = "duplicates"    // Marks as duplicate
	DependencyDuplicatedBy DependencyType = "duplicated-by" // Inverse
	DependencyRelatesTo    DependencyType = "relates-to"    // General association
	DependencySupersedes   DependencyType = "supersedes"    // Replaces older task
	DependencySupersededBy DependencyType = "superseded-by" // Inverse

	// Ordering
	DependencyFollows  DependencyType = "follows"  // Sequencing hint
	DependencyPrecedes DependencyType = "precedes" // Inverse

	// Technical
	DependencyCausedBy DependencyType = "caused-by" // Bug causation
	DependencyCauses   DependencyType = "causes"    // Inverse
	DependencyFixes    DependencyType = "fixes"     // Fix relationship
	DependencyFixedBy  DependencyType = "fixed-by"  // Inverse
)

// IsBlockingType returns true if this dependency type blocks execution
func (dt DependencyType) IsBlockingType() bool {
	return dt == DependencyBlocks || dt == DependencyBlockedBy
}

// IsSoftDependency returns true if this is a soft dependency
func (dt DependencyType) IsSoftDependency() bool {
	return dt == DependencyWaitsFor || dt == DependencyDependsOn
}

// IssueStatus represents the status of an issue
type IssueStatus string

const (
	StatusOpen       IssueStatus = "open"
	StatusInProgress IssueStatus = "in_progress"
	StatusBlocked    IssueStatus = "blocked"
	StatusClosed     IssueStatus = "closed"
	StatusTombstone  IssueStatus = "tombstone"
	StatusDeferred   IssueStatus = "deferred"
	StatusPinned     IssueStatus = "pinned"
)

// Priority represents task priority (0=Critical to 4=Backlog)
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// Node represents a graph node (issue)
type Node struct {
	ID        string
	Title     string
	Status    IssueStatus
	Priority  Priority
	CreatedAt time.Time
	Ephemeral bool   // If true, excluded from normal exports (Wisp)
	AwaitType string // Gate type: "gh:pr", "timer", "human", empty for none
	AwaitID   string // Gate identifier
	Metadata  map[string]interface{}
}

// Edge represents a directed edge between two nodes
type Edge struct {
	FromID   string
	ToID     string
	Type     DependencyType
	Metadata map[string]interface{}
}

// ReadyTask represents a task ready for execution
type ReadyTask struct {
	Node              *Node
	EffectivePriority Priority // After priority inheritance
	Age               time.Duration
	PriorityBoosted   bool
}
