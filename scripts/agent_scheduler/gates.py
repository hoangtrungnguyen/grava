"""
Gate evaluation system for external dependencies.

Gates block task execution until external conditions are met.
"""

from abc import ABC, abstractmethod
from datetime import datetime
from typing import Optional


class Gate(ABC):
    """Abstract base class for gate evaluators."""

    @abstractmethod
    def is_open(self, await_id: str) -> bool:
        """
        Check if the gate condition is met.

        Args:
            await_id: Gate-specific identifier

        Returns:
            True if gate is open (condition met), False otherwise
        """
        pass

    @abstractmethod
    def get_status(self, await_id: str) -> str:
        """
        Get human-readable gate status.

        Returns:
            Status string: "open", "closed", "pending", "error"
        """
        pass


class TimerGate(Gate):
    """Gate that opens after a specific timestamp."""

    def is_open(self, await_id: str) -> bool:
        """
        Check if current time is past the target time.

        Args:
            await_id: ISO 8601 timestamp (e.g., "2026-03-01T00:00:00Z")

        Returns:
            True if current time >= target time
        """
        try:
            target_time = datetime.fromisoformat(await_id.replace('Z', '+00:00'))
            return datetime.now(target_time.tzinfo) >= target_time
        except (ValueError, AttributeError) as e:
            raise ValueError(f"Invalid timer format '{await_id}': {e}")

    def get_status(self, await_id: str) -> str:
        try:
            target_time = datetime.fromisoformat(await_id.replace('Z', '+00:00'))
            now = datetime.now(target_time.tzinfo)
            if now >= target_time:
                return "open"
            time_remaining = target_time - now
            return f"closed (opens in {time_remaining})"
        except (ValueError, AttributeError):
            return "error"


class HumanGate(Gate):
    """Gate that requires manual human approval."""

    def __init__(self):
        self.approvals = set()  # Set of approved await_ids

    def is_open(self, await_id: str) -> bool:
        """Check if this gate has been manually approved."""
        return await_id in self.approvals

    def approve(self, await_id: str):
        """Manually approve a gate."""
        self.approvals.add(await_id)

    def revoke(self, await_id: str):
        """Revoke a previous approval."""
        self.approvals.discard(await_id)

    def get_status(self, await_id: str) -> str:
        return "open" if self.is_open(await_id) else "pending approval"


class GitHubPRGate(Gate):
    """Gate that opens when a GitHub PR is merged."""

    def __init__(self, github_client=None):
        """
        Initialize with optional GitHub client.

        Args:
            github_client: Client with is_pr_merged(owner, repo, pr_number) method
        """
        self.github_client = github_client
        self._cache = {}  # Cache PR status (await_id -> (is_merged, timestamp))
        self._cache_ttl = 300  # 5 minutes

    def is_open(self, await_id: str) -> bool:
        """
        Check if GitHub PR is merged.

        Args:
            await_id: Format "owner/repo/pulls/123"

        Returns:
            True if PR is merged, False otherwise (or if API unavailable)
        """
        if not self.github_client:
            # Graceful degradation: if API unavailable, gate stays closed
            return False

        # Check cache
        if await_id in self._cache:
            is_merged, timestamp = self._cache[await_id]
            age = (datetime.now() - timestamp).total_seconds()
            if age < self._cache_ttl:
                return is_merged

        # Parse await_id: "owner/repo/pulls/123"
        try:
            parts = await_id.split('/')
            if len(parts) != 4 or parts[2] != 'pulls':
                raise ValueError(f"Invalid GitHub PR format: {await_id}")

            owner, repo, _, pr_number = parts
            pr_number = int(pr_number)

            # Query GitHub API
            is_merged = self.github_client.is_pr_merged(owner, repo, pr_number)

            # Cache result
            self._cache[await_id] = (is_merged, datetime.now())

            return is_merged

        except (ValueError, IndexError, AttributeError) as e:
            raise ValueError(f"Failed to parse GitHub PR ID '{await_id}': {e}")

    def get_status(self, await_id: str) -> str:
        try:
            if self.is_open(await_id):
                return "open (PR merged)"
            return "closed (PR not merged)"
        except ValueError:
            return "error (invalid format)"

    def clear_cache(self):
        """Clear the PR status cache."""
        self._cache.clear()


class GateEvaluator:
    """
    Composite gate evaluator that manages all gate types.

    Automatically routes to the appropriate gate based on await_type.
    """

    def __init__(self, github_client=None):
        self.timer_gate = TimerGate()
        self.human_gate = HumanGate()
        self.github_gate = GitHubPRGate(github_client)

    def is_open(self, await_type: Optional[str], await_id: Optional[str]) -> bool:
        """
        Check if a gate is open.

        Args:
            await_type: Gate type ("timer", "human", "gh:pr", None)
            await_id: Gate identifier

        Returns:
            True if no gate or gate is open, False if gate is closed
        """
        if not await_type or not await_id:
            return True  # No gate

        gate_map = {
            "timer": self.timer_gate,
            "human": self.human_gate,
            "gh:pr": self.github_gate,
        }

        gate = gate_map.get(await_type)
        if not gate:
            raise ValueError(f"Unknown gate type: {await_type}")

        return gate.is_open(await_id)

    def get_status(self, await_type: Optional[str], await_id: Optional[str]) -> str:
        """Get human-readable gate status."""
        if not await_type or not await_id:
            return "no gate"

        gate_map = {
            "timer": self.timer_gate,
            "human": self.human_gate,
            "gh:pr": self.github_gate,
        }

        gate = gate_map.get(await_type)
        if not gate:
            return f"error (unknown gate type: {await_type})"

        return gate.get_status(await_id)

    def approve_human_gate(self, await_id: str):
        """Manually approve a human gate."""
        self.human_gate.approve(await_id)

    def revoke_human_gate(self, await_id: str):
        """Revoke human gate approval."""
        self.human_gate.revoke(await_id)
