package graph

// Graph represents a directed graph
type Graph interface {
	// Node operations
	AddNode(node *Node) error
	GetNode(id string) (*Node, error)
	HasNode(id string) bool
	RemoveNode(id string) error
	NodeCount() int

	// Edge operations
	AddEdge(edge *Edge) error
	RemoveEdge(fromID, toID string, depType DependencyType) error
	GetEdges(nodeID string) ([]*Edge, error)
	GetOutgoingEdges(nodeID string) ([]*Edge, error)
	GetIncomingEdges(nodeID string) ([]*Edge, error)
	EdgeCount() int

	// Graph queries
	GetSuccessors(nodeID string) ([]string, error)
	GetPredecessors(nodeID string) ([]string, error)
	GetIndegree(nodeID string) int
	GetOutdegree(nodeID string) int

	// Traversal
	GetAllNodes() []*Node
	GetAllEdges() []*Edge
}

// DAG represents a Directed Acyclic Graph
type DAG interface {
	Graph

	// DAG-specific operations
	AddEdgeWithCycleCheck(edge *Edge) error
	DetectCycle() ([]string, error)
	TopologicalSort() ([]string, error)
	GetTransitiveDependencies(nodeID string, depth int) ([]string, error)
	IsReachable(fromID, toID string) bool
	SetNodeStatus(id string, status IssueStatus) error
	SetNodePriority(id string, priority Priority) error
	SetPriorityInheritanceDepth(depth int)

	// New advanced algorithms
	TransitiveReduction() error
	GetBlockingPath(fromID, toID string) ([]string, error)
}
