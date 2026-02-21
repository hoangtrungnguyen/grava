"""
Unit tests for PearceKellyScheduler.

Tests all major functionality including edge cases.
"""

import unittest
from datetime import datetime, timedelta
from agent_scheduler import (
    PearceKellyScheduler,
    Task,
    Priority,
    TaskStatus,
)


class TestTask(unittest.TestCase):
    """Test Task domain model."""

    def test_task_creation(self):
        task = Task("test-task", Priority.HIGH, duration=2, estimated_tokens=1000)
        self.assertEqual(task.name, "test-task")
        self.assertEqual(task.priority, Priority.HIGH)
        self.assertEqual(task.status, TaskStatus.OPEN)

    def test_task_validation(self):
        with self.assertRaises(ValueError):
            Task("", Priority.HIGH)  # Empty name

        with self.assertRaises(ValueError):
            Task("test", Priority.HIGH, duration=0)  # Invalid duration

        with self.assertRaises(ValueError):
            Task("test", Priority.HIGH, estimated_tokens=-1)  # Invalid tokens

    def test_priority_boost(self):
        p = Priority.BACKLOG
        boosted = p.boost(2)
        self.assertEqual(boosted, Priority.MEDIUM)

        # Test ceiling at CRITICAL
        p = Priority.HIGH
        boosted = p.boost(5)
        self.assertEqual(boosted, Priority.CRITICAL)


class TestScheduler(unittest.TestCase):
    """Test PearceKellyScheduler core functionality."""

    def setUp(self):
        self.scheduler = PearceKellyScheduler()

    def test_register_task(self):
        task = Task("task1", Priority.HIGH)
        self.scheduler.register_task(task)
        self.assertIn("task1", self.scheduler.tasks)

    def test_duplicate_registration(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task1", Priority.LOW)
        self.scheduler.register_task(task1)

        with self.assertRaises(ValueError):
            self.scheduler.register_task(task2)

    def test_add_dependency(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task2", Priority.MEDIUM)
        self.scheduler.register_task(task1)
        self.scheduler.register_task(task2)

        result = self.scheduler.add_dependency("task1", "task2")
        self.assertTrue(result)
        self.assertIn("task2", self.scheduler.adj["task1"])
        self.assertIn("task1", self.scheduler.preds["task2"])

    def test_self_loop_prevention(self):
        task = Task("task1", Priority.HIGH)
        self.scheduler.register_task(task)

        with self.assertRaises(ValueError):
            self.scheduler.add_dependency("task1", "task1")

    def test_cycle_detection(self):
        tasks = [Task(f"task{i}", Priority.MEDIUM) for i in range(3)]
        for task in tasks:
            self.scheduler.register_task(task)

        self.scheduler.add_dependency("task0", "task1")
        self.scheduler.add_dependency("task1", "task2")

        # This would create a cycle
        with self.assertRaises(ValueError) as cm:
            self.scheduler.add_dependency("task2", "task0")

        self.assertIn("Cycle detected", str(cm.exception))

    def test_remove_dependency(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task2", Priority.MEDIUM)
        self.scheduler.register_task(task1)
        self.scheduler.register_task(task2)

        self.scheduler.add_dependency("task1", "task2")
        removed = self.scheduler.remove_dependency("task1", "task2")

        self.assertTrue(removed)
        self.assertNotIn("task2", self.scheduler.adj["task1"])

    def test_remove_nonexistent_edge(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task2", Priority.MEDIUM)
        self.scheduler.register_task(task1)
        self.scheduler.register_task(task2)

        removed = self.scheduler.remove_dependency("task1", "task2")
        self.assertFalse(removed)

    def test_indegree_caching(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task2", Priority.MEDIUM)
        self.scheduler.register_task(task1)
        self.scheduler.register_task(task2)

        # Initial indegree
        indegree = self.scheduler.get_indegree("task2")
        self.assertEqual(indegree, 0)

        # Add dependency
        self.scheduler.add_dependency("task1", "task2")

        # Indegree should be invalidated and recomputed
        indegree = self.scheduler.get_indegree("task2")
        self.assertEqual(indegree, 1)

        # Second call should use cache
        indegree_cached = self.scheduler.get_indegree("task2")
        self.assertEqual(indegree_cached, 1)

    def test_compute_ready_tasks(self):
        task1 = Task("task1", Priority.HIGH)
        task2 = Task("task2", Priority.MEDIUM)
        task3 = Task("task3", Priority.LOW)
        self.scheduler.register_task(task1)
        self.scheduler.register_task(task2)
        self.scheduler.register_task(task3)

        self.scheduler.add_dependency("task1", "task2")

        ready = self.scheduler.compute_ready_tasks()

        # task1 and task3 should be ready, task2 is blocked
        ready_names = [task.name for task, _, _ in ready]
        self.assertIn("task1", ready_names)
        self.assertIn("task3", ready_names)
        self.assertNotIn("task2", ready_names)

    def test_priority_ordering(self):
        # Create tasks with different priorities
        tasks = [
            Task("low", Priority.LOW),
            Task("high", Priority.HIGH),
            Task("critical", Priority.CRITICAL),
            Task("medium", Priority.MEDIUM),
        ]
        for task in tasks:
            self.scheduler.register_task(task)

        ready = self.scheduler.compute_ready_tasks()

        # Should be ordered: critical, high, medium, low
        expected_order = ["critical", "high", "medium", "low"]
        actual_order = [task.name for task, _, _ in ready]
        self.assertEqual(actual_order, expected_order)

    def test_priority_inheritance(self):
        scheduler = PearceKellyScheduler(enable_priority_inheritance=True)

        task1 = Task("blocker", Priority.BACKLOG)
        task2 = Task("blocked", Priority.CRITICAL)
        scheduler.register_task(task1)
        scheduler.register_task(task2)

        scheduler.add_dependency("blocker", "blocked")

        # blocker should inherit CRITICAL priority from blocked
        effective = scheduler.compute_effective_priority("blocker")
        self.assertEqual(effective, Priority.CRITICAL)

    def test_topological_sort(self):
        tasks = [Task(f"task{i}", Priority.MEDIUM) for i in range(4)]
        for task in tasks:
            self.scheduler.register_task(task)

        self.scheduler.add_dependency("task0", "task1")
        self.scheduler.add_dependency("task1", "task2")
        self.scheduler.add_dependency("task2", "task3")

        topo_order = self.scheduler.topological_sort()

        # Verify topological property: for edge u->v, u comes before v
        task0_idx = topo_order.index("task0")
        task1_idx = topo_order.index("task1")
        task2_idx = topo_order.index("task2")
        task3_idx = topo_order.index("task3")

        self.assertLess(task0_idx, task1_idx)
        self.assertLess(task1_idx, task2_idx)
        self.assertLess(task2_idx, task3_idx)


class TestGates(unittest.TestCase):
    """Test gate evaluation system."""

    def setUp(self):
        self.scheduler = PearceKellyScheduler()

    def test_timer_gate_open(self):
        past_time = datetime.now() - timedelta(hours=1)
        task = Task(
            "timed-task",
            Priority.MEDIUM,
            await_type="timer",
            await_id=past_time.isoformat(),
        )
        self.scheduler.register_task(task)

        is_open = self.scheduler.gate_evaluator.is_open(task.await_type, task.await_id)
        self.assertTrue(is_open)

    def test_timer_gate_closed(self):
        future_time = datetime.now() + timedelta(hours=1)
        task = Task(
            "timed-task",
            Priority.MEDIUM,
            await_type="timer",
            await_id=future_time.isoformat(),
        )
        self.scheduler.register_task(task)

        is_open = self.scheduler.gate_evaluator.is_open(task.await_type, task.await_id)
        self.assertFalse(is_open)

    def test_human_gate(self):
        task = Task(
            "approval-task",
            Priority.HIGH,
            await_type="human",
            await_id="approval-123",
        )
        self.scheduler.register_task(task)

        # Initially closed
        is_open = self.scheduler.gate_evaluator.is_open(task.await_type, task.await_id)
        self.assertFalse(is_open)

        # Approve
        self.scheduler.gate_evaluator.approve_human_gate("approval-123")
        is_open = self.scheduler.gate_evaluator.is_open(task.await_type, task.await_id)
        self.assertTrue(is_open)

        # Revoke
        self.scheduler.gate_evaluator.revoke_human_gate("approval-123")
        is_open = self.scheduler.gate_evaluator.is_open(task.await_type, task.await_id)
        self.assertFalse(is_open)

    def test_gated_task_not_ready(self):
        future_time = datetime.now() + timedelta(hours=1)
        task = Task(
            "gated-task",
            Priority.HIGH,
            await_type="timer",
            await_id=future_time.isoformat(),
        )
        self.scheduler.register_task(task)

        ready = self.scheduler.compute_ready_tasks()
        ready_names = [t.name for t, _, _ in ready]
        self.assertNotIn("gated-task", ready_names)


class TestPearceKellyAlgorithm(unittest.TestCase):
    """Test Pearce-Kelly specific features."""

    def test_fast_path_optimization(self):
        """Test that edges preserving order don't trigger reordering."""
        scheduler = PearceKellyScheduler()

        tasks = [Task(f"task{i}", Priority.MEDIUM) for i in range(10)]
        for task in tasks:
            scheduler.register_task(task)

        # Add edges in order (should hit fast path)
        for i in range(9):
            scheduler.add_dependency(f"task{i}", f"task{i+1}")

        # All ranks should be in sequential order
        for i in range(9):
            self.assertLess(
                scheduler.ranks[f"task{i}"],
                scheduler.ranks[f"task{i+1}"]
            )

    def test_reordering_when_needed(self):
        """Test that edges violating order trigger reordering."""
        scheduler = PearceKellyScheduler()

        tasks = [Task(f"task{i}", Priority.MEDIUM) for i in range(5)]
        for task in tasks:
            scheduler.register_task(task)

        # Add edges that will require reordering
        scheduler.add_dependency("task0", "task1")
        scheduler.add_dependency("task2", "task3")
        scheduler.add_dependency("task1", "task3")  # Requires reordering

        # Verify topological property maintained
        self.assertLess(scheduler.ranks["task0"], scheduler.ranks["task1"])
        self.assertLess(scheduler.ranks["task1"], scheduler.ranks["task3"])
        self.assertLess(scheduler.ranks["task2"], scheduler.ranks["task3"])


if __name__ == "__main__":
    unittest.main()
