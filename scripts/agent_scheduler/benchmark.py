"""
Performance benchmarks for PearceKellyScheduler.

Tests various graph sizes and operations to measure performance characteristics.
"""

import time
import random
import json
from datetime import datetime, timedelta
from typing import List, Dict, Tuple
from agent_scheduler import PearceKellyScheduler, Task, Priority


class BenchmarkResults:
    """Container for benchmark results."""

    def __init__(self):
        self.results = []
        self.metadata = {
            "timestamp": datetime.now().isoformat(),
            "python_version": "3.x",
        }

    def add_result(self, test_name: str, graph_size: Tuple[int, int],
                   operation: str, duration_ms: float, iterations: int = 1):
        """Add a benchmark result."""
        self.results.append({
            "test_name": test_name,
            "nodes": graph_size[0],
            "edges": graph_size[1],
            "operation": operation,
            "duration_ms": round(duration_ms, 3),
            "iterations": iterations,
            "avg_ms": round(duration_ms / iterations, 3) if iterations > 0 else 0,
        })

    def to_dict(self) -> Dict:
        """Convert to dictionary."""
        return {
            "metadata": self.metadata,
            "results": self.results,
        }

    def to_json(self) -> str:
        """Convert to JSON string."""
        return json.dumps(self.to_dict(), indent=2)


def create_graph(num_nodes: int, num_edges: int,
                 enable_priority_inheritance: bool = True) -> Tuple[PearceKellyScheduler, List[str]]:
    """
    Create a random DAG for benchmarking.

    Args:
        num_nodes: Number of tasks
        num_edges: Number of dependencies
        enable_priority_inheritance: Enable priority inheritance feature

    Returns:
        (scheduler, list of task names)
    """
    scheduler = PearceKellyScheduler(
        enable_priority_inheritance=enable_priority_inheritance,
        priority_inheritance_depth=10,
        aging_threshold=timedelta(days=7),
    )

    # Create tasks
    task_names = [f"task_{i:05d}" for i in range(num_nodes)]
    priorities = list(Priority)

    for name in task_names:
        priority = random.choice(priorities)
        duration = random.randint(1, 5)
        tokens = random.randint(500, 5000)
        task = Task(name, priority, duration, tokens)
        scheduler.register_task(task)

    # Add edges (ensuring DAG property by only connecting to higher-indexed nodes)
    edges_added = 0
    max_attempts = num_edges * 3

    for _ in range(max_attempts):
        if edges_added >= num_edges:
            break

        from_idx = random.randint(0, num_nodes - 2)
        to_idx = random.randint(from_idx + 1, num_nodes - 1)

        try:
            scheduler.add_dependency(task_names[from_idx], task_names[to_idx])
            edges_added += 1
        except ValueError:
            # Edge already exists or would create cycle
            continue

    return scheduler, task_names


def benchmark_graph_creation(num_nodes: int, num_edges: int, results: BenchmarkResults):
    """Benchmark graph creation time."""
    print(f"\n  Creating graph: {num_nodes} nodes, {num_edges} edges...")

    start = time.time()
    scheduler, task_names = create_graph(num_nodes, num_edges)
    duration_ms = (time.time() - start) * 1000

    results.add_result(
        f"graph_creation_{num_nodes}",
        (num_nodes, num_edges),
        "create_graph",
        duration_ms,
    )

    print(f"    Created in {duration_ms:.2f}ms")
    return scheduler, task_names


def benchmark_edge_addition(scheduler: PearceKellyScheduler, task_names: List[str],
                            num_additions: int, results: BenchmarkResults):
    """Benchmark incremental edge additions."""
    print(f"  Adding {num_additions} edges incrementally...")

    num_nodes = len(task_names)
    durations = []

    for i in range(num_additions):
        from_idx = random.randint(0, num_nodes - 2)
        to_idx = random.randint(from_idx + 1, num_nodes - 1)

        start = time.time()
        try:
            scheduler.add_dependency(task_names[from_idx], task_names[to_idx])
            duration_ms = (time.time() - start) * 1000
            durations.append(duration_ms)
        except ValueError:
            # Edge exists or cycle, skip
            continue

    if durations:
        avg_duration = sum(durations) / len(durations)
        max_duration = max(durations)
        min_duration = min(durations)
        p95_duration = sorted(durations)[int(len(durations) * 0.95)]

        results.add_result(
            f"edge_add_{num_nodes}",
            (num_nodes, len(scheduler.adj)),
            "add_edge_avg",
            avg_duration,
        )
        results.add_result(
            f"edge_add_{num_nodes}",
            (num_nodes, len(scheduler.adj)),
            "add_edge_p95",
            p95_duration,
        )
        results.add_result(
            f"edge_add_{num_nodes}",
            (num_nodes, len(scheduler.adj)),
            "add_edge_max",
            max_duration,
        )

        print(f"    Avg: {avg_duration:.3f}ms, P95: {p95_duration:.3f}ms, Max: {max_duration:.3f}ms")


def benchmark_edge_removal(scheduler: PearceKellyScheduler, task_names: List[str],
                           num_removals: int, results: BenchmarkResults):
    """Benchmark edge removals."""
    print(f"  Removing {num_removals} edges...")

    # Get existing edges
    edges = []
    for from_id, to_ids in scheduler.adj.items():
        for to_id in to_ids:
            edges.append((from_id, to_id))

    if not edges:
        print("    No edges to remove")
        return

    num_removals = min(num_removals, len(edges))
    edges_to_remove = random.sample(edges, num_removals)

    durations = []
    for from_id, to_id in edges_to_remove:
        start = time.time()
        scheduler.remove_dependency(from_id, to_id)
        duration_ms = (time.time() - start) * 1000
        durations.append(duration_ms)

    avg_duration = sum(durations) / len(durations)
    results.add_result(
        f"edge_remove_{len(task_names)}",
        (len(task_names), len(scheduler.adj)),
        "remove_edge",
        avg_duration,
    )

    print(f"    Avg: {avg_duration:.3f}ms")


def benchmark_ready_query(scheduler: PearceKellyScheduler, task_names: List[str],
                          num_queries: int, results: BenchmarkResults):
    """Benchmark ready task queries."""
    print(f"  Querying ready tasks {num_queries} times...")

    durations = []
    for i in range(num_queries):
        start = time.time()
        ready_tasks = scheduler.compute_ready_tasks(limit=10)
        duration_ms = (time.time() - start) * 1000
        durations.append(duration_ms)

        # Invalidate cache every 10 queries to test both cached and uncached
        if i % 10 == 0:
            scheduler._indegree_valid.clear()

    avg_duration = sum(durations) / len(durations)
    min_duration = min(durations)
    max_duration = max(durations)

    results.add_result(
        f"ready_query_{len(task_names)}",
        (len(task_names), sum(len(adj) for adj in scheduler.adj.values())),
        "ready_query_avg",
        avg_duration,
    )
    results.add_result(
        f"ready_query_{len(task_names)}",
        (len(task_names), sum(len(adj) for adj in scheduler.adj.values())),
        "ready_query_max",
        max_duration,
    )

    print(f"    Avg: {avg_duration:.3f}ms, Min: {min_duration:.3f}ms, Max: {max_duration:.3f}ms")


def benchmark_cycle_detection(scheduler: PearceKellyScheduler, task_names: List[str],
                              num_attempts: int, results: BenchmarkResults):
    """Benchmark cycle detection (by attempting to add edges that would create cycles)."""
    print(f"  Testing cycle detection {num_attempts} times...")

    num_nodes = len(task_names)
    durations = []

    for _ in range(num_attempts):
        # Try to add edge from high to low index (likely to create cycle)
        from_idx = random.randint(num_nodes // 2, num_nodes - 1)
        to_idx = random.randint(0, num_nodes // 2)

        start = time.time()
        try:
            scheduler.add_dependency(task_names[from_idx], task_names[to_idx])
        except ValueError:
            # Expected: cycle detected
            pass
        duration_ms = (time.time() - start) * 1000
        durations.append(duration_ms)

    avg_duration = sum(durations) / len(durations)
    results.add_result(
        f"cycle_detect_{num_nodes}",
        (num_nodes, sum(len(adj) for adj in scheduler.adj.values())),
        "cycle_detection",
        avg_duration,
    )

    print(f"    Avg: {avg_duration:.3f}ms")


def benchmark_priority_inheritance(scheduler: PearceKellyScheduler, task_names: List[str],
                                   num_queries: int, results: BenchmarkResults):
    """Benchmark priority inheritance calculation."""
    print(f"  Computing priority inheritance {num_queries} times...")

    durations = []
    sample_tasks = random.sample(task_names, min(num_queries, len(task_names)))

    for task_name in sample_tasks:
        start = time.time()
        scheduler.compute_effective_priority(task_name)
        duration_ms = (time.time() - start) * 1000
        durations.append(duration_ms)

    avg_duration = sum(durations) / len(durations)
    results.add_result(
        f"priority_inherit_{len(task_names)}",
        (len(task_names), sum(len(adj) for adj in scheduler.adj.values())),
        "priority_inheritance",
        avg_duration,
    )

    print(f"    Avg: {avg_duration:.3f}ms")


def benchmark_topological_sort(scheduler: PearceKellyScheduler, task_names: List[str],
                               results: BenchmarkResults):
    """Benchmark full topological sort."""
    print(f"  Computing topological sort...")

    start = time.time()
    topo_order = scheduler.topological_sort()
    duration_ms = (time.time() - start) * 1000

    results.add_result(
        f"topo_sort_{len(task_names)}",
        (len(task_names), sum(len(adj) for adj in scheduler.adj.values())),
        "topological_sort",
        duration_ms,
    )

    print(f"    Completed in {duration_ms:.2f}ms")


def benchmark_full_schedule(scheduler: PearceKellyScheduler, task_names: List[str],
                            results: BenchmarkResults):
    """Benchmark full schedule generation."""
    print(f"  Generating full schedule...")

    start = time.time()
    schedule_json = scheduler.calculate_schedule()
    duration_ms = (time.time() - start) * 1000

    results.add_result(
        f"full_schedule_{len(task_names)}",
        (len(task_names), sum(len(adj) for adj in scheduler.adj.values())),
        "full_schedule",
        duration_ms,
    )

    print(f"    Completed in {duration_ms:.2f}ms")


def run_benchmark_suite(num_nodes: int, num_edges: int, results: BenchmarkResults):
    """Run full benchmark suite for a given graph size."""
    print(f"\n{'='*60}")
    print(f"Benchmark Suite: {num_nodes} nodes, {num_edges} edges")
    print(f"{'='*60}")

    # 1. Graph creation
    scheduler, task_names = benchmark_graph_creation(num_nodes, num_edges, results)

    # 2. Edge addition (incremental)
    num_additions = min(100, num_nodes // 10)
    benchmark_edge_addition(scheduler, task_names, num_additions, results)

    # 3. Ready task queries
    num_queries = min(100, num_nodes)
    benchmark_ready_query(scheduler, task_names, num_queries, results)

    # 4. Cycle detection
    num_attempts = min(50, num_nodes // 10)
    benchmark_cycle_detection(scheduler, task_names, num_attempts, results)

    # 5. Priority inheritance
    num_priority_queries = min(50, num_nodes // 10)
    benchmark_priority_inheritance(scheduler, task_names, num_priority_queries, results)

    # 6. Edge removal
    num_removals = min(50, num_nodes // 20)
    benchmark_edge_removal(scheduler, task_names, num_removals, results)

    # 7. Topological sort
    benchmark_topological_sort(scheduler, task_names, results)

    # 8. Full schedule
    benchmark_full_schedule(scheduler, task_names, results)


def main():
    """Run all benchmarks."""
    print("=" * 60)
    print("PearceKellyScheduler - Performance Benchmarks")
    print("=" * 60)
    print(f"Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")

    results = BenchmarkResults()

    # Test configurations: (nodes, edges)
    # Edges ≈ 2-3x nodes for sparse graphs
    test_configs = [
        (100, 200),          # Small
        (500, 1000),         # Medium-small
        (1000, 3000),        # Medium
        (5000, 15000),       # Large
        (10000, 30000),      # Very large
    ]

    for num_nodes, num_edges in test_configs:
        try:
            run_benchmark_suite(num_nodes, num_edges, results)
        except Exception as e:
            print(f"\n  ERROR: {e}")
            continue

    # Save results
    output_file = "benchmark_results.json"
    with open(output_file, 'w') as f:
        f.write(results.to_json())

    print(f"\n{'='*60}")
    print(f"Benchmarks complete!")
    print(f"Results saved to: {output_file}")
    print(f"Completed at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"{'='*60}")

    # Print summary
    print("\n" + "=" * 60)
    print("Summary of Key Results")
    print("=" * 60)

    for config in test_configs:
        num_nodes, num_edges = config
        print(f"\nGraph: {num_nodes} nodes, {num_edges} edges")

        # Find relevant results
        for result in results.results:
            if result['nodes'] == num_nodes and result['operation'] in [
                'add_edge_avg', 'ready_query_avg', 'cycle_detection',
                'priority_inheritance', 'topological_sort'
            ]:
                print(f"  {result['operation']:25s}: {result['avg_ms']:8.3f}ms")

    return results


if __name__ == "__main__":
    main()
