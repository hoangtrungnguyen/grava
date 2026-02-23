"""
Pearce-Kelly Scheduler Implementation.

Implements dynamic topological sorting with caching and priority inheritance.
"""

import json
import heapq
from collections import defaultdict
from typing import Dict, List, Set, Tuple, Optional
from datetime import datetime, timedelta

from .task import Task, TaskStatus, Priority
from .gates import GateEvaluator


class PearceKellyScheduler:
    """
    Task scheduler using Pearce-Kelly dynamic topological sort.

    Features:
    - Incremental edge additions with cycle detection
    - Edge deletion support
    - Cached indegree calculations
    - Priority inheritance (high-priority tasks boost blockers)
    - Aging mechanism (old tasks get priority boost)
    - Gate evaluation (timer, human, GitHub PR)
    """

    def __init__(
        self,
        enable_priority_inheritance: bool = True,
        priority_inheritance_depth: int = 10,
        aging_threshold: timedelta = timedelta(days=7),
        aging_boost: int = 1,
        github_client=None,
    ):
        """
        Initialize the scheduler.

        Args:
            enable_priority_inheritance: Enable priority boosting for blockers
            priority_inheritance_depth: Max depth for priority propagation
            aging_threshold: Time before task gets priority boost
            aging_boost: How many priority levels to boost (default: 1)
            github_client: Optional GitHub API client for PR gates
        """
        self.tasks: Dict[str, Task] = {}

        # Graph structure (using sets for O(1) operations)
        self.adj: Dict[str, Set[str]] = defaultdict(set)    # source -> {destinations}
        self.preds: Dict[str, Set[str]] = defaultdict(set)  # destination -> {sources}
        self.ranks: Dict[str, int] = {}                     # task_name -> topological_rank

        # Cached indegree calculations
        self._indegree_cache: Dict[str, int] = {}
        self._indegree_valid: Set[str] = set()

        # Priority inheritance configuration
        self.enable_priority_inheritance = enable_priority_inheritance
        self.priority_inheritance_depth = priority_inheritance_depth
        self.aging_threshold = aging_threshold
        self.aging_boost = aging_boost

        # Gate evaluator
        self.gate_evaluator = GateEvaluator(github_client)

    def register_task(self, task: Task) -> None:
        """
        Register a new task in the scheduler.

        Args:
            task: Task to register

        Raises:
            ValueError: If task with same name already exists
        """
        if not isinstance(task, Task):
            raise TypeError(f"Expected Task instance, got {type(task)}")

        if task.name in self.tasks:
            raise ValueError(f"Task '{task.name}' is already registered.")

        self.tasks[task.name] = task
        self.adj[task.name] = set()
        self.preds[task.name] = set()

        # Initial rank is just the next available number
        self.ranks[task.name] = len(self.tasks) - 1

        # Initialize indegree cache
        self._indegree_cache[task.name] = 0
        self._indegree_valid.add(task.name)

    def add_dependency(self, source: str, dest: str) -> bool:
        """
        Add a dependency edge using Pearce-Kelly algorithm.

        Args:
            source: Task that blocks dest
            dest: Task that is blocked by source

        Returns:
            True if edge was added successfully

        Raises:
            KeyError: If either task doesn't exist
            ValueError: If edge would create a cycle or self-loop
        """
        # Validation
        if source not in self.tasks or dest not in self.tasks:
            raise KeyError(f"Both '{source}' and '{dest}' must be registered.")
        if source == dest:
            raise ValueError(f"Self-loop not allowed: '{source}' cannot depend on itself.")
        if dest in self.adj[source]:
            return True  # Edge already exists

        # 1. Fast path: Order already preserved
        if self.ranks[source] < self.ranks[dest]:
            self.adj[source].add(dest)
            self.preds[dest].add(source)
            self._invalidate_indegree(dest)
            self._update_task_status(dest)
            return True

        # 2. Potential Cycle/Reordering: Rank(source) >= Rank(dest)
        lower_bound = self.ranks[dest]
        upper_bound = self.ranks[source]

        # Find affected descendants of dest
        descendants = self._get_affected_descendants(dest, upper_bound)

        # Cycle Detection: if source is reachable from dest
        if source in descendants:
            cycle_path = self._reconstruct_cycle(source, dest)
            raise ValueError(
                f"Cycle detected! Cannot add edge '{source}' -> '{dest}'. "
                f"Cycle path: {' -> '.join(cycle_path)}"
            )

        # Find affected ancestors of source
        ancestors = self._get_affected_ancestors(source, lower_bound)

        # 3. Reorder the affected subset
        self._reorder(ancestors, descendants)

        # Safely add the edge
        self.adj[source].add(dest)
        self.preds[dest].add(source)
        self._invalidate_indegree(dest)
        self._update_task_status(dest)

        return True

    def remove_dependency(self, source: str, dest: str) -> bool:
        """
        Remove a dependency edge.

        Args:
            source: Source task
            dest: Destination task

        Returns:
            True if edge was removed, False if it didn't exist

        Raises:
            KeyError: If either task doesn't exist
        """
        if source not in self.tasks or dest not in self.tasks:
            raise KeyError(f"Both '{source}' and '{dest}' must be registered.")

        if dest not in self.adj[source]:
            return False  # Edge doesn't exist

        # Remove edge
        self.adj[source].discard(dest)
        self.preds[dest].discard(source)

        # Invalidate indegree cache
        self._invalidate_indegree(dest)
        self._update_task_status(dest)

        # Note: PK deletion algorithm could tighten the order here,
        # but it's not critical for correctness. The order remains valid,
        # just not maximally tight. Can be optimized later if needed.

        return True

    def _get_affected_descendants(self, start_task: str, upper_bound: int) -> List[str]:
        """DFS to find all descendants within rank bounds."""
        affected = []
        stack = [start_task]
        visited = {start_task}

        while stack:
            curr = stack.pop()
            affected.append(curr)
            for neighbor in self.adj[curr]:
                if neighbor not in visited and self.ranks[neighbor] <= upper_bound:
                    visited.add(neighbor)
                    stack.append(neighbor)

        return affected

    def _get_affected_ancestors(self, start_task: str, lower_bound: int) -> List[str]:
        """DFS to find all ancestors within rank bounds."""
        affected = []
        stack = [start_task]
        visited = {start_task}

        while stack:
            curr = stack.pop()
            affected.append(curr)
            # Using self.preds makes this extremely fast
            for p in self.preds[curr]:
                if p not in visited and self.ranks[p] >= lower_bound:
                    visited.add(p)
                    stack.append(p)

        return affected

    def _reorder(self, ancestors: List[str], descendants: List[str]) -> None:
        """Reorder affected vertices to maintain topological invariant."""
        # Combine unique affected tasks and sort by current ranks
        affected_tasks = list(set(ancestors + descendants))
        affected_tasks.sort(key=lambda x: self.ranks[x])

        # Capture the specific rank slots they currently occupy
        available_ranks = [self.ranks[t] for t in affected_tasks]

        # Generate new valid order for just this subset
        new_order = self._subgraph_topological_sort(affected_tasks)

        # Map the new valid order back into the available rank slots
        for i, task_name in enumerate(new_order):
            self.ranks[task_name] = available_ranks[i]

        # Invalidate indegree cache for affected tasks
        for task_name in affected_tasks:
            self._indegree_valid.discard(task_name)

    def _subgraph_topological_sort(self, subset_tasks: List[str]) -> List[str]:
        """Kahn's algorithm isolated to the affected subset."""
        subset = set(subset_tasks)
        local_in_degree = {t: 0 for t in subset}

        for t in subset:
            for neighbor in self.adj[t]:
                if neighbor in subset:
                    local_in_degree[neighbor] += 1

        queue = [t for t in subset if local_in_degree[t] == 0]
        result = []

        while queue:
            u = queue.pop(0)
            result.append(u)
            for v in self.adj[u]:
                if v in subset:
                    local_in_degree[v] -= 1
                    if local_in_degree[v] == 0:
                        queue.append(v)

        return result

    def _reconstruct_cycle(self, source: str, dest: str) -> List[str]:
        """Reconstruct cycle path for error reporting."""
        # Simple BFS to find path from dest to source
        queue = [(dest, [dest])]
        visited = {dest}

        while queue:
            curr, path = queue.pop(0)
            if curr == source:
                return path + [dest]  # Complete the cycle

            for neighbor in self.adj[curr]:
                if neighbor not in visited:
                    visited.add(neighbor)
                    queue.append((neighbor, path + [neighbor]))

        return [source, dest]  # Fallback

    def get_indegree(self, task_name: str) -> int:
        """
        Get the indegree (number of blockers) for a task.

        Uses caching for O(1) lookup after first computation.

        Args:
            task_name: Name of the task

        Returns:
            Number of tasks blocking this task
        """
        if task_name not in self.tasks:
            raise KeyError(f"Task '{task_name}' not found.")

        # Check cache
        if task_name in self._indegree_valid:
            return self._indegree_cache[task_name]

        # Compute indegree (only count open tasks)
        indegree = 0
        for pred in self.preds[task_name]:
            if self.tasks[pred].status == TaskStatus.OPEN:
                indegree += 1

        # Cache result
        self._indegree_cache[task_name] = indegree
        self._indegree_valid.add(task_name)

        return indegree

    def _invalidate_indegree(self, task_name: str) -> None:
        """Invalidate cached indegree for a task and its successors."""
        self._indegree_valid.discard(task_name)

        # Invalidate all successors (they may now be unblocked)
        for successor in self.adj[task_name]:
            self._indegree_valid.discard(successor)

    def _update_task_status(self, task_name: str) -> None:
        """Update task status based on dependencies."""
        task = self.tasks[task_name]

        if task.status == TaskStatus.CLOSED:
            return  # Don't change closed tasks

        # Check if blocked
        indegree = self.get_indegree(task_name)
        gate_open = self.gate_evaluator.is_open(task.await_type, task.await_id)

        if indegree > 0 or not gate_open:
            if task.status != TaskStatus.BLOCKED:
                task.status = TaskStatus.BLOCKED
        else:
            if task.status == TaskStatus.BLOCKED:
                task.status = TaskStatus.OPEN

    def compute_effective_priority(self, task_name: str) -> Priority:
        """
        Calculate effective priority with inheritance.

        High-priority dependents boost the priority of their blockers.

        Args:
            task_name: Name of the task

        Returns:
            Effective priority (original or inherited, whichever is higher)
        """
        task = self.tasks[task_name]
        base_priority = task.priority
        min_priority = base_priority

        if not self.enable_priority_inheritance:
            return base_priority

        # BFS to find highest-priority dependent
        queue = [(task_name, 0)]
        visited = {task_name}

        while queue:
            curr, depth = queue.pop(0)
            if depth >= self.priority_inheritance_depth:
                continue

            # Check all tasks blocked by curr
            for dependent in self.adj[curr]:
                if dependent in visited:
                    continue
                visited.add(dependent)

                dependent_task = self.tasks[dependent]
                # Check priority regardless of status for inheritance
                if dependent_task.priority < min_priority:
                    min_priority = dependent_task.priority

                # Continue traversal for open/blocked tasks
                if dependent_task.status in (TaskStatus.OPEN, TaskStatus.BLOCKED):
                    queue.append((dependent, depth + 1))

        return min_priority

    def compute_ready_tasks(self, limit: int = 0) -> List[Tuple[Task, Priority, bool]]:
        """
        Compute tasks that are ready to execute.

        Returns tasks sorted by effective priority (with inheritance and aging).

        Args:
            limit: Maximum number of tasks to return (0 = unlimited)

        Returns:
            List of (task, effective_priority, priority_boosted) tuples
        """
        now = datetime.now()
        ready_tasks = []

        for task_name, task in self.tasks.items():
            # Only consider open tasks
            if task.status != TaskStatus.OPEN:
                continue

            # Check if blocked by dependencies
            indegree = self.get_indegree(task_name)
            if indegree > 0:
                continue

            # Check gate conditions
            gate_open = self.gate_evaluator.is_open(task.await_type, task.await_id)
            if not gate_open:
                continue

            # Calculate effective priority
            effective_priority = self.compute_effective_priority(task_name)
            priority_boosted = (effective_priority < task.priority)

            # Apply aging boost
            age = now - task.created_at
            if age >= self.aging_threshold and effective_priority.value > Priority.CRITICAL.value:
                effective_priority = effective_priority.boost(self.aging_boost)
                priority_boosted = True

            ready_tasks.append((task, effective_priority, priority_boosted))

        # Sort by effective priority (lower value = higher priority), then by creation time
        ready_tasks.sort(key=lambda x: (x[1].value, x[0].created_at))

        # Apply limit
        if limit > 0:
            ready_tasks = ready_tasks[:limit]

        return ready_tasks

    def topological_sort(self) -> List[str]:
        """
        Full priority-based topological sort.

        Returns:
            List of task names in execution order
        """
        temp_in_degree = {name: self.get_indegree(name) for name in self.tasks}
        pq = []

        for name in self.tasks:
            if temp_in_degree[name] == 0:
                task = self.tasks[name]
                heapq.heappush(pq, (task.priority.value, task.created_at, name))

        topo_order = []
        while pq:
            _, _, current_name = heapq.heappop(pq)
            topo_order.append(current_name)

            for neighbor in self.adj[current_name]:
                temp_in_degree[neighbor] -= 1
                if temp_in_degree[neighbor] == 0:
                    neighbor_task = self.tasks[neighbor]
                    heapq.heappush(pq, (neighbor_task.priority.value, neighbor_task.created_at, neighbor))

        return topo_order

    def calculate_schedule(self) -> str:
        """
        Calculate execution schedule with timeline.

        Returns:
            JSON string with schedule details
        """
        topo_order = self.topological_sort()
        earliest_start = {name: 0 for name in topo_order}
        schedule_list = []
        total_projected_tokens = 0

        for name in topo_order:
            task = self.tasks[name]
            start_time = earliest_start[name]
            end_time = start_time + task.duration
            total_projected_tokens += task.estimated_tokens

            schedule_list.append({
                "task_name": task.name,
                "start_time": start_time,
                "end_time": end_time,
                "duration": task.duration,
                "priority": task.priority.value,
                "estimated_tokens": task.estimated_tokens,
                "status": task.status.value,
            })

            for neighbor in self.adj[name]:
                earliest_start[neighbor] = max(earliest_start[neighbor], end_time)

        schedule_list.sort(key=lambda x: (x["start_time"], x["priority"]))

        return json.dumps({
            "total_projected_tokens": total_projected_tokens,
            "task_count": len(schedule_list),
            "schedule": schedule_list
        }, indent=2)

    def get_statistics(self) -> Dict:
        """Get scheduler statistics."""
        total_tasks = len(self.tasks)
        status_counts = defaultdict(int)
        priority_counts = defaultdict(int)

        for task in self.tasks.values():
            status_counts[task.status.value] += 1
            priority_counts[task.priority.value] += 1

        ready_tasks = self.compute_ready_tasks()

        return {
            "total_tasks": total_tasks,
            "status_breakdown": dict(status_counts),
            "priority_breakdown": dict(priority_counts),
            "ready_tasks": len(ready_tasks),
            "total_edges": sum(len(edges) for edges in self.adj.values()),
            "avg_indegree": sum(len(preds) for preds in self.preds.values()) / total_tasks if total_tasks > 0 else 0,
        }
