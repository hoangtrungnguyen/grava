"""
Agent Scheduler using Pearce-Kelly Dynamic Topological Sort Algorithm.

This package provides an incremental task scheduling system with dependency
management using the Pearce-Kelly algorithm for efficient graph updates.
"""

from .scheduler import PearceKellyScheduler
from .task import Task, TaskStatus, Priority
from .gates import Gate, TimerGate, HumanGate, GitHubPRGate

__all__ = [
    'PearceKellyScheduler',
    'Task',
    'TaskStatus',
    'Priority',
    'Gate',
    'TimerGate',
    'HumanGate',
    'GitHubPRGate',
]

__version__ = '1.0.0'
