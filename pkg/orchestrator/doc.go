// Package orchestrator coordinates dispatch of Grava issues to remote agents.
//
// The orchestrator wires three cooperating components into a single dispatch
// loop:
//
//   - Poller — queries Dolt every PollIntervalSecs for open, unblocked issues
//     and feeds them into a TaskSink in priority order.
//   - AgentPool — manages a pool of HTTP agents, picking the least-loaded
//     available agent up to its MaxConcurrentTasks limit and POSTing claimed
//     tasks to its /task endpoint.
//   - Watchdog — pings each agent's /health endpoint, declares agents dead
//     after maxConsecutiveFailures heartbeat misses, and resets in-progress
//     tasks back to open both when the owning agent dies and when an issue
//     exceeds TaskTimeoutSecs.
//
// Orchestrator.sink claims tasks atomically (UPDATE … status='in_progress'
// WHERE status='open') before dispatch, eliminating the double-dispatch race
// when multiple orchestrators run concurrently. Failed dispatches reset the
// task to open and write a comment so the Poller will retry on the next tick.
//
// An optional StatusServer (attached via Orchestrator.WithStatusServer)
// exposes a /status JSON endpoint reporting uptime, dispatch/failure counters,
// and per-agent state. Configuration is loaded from YAML via LoadConfig and
// LoadAgents.
package orchestrator
