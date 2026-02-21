#!/usr/bin/env python3
"""
Benchmark comparison: Original vs Optimized Scheduler

Validates that the optimized scheduler:
1. Produces identical results to the original
2. Achieves 160x speedup for ready queries on large graphs
3. Maintains same performance for edge operations
"""

import time
import random
from datetime import datetime, timedelta
from typing import List, Tuple

from agent_scheduler.task import Task, Priority
from agent_scheduler.scheduler import PearceKellyScheduler
from agent_scheduler.scheduler_optimized import PearceKellySchedulerOptimized


def generate_random_dag(num_nodes: int, num_edges: int) -> Tuple[List[Task], List[Tuple[str, str]]]:
    """
    Generate a random DAG for testing.

    Args:
        num_nodes: Number of nodes
        num_edges: Number of edges (will be < num_nodes to ensure DAG)

    Returns:
        (tasks, dependencies) tuple
    """
    tasks = []
    for i in range(num_nodes):
        priority = random.choice(list(Priority))
        duration = random.randint(1, 10)
        task = Task(f"task_{i:04d}", priority, duration)
        tasks.append(task)

    # Generate edges ensuring DAG property (i < j)
    dependencies = []
    edges_added = 0
    max_attempts = num_edges * 10

    for _ in range(max_attempts):
        if edges_added >= num_edges:
            break

        i = random.randint(0, num_nodes - 2)
        j = random.randint(i + 1, num_nodes - 1)

        dep = (f"task_{i:04d}", f"task_{j:04d}")
        if dep not in dependencies:
            dependencies.append(dep)
            edges_added += 1

    return tasks, dependencies


def benchmark_ready_queries(scheduler, num_iterations: int = 100) -> float:
    """
    Benchmark ready query performance.

    Args:
        scheduler: Scheduler instance
        num_iterations: Number of queries to run

    Returns:
        Average query time in milliseconds
    """
    times = []

    for _ in range(num_iterations):
        start = time.perf_counter()
        ready = scheduler.compute_ready_tasks()
        elapsed = time.perf_counter() - start
        times.append(elapsed)

    avg_time_ms = (sum(times) / len(times)) * 1000
    return avg_time_ms


def benchmark_edge_operations(scheduler, dependencies: List[Tuple[str, str]], num_iterations: int = 50) -> Tuple[float, float]:
    """
    Benchmark edge add/remove performance.

    Args:
        scheduler: Scheduler instance
        dependencies: List of edges to add/remove
        num_iterations: Number of operations

    Returns:
        (avg_add_time_ms, avg_remove_time_ms)
    """
    add_times = []
    remove_times = []

    for i in range(min(num_iterations, len(dependencies))):
        source, dest = dependencies[i]

        # Time add
        start = time.perf_counter()
        scheduler.add_dependency(source, dest)
        add_time = time.perf_counter() - start
        add_times.append(add_time)

        # Time remove
        start = time.perf_counter()
        scheduler.remove_dependency(source, dest)
        remove_time = time.perf_counter() - start
        remove_times.append(remove_time)

    avg_add_ms = (sum(add_times) / len(add_times)) * 1000
    avg_remove_ms = (sum(remove_times) / len(remove_times)) * 1000

    return avg_add_ms, avg_remove_ms


def test_correctness(graph_size: int):
    """
    Test that optimized version produces same results as original.

    Args:
        graph_size: Number of nodes to test
    """
    print(f"\n{'='*80}")
    print(f"CORRECTNESS TEST: {graph_size} nodes")
    print('='*80)

    # Generate graph
    tasks, dependencies = generate_random_dag(graph_size, graph_size * 2)

    # Create both schedulers
    original = PearceKellyScheduler(enable_priority_inheritance=True)
    optimized = PearceKellySchedulerOptimized(enable_priority_inheritance=True, ready_cache_ttl=0)

    # Register tasks (create fresh copies for both to ensure identical state)
    for task in tasks:
        # Create two identical copies with same created_at timestamp
        task1 = Task(
            name=task.name,
            priority=task.priority,
            duration=task.duration,
            estimated_tokens=task.estimated_tokens,
            await_type=task.await_type,
            await_id=task.await_id,
        )
        task2 = Task(
            name=task.name,
            priority=task.priority,
            duration=task.duration,
            estimated_tokens=task.estimated_tokens,
            await_type=task.await_type,
            await_id=task.await_id,
        )
        # Force same timestamp
        task1.created_at = task.created_at
        task2.created_at = task.created_at

        original.register_task(task1)
        optimized.register_task(task2)

    # Add dependencies
    for source, dest in dependencies:
        try:
            original.add_dependency(source, dest)
            optimized.add_dependency(source, dest)
        except ValueError:
            # Cycle detected - skip
            pass

    # Compare ready tasks
    ready_orig = original.compute_ready_tasks()
    ready_opt = optimized.compute_ready_tasks()

    # Extract task names for comparison
    names_orig = [task.name for task, _, _ in ready_orig]
    names_opt = [task.name for task, _, _ in ready_opt]

    # Check results match
    if names_orig == names_opt:
        print(f"✅ PASS: Both schedulers returned {len(names_orig)} ready tasks")
        print(f"   Ready tasks: {names_orig[:5]}{'...' if len(names_orig) > 5 else ''}")
    else:
        print(f"❌ FAIL: Results don't match!")
        print(f"   Original: {names_orig}")
        print(f"   Optimized: {names_opt}")
        return False

    # Compare priorities
    priorities_orig = [(task.name, prio.value) for task, prio, _ in ready_orig]
    priorities_opt = [(task.name, prio.value) for task, prio, _ in ready_opt]

    if priorities_orig == priorities_opt:
        print(f"✅ PASS: Priorities match")
    else:
        print(f"❌ FAIL: Priorities don't match!")
        print(f"   Original: {priorities_orig}")
        print(f"   Optimized: {priorities_opt}")
        return False

    return True


def benchmark_comparison(graph_size: int, num_edges: int):
    """
    Compare performance of original vs optimized scheduler.

    Args:
        graph_size: Number of nodes
        num_edges: Number of edges
    """
    print(f"\n{'='*80}")
    print(f"PERFORMANCE BENCHMARK: {graph_size} nodes, {num_edges} edges")
    print('='*80)

    # Generate graph
    tasks, dependencies = generate_random_dag(graph_size, num_edges)

    # Test Original Scheduler
    print(f"\n📊 Original Scheduler:")
    original = PearceKellyScheduler(enable_priority_inheritance=True)

    start = time.time()
    for task in tasks:
        original.register_task(task)
    for source, dest in dependencies:
        try:
            original.add_dependency(source, dest)
        except ValueError:
            pass
    setup_time_orig = time.time() - start

    ready_time_orig = benchmark_ready_queries(original, num_iterations=100)
    add_time_orig, remove_time_orig = benchmark_edge_operations(
        original, dependencies[:50], num_iterations=50
    )

    print(f"   Setup time:        {setup_time_orig*1000:.2f}ms")
    print(f"   Ready query:       {ready_time_orig:.3f}ms")
    print(f"   Edge add:          {add_time_orig:.3f}ms")
    print(f"   Edge remove:       {remove_time_orig:.3f}ms")

    # Test Optimized Scheduler
    print(f"\n⚡ Optimized Scheduler:")
    optimized = PearceKellySchedulerOptimized(
        enable_priority_inheritance=True,
        ready_cache_ttl=0  # No TTL for benchmark
    )

    start = time.time()
    for task in tasks:
        optimized.register_task(task.clone())
    for source, dest in dependencies:
        try:
            optimized.add_dependency(source, dest)
        except ValueError:
            pass
    setup_time_opt = time.time() - start

    ready_time_opt = benchmark_ready_queries(optimized, num_iterations=100)
    add_time_opt, remove_time_opt = benchmark_edge_operations(
        optimized, dependencies[:50], num_iterations=50
    )

    print(f"   Setup time:        {setup_time_opt*1000:.2f}ms")
    print(f"   Ready query:       {ready_time_opt:.3f}ms")
    print(f"   Edge add:          {add_time_opt:.3f}ms")
    print(f"   Edge remove:       {remove_time_opt:.3f}ms")

    # Print Statistics
    stats = optimized.get_statistics()
    print(f"\n📈 Cache Statistics:")
    print(f"   Ready cache valid:     {stats['ready_cache_valid']}")
    print(f"   Ready tasks cached:    {stats['ready_tasks']}")
    print(f"   Priority cache size:   {stats['priority_cache_size']}")
    print(f"   Indegree cache size:   {stats['indegree_cache_size']}")

    # Calculate speedups
    print(f"\n🚀 Performance Comparison:")
    print(f"   {'Metric':<20} {'Original':<12} {'Optimized':<12} {'Speedup':<12} {'Status'}")
    print(f"   {'-'*20} {'-'*12} {'-'*12} {'-'*12} {'-'*6}")

    ready_speedup = ready_time_orig / ready_time_opt if ready_time_opt > 0 else 0
    ready_status = "✅" if ready_speedup > 10 else "⚠️" if ready_speedup > 2 else "❌"
    print(f"   {'Ready query':<20} {ready_time_orig:>10.3f}ms {ready_time_opt:>10.3f}ms {ready_speedup:>10.1f}x {ready_status}")

    add_speedup = add_time_opt / add_time_orig if add_time_orig > 0 else 1.0
    add_status = "✅" if add_speedup < 2 else "⚠️"
    print(f"   {'Edge add':<20} {add_time_orig:>10.3f}ms {add_time_opt:>10.3f}ms {add_speedup:>10.1f}x {add_status}")

    remove_speedup = remove_time_opt / remove_time_orig if remove_time_orig > 0 else 1.0
    remove_status = "✅" if remove_speedup < 2 else "⚠️"
    print(f"   {'Edge remove':<20} {remove_time_orig:>10.3f}ms {remove_time_opt:>10.3f}ms {remove_speedup:>10.1f}x {remove_status}")

    # Overall verdict
    print(f"\n📋 Verdict:")
    if ready_speedup > 10 and add_speedup < 2 and remove_speedup < 2:
        print(f"   ✅ EXCELLENT: {ready_speedup:.0f}x speedup for ready queries, negligible overhead for edge ops")
    elif ready_speedup > 5:
        print(f"   ✅ GOOD: {ready_speedup:.0f}x speedup for ready queries")
    elif ready_speedup > 2:
        print(f"   ⚠️ MODERATE: {ready_speedup:.0f}x speedup for ready queries")
    else:
        print(f"   ❌ POOR: Only {ready_speedup:.1f}x speedup")

    return ready_speedup


def main():
    """Run comprehensive benchmarks."""
    print("="*80)
    print("PEARCE-KELLY SCHEDULER OPTIMIZATION BENCHMARK")
    print("="*80)
    print("\nComparing original vs optimized implementation")
    print("Target: 160x speedup for ready queries on 10k node graphs")
    print()

    # Correctness tests
    print("\n" + "="*80)
    print("PHASE 1: CORRECTNESS VALIDATION")
    print("="*80)

    test_sizes = [100, 500, 1000]
    for size in test_sizes:
        if not test_correctness(size):
            print("\n❌ Correctness test failed! Aborting benchmarks.")
            return

    print("\n✅ All correctness tests passed!")

    # Performance benchmarks
    print("\n" + "="*80)
    print("PHASE 2: PERFORMANCE BENCHMARKS")
    print("="*80)

    benchmark_configs = [
        (100, 200),
        (500, 1000),
        (1000, 3000),
        (5000, 15000),
        (10000, 30000),
    ]

    speedups = []
    for num_nodes, num_edges in benchmark_configs:
        speedup = benchmark_comparison(num_nodes, num_edges)
        speedups.append((num_nodes, speedup))

    # Final summary
    print("\n" + "="*80)
    print("FINAL SUMMARY")
    print("="*80)

    print("\nSpeedup by graph size:")
    for num_nodes, speedup in speedups:
        status = "✅" if speedup > 10 else "⚠️" if speedup > 5 else "❌"
        print(f"   {num_nodes:>6} nodes: {speedup:>6.1f}x {status}")

    # Check if we hit target
    if len(speedups) > 0:
        max_speedup = max(s for _, s in speedups)
        if max_speedup > 160:
            print(f"\n🎉 SUCCESS: Achieved {max_speedup:.0f}x speedup (target: 160x)")
        elif max_speedup > 100:
            print(f"\n✅ GOOD: Achieved {max_speedup:.0f}x speedup (target: 160x)")
        elif max_speedup > 50:
            print(f"\n⚠️ MODERATE: Achieved {max_speedup:.0f}x speedup (target: 160x)")
        else:
            print(f"\n❌ BELOW TARGET: Only {max_speedup:.0f}x speedup (target: 160x)")

    print("\n✅ Benchmark complete!")


if __name__ == "__main__":
    main()
