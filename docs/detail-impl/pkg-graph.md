# Module: `pkg/graph`

**Package role:** DAG engine for context-aware work dispatching. Implements topological sort, cycle detection, priority inheritance, gate evaluation, and ready-task discovery.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `adv_algo_test.go` | 72 | TestAdjacencyDAG_TransitiveReduction,TestAdjacencyDAG_GetBlockingPath |
| `benchmark_test.go` | 145 | TestMemorySmallScale,BenchmarkReadyEngine_10K BenchmarkReadyEngine_100K,BenchmarkCycleDetection_10K BenchmarkCycleDetection_100K |
| `cache.go` | 253 | GraphCache,NewGraphCache |
| `cache_test.go` | 165 | TestGraphCache,TestReadyEngine_Caching TestReadyEngine_IncrementalPriority,TestPriorityPropagation TestPriorityPropagation_Rollback |
| `cycle.go` | 1 | — |
| `cycle_test.go` | 1 | — |
| `dag.go` | 814 | AdjacencyDAG,NewAdjacencyDAG |
| `dag_test.go` | 65 | TestAdjacencyDAG_CycleDetection,TestAdjacencyDAG_TransitiveDependencies |
| `errors.go` | 29 | CycleError |
| `gates.go` | 100 | GateEvaluator,DefaultGateEvaluator NewDefaultGateEvaluator,GitHubClient |
| `gates_test.go` | 42 | TestDefaultGateEvaluator_TimerGate,TestDefaultGateEvaluator_NoGate |
| `graph.go` | 49 | Graph,DAG |
| `graph_test.go` | 91 | TestAdjacencyDAG_Nodes,TestAdjacencyDAG_Edges |
| `loader.go` | 81 | LoadGraphFromDB |
| `loader_test.go` | 119 | TestLoadGraphMetadataIntegration,TestLoadGraphMalformedMetadataIntegration TestLoaderMetadataErrorHandling |
| `mermaid.go` | 62 | ToMermaid |
| `mermaid_test.go` | 61 | TestToMermaid |
| `persistence_test.go` | 257 | TestSetNodeStatus_Persistence,TestSetNodePriority_Persistence TestUpdate_CacheConsistency,TestRemoveNode_Persistence TestRemoveEdge_Persistence |
| `priority_queue.go` | 61 | PriorityQueue,NewPriorityQueue |
| `priority_queue_test.go` | 53 | TestPriorityQueue,TestPriorityQueue_TieBreak |
| `ready_engine.go` | 242 | ReadyEngineConfig,DefaultReadyEngineConfig ReadyEngine,NewReadyEngine |
| `ready_engine_test.go` | 177 | TestReadyEngine_ComputeReady,TestReadyEngine_GateFiltering TestReadyEngine_DeepInheritance,TestReadyEngine_InheritanceLimit TestReadyEngine_Aging |
| `topology.go` | 115 | — |
| `topology_test.go` | 38 | TestAdjacencyDAG_TopologicalSort |
| `traversal.go` | 63 | — |
| `traversal_test.go` | 42 | TestAdjacencyDAG_Traversal |
| `types.go` | 105 | DependencyType,IssueStatus Priority,Node Edge |

## Public API

```
var ErrNilNode = errors.New("node is nil") ...
func ToMermaid(g DAG) string
type AdjacencyDAG struct{ ... }
    func LoadGraphFromDB(store dolt.Store) (*AdjacencyDAG, error)
    func NewAdjacencyDAG(enableCache bool) *AdjacencyDAG
type CycleError struct{ ... }
type DAG interface{ ... }
type DefaultGateEvaluator struct{ ... }
    func NewDefaultGateEvaluator() *DefaultGateEvaluator
type DependencyType string
    const DependencyBlocks DependencyType = "blocks" ...
type Edge struct{ ... }
type GateEvaluator interface{ ... }
type GitHubClient interface{ ... }
type Graph interface{ ... }
type GraphCache struct{ ... }
    func NewGraphCache(dag *AdjacencyDAG) *GraphCache
type IssueStatus string
    const StatusOpen IssueStatus = "open" ...
type Node struct{ ... }
type Priority int
    const PriorityCritical Priority = 0 ...
type PriorityQueue []*ReadyTask
    func NewPriorityQueue(tasks []*ReadyTask) *PriorityQueue
type ReadyEngine struct{ ... }
    func NewReadyEngine(dag *AdjacencyDAG, config *ReadyEngineConfig) *ReadyEngine
type ReadyEngineConfig struct{ ... }
    func DefaultReadyEngineConfig() *ReadyEngineConfig
type ReadyTask struct{ ... }
```

