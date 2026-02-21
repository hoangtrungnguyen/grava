"""
Example usage of PearceKellyScheduler.

Demonstrates all major features:
- Task registration
- Dependency management
- Priority inheritance
- Gate evaluation
- Ready task computation
"""

from datetime import datetime, timedelta
from agent_scheduler import (
    PearceKellyScheduler,
    Task,
    Priority,
)


def main():
    print("=" * 60)
    print("Pearce-Kelly AgentScheduler - Example Usage")
    print("=" * 60)

    # Initialize scheduler with priority inheritance and aging
    scheduler = PearceKellyScheduler(
        enable_priority_inheritance=True,
        priority_inheritance_depth=10,
        aging_threshold=timedelta(days=7),
        aging_boost=1,
    )

    # Create tasks
    print("\n1. Creating tasks...")
    tasks = [
        Task("design-api", Priority.HIGH, duration=2, estimated_tokens=5000),
        Task("implement-auth", Priority.CRITICAL, duration=3, estimated_tokens=8000),
        Task("write-tests", Priority.MEDIUM, duration=2, estimated_tokens=3000),
        Task("deploy-staging", Priority.HIGH, duration=1, estimated_tokens=2000),
        Task("code-review", Priority.MEDIUM, duration=1, estimated_tokens=1000),
        Task("deploy-prod", Priority.CRITICAL, duration=1, estimated_tokens=2000),
    ]

    for task in tasks:
        scheduler.register_task(task)
        print(f"  ✓ Registered: {task}")

    # Add dependencies
    print("\n2. Adding dependencies...")
    dependencies = [
        ("design-api", "implement-auth"),
        ("implement-auth", "write-tests"),
        ("write-tests", "code-review"),
        ("code-review", "deploy-staging"),
        ("deploy-staging", "deploy-prod"),
    ]

    for source, dest in dependencies:
        scheduler.add_dependency(source, dest)
        print(f"  ✓ Added: {source} -> {dest}")

    # Show initial statistics
    print("\n3. Scheduler Statistics:")
    stats = scheduler.get_statistics()
    print(f"  Total tasks: {stats['total_tasks']}")
    print(f"  Total edges: {stats['total_edges']}")
    print(f"  Ready tasks: {stats['ready_tasks']}")
    print(f"  Status breakdown: {stats['status_breakdown']}")

    # Compute ready tasks
    print("\n4. Ready Tasks (unblocked and ready to execute):")
    ready_tasks = scheduler.compute_ready_tasks(limit=3)

    if ready_tasks:
        for task, effective_priority, boosted in ready_tasks:
            boost_indicator = " 🚀" if boosted else ""
            print(f"  • {task.name} (P{effective_priority.value}){boost_indicator}")
            print(f"    Duration: {task.duration}h, Tokens: {task.estimated_tokens}")
    else:
        print("  (No tasks are ready)")

    # Demonstrate priority inheritance
    print("\n5. Priority Inheritance Demo:")
    print("  Before: 'design-api' has priority HIGH (1)")
    effective = scheduler.compute_effective_priority("design-api")
    print(f"  After inheritance: Effective priority is {effective.name} ({effective.value})")
    print(f"  (Inherited from downstream CRITICAL task 'implement-auth')")

    # Add gate-based task
    print("\n6. Adding task with timer gate...")
    future_time = datetime.now() + timedelta(hours=2)
    gated_task = Task(
        "scheduled-maintenance",
        Priority.MEDIUM,
        duration=1,
        estimated_tokens=1500,
        await_type="timer",
        await_id=future_time.isoformat(),
    )
    scheduler.register_task(gated_task)
    print(f"  ✓ Registered: {gated_task}")
    print(f"  Gate status: {scheduler.gate_evaluator.get_status(gated_task.await_type, gated_task.await_id)}")

    # Demonstrate human gate
    print("\n7. Adding task with human approval gate...")
    approval_task = Task(
        "production-deployment",
        Priority.CRITICAL,
        duration=1,
        estimated_tokens=2000,
        await_type="human",
        await_id="security-review-2026",
    )
    scheduler.register_task(approval_task)
    print(f"  ✓ Registered: {approval_task}")
    print(f"  Gate status: {scheduler.gate_evaluator.get_status(approval_task.await_type, approval_task.await_id)}")

    print("\n  Approving gate...")
    scheduler.gate_evaluator.approve_human_gate("security-review-2026")
    print(f"  Gate status: {scheduler.gate_evaluator.get_status(approval_task.await_type, approval_task.await_id)}")

    # Test cycle detection
    print("\n8. Testing cycle detection...")
    try:
        scheduler.add_dependency("deploy-prod", "design-api")  # Would create cycle
        print("  ✗ ERROR: Cycle was not detected!")
    except ValueError as e:
        print(f"  ✓ Cycle detected: {e}")

    # Remove dependency
    print("\n9. Removing dependency...")
    removed = scheduler.remove_dependency("design-api", "implement-auth")
    print(f"  ✓ Removed: design-api -> implement-auth (success: {removed})")

    # Re-check ready tasks
    print("\n10. Ready Tasks after dependency removal:")
    ready_tasks = scheduler.compute_ready_tasks(limit=5)
    for task, effective_priority, boosted in ready_tasks:
        boost_indicator = " 🚀" if boosted else ""
        print(f"  • {task.name} (P{effective_priority.value}){boost_indicator}")

    # Generate full schedule
    print("\n11. Full Execution Schedule:")
    schedule_json = scheduler.calculate_schedule()
    print(schedule_json)

    # Topological order
    print("\n12. Topological Sort (execution order):")
    topo_order = scheduler.topological_sort()
    print(f"  {' -> '.join(topo_order)}")

    print("\n" + "=" * 60)
    print("Example completed successfully!")
    print("=" * 60)


if __name__ == "__main__":
    main()
