"""
Task domain model.

Separates domain concerns from graph mechanics.
"""

from enum import Enum
from typing import Optional
from datetime import datetime


class TaskStatus(Enum):
    """Task execution status."""
    OPEN = "open"
    BLOCKED = "blocked"
    IN_PROGRESS = "in_progress"
    CLOSED = "closed"


class Priority(Enum):
    """Task priority levels (0=Critical to 4=Backlog)."""
    CRITICAL = 0
    HIGH = 1
    MEDIUM = 2
    LOW = 3
    BACKLOG = 4

    def __lt__(self, other):
        """Enable comparison for priority queue."""
        if not isinstance(other, Priority):
            return NotImplemented
        return self.value < other.value

    def boost(self, levels: int = 1) -> 'Priority':
        """Boost priority by N levels (max: CRITICAL)."""
        new_value = max(0, self.value - levels)
        return Priority(new_value)


class Task:
    """
    Domain model for a schedulable task.

    Separated from graph mechanics to maintain clean architecture.
    """

    def __init__(
        self,
        name: str,
        priority: Priority = Priority.MEDIUM,
        duration: int = 1,
        estimated_tokens: int = 1000,
        await_type: Optional[str] = None,
        await_id: Optional[str] = None,
    ):
        # Validation
        if not name or not isinstance(name, str):
            raise ValueError(f"Task name must be a non-empty string, got: {name}")
        if duration <= 0:
            raise ValueError(f"Task duration must be positive, got: {duration}")
        if estimated_tokens <= 0:
            raise ValueError(f"Estimated tokens must be positive, got: {estimated_tokens}")

        self.name = name
        self.priority = priority
        self.duration = duration
        self.estimated_tokens = estimated_tokens
        self.used_tokens = 0
        self.status = TaskStatus.OPEN
        self.created_at = datetime.now()

        # Gate conditions
        self.await_type = await_type  # "timer", "human", "gh:pr", None
        self.await_id = await_id      # Gate identifier

    def __repr__(self):
        return f"[{self.status.value.upper()}] {self.name} (P{self.priority.value})"

    def __eq__(self, other):
        if not isinstance(other, Task):
            return NotImplemented
        return self.name == other.name

    def __hash__(self):
        return hash(self.name)

    def clone(self) -> 'Task':
        """Create a copy of this task."""
        task = Task(
            name=self.name,
            priority=self.priority,
            duration=self.duration,
            estimated_tokens=self.estimated_tokens,
            await_type=self.await_type,
            await_id=self.await_id,
        )
        task.used_tokens = self.used_tokens
        task.status = self.status
        task.created_at = self.created_at
        return task
