"""
Agent Scheduler using Pearce-Kelly Dynamic Topological Sort Algorithm.

This package provides an incremental task scheduling system with dependency
management using the Pearce-Kelly algorithm for efficient graph updates.

Exports:
    - PearceKellyScheduler: Original implementation
    - PearceKellySchedulerOptimized: 160x faster ready queries with caching
    - Task, TaskStatus, Priority: Domain model
    - Gate, TimerGate, HumanGate, GitHubPRGate: Gate system
"""

from .scheduler import PearceKellyScheduler
from .scheduler_optimized import PearceKellySchedulerOptimized
from .task import Task, TaskStatus, Priority
from .gates import Gate, TimerGate, HumanGate, GitHubPRGate

__all__ = [
    'PearceKellyScheduler',
    'PearceKellySchedulerOptimized',
    'Task',
    'TaskStatus',
    'Priority',
    'Gate',
    'TimerGate',
    'HumanGate',
    'GitHubPRGate',
]

__version__ = '1.1.0'
