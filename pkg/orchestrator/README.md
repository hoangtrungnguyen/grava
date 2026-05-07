# Package: orchestrator

Path: `github.com/hoangtrungnguyen/grava/pkg/orchestrator`

## Purpose

High-level workflow engine that polls Grava's Dolt store for open issues and
dispatches them to a pool of HTTP agents, monitoring their health and
reclaiming abandoned tasks.

## Key Types & Functions

- `Orchestrator` / `NewOrchestrator(store, pool, cfg)` / `Run(ctx)` — wires
  Poller + AgentPool + Watchdog and runs the dispatch loop with graceful
  drain on cancellation. `WithStatusServer(srv)` opts into status counters.
- `Poller` / `NewPoller` / `Run` — ticks every `PollIntervalSecs`, queries
  open non-ephemeral issues whose dependencies are not still open, ordered
  by priority, and forwards each to a `TaskSink`. `DispatchableTask`
  describes the payload.
- `AgentPool` / `NewAgentPool` / `Pick`, `Dispatch`, `Complete`,
  `MarkAvailable`, `Stats` — least-loaded selection (respects
  `MaxConcurrentTasks`), atomic slot reservation, network-error handling,
  and per-agent counters.
- `Watchdog` / `NewWatchdog` / `Run` — periodic `/health` pings; declares
  agents DEAD after three consecutive misses, resets tasks past
  `TaskTimeoutSecs`, and writes audit `events` rows + comments.
- `Config`, `AgentConfig`, `LoadConfig`, `LoadAgents` — YAML configuration
  with sensible defaults and validation.
- `StatusServer` / `NewStatusServer` / `Handler` — `/status` JSON endpoint
  with atomic dispatch/failure counters.
- `writeEvent` (internal) — JSON-marshals before/after values and inserts
  into `events`.

## Dependencies

- `github.com/hoangtrungnguyen/grava/pkg/dolt` — store interface for issue
  reads/writes (`QueryContext`, `ExecContext`).
- `gopkg.in/yaml.v3` — config parsing.
- Standard library: `net/http`, `context`, `sync`, `log/slog`,
  `encoding/json`.

## How It Fits

Backs the `grava orchestrate` command. Agents (often Claude-powered worker
processes) register via the agents YAML; the orchestrator owns claim
ordering and timeout/recovery semantics so individual agents can stay
stateless. Audit `events` rows produced here feed the same ledger that
`grava history` and the Wisp/timeline tooling consume.

## Usage

```go
cfg, _ := orchestrator.LoadConfig("orchestrator.yaml")
agents, _ := orchestrator.LoadAgents(cfg.AgentsConfigPath)
pool := orchestrator.NewAgentPool(agents)
orc := orchestrator.NewOrchestrator(store, pool, cfg).
    WithStatusServer(orchestrator.NewStatusServer(pool))
orc.Run(ctx) // blocks until ctx cancelled
```
