# Story 4.4: Visualize the Full Dependency Graph

Status: ready-for-dev

## Story

As a developer or agent,
I want to visualize or traverse the overarching dependency structure,
So that I can understand the full project topology and identify critical paths.

## Acceptance Criteria

1. **AC#1 -- Graph Visualization (Default ASCII)**
   When I run `grava graph`,
   Then an ASCII-art dependency tree is printed to stdout,
   And all non-archived issues are included,
   And relationships are clearly indicated (e.g. `[A] --> [B]`).

2. **AC#2 -- Machine Readable Formats (DOT, JSON)**
   When I run `grava graph --format=dot`,
   Then a Graphviz-compatible DOT representation is printed.
   When I run `grava graph --format=json`,
   Then a JSON adjacency list is printed: `{"nodes": [...], "edges": [...]}` conforming to NFR5 schema.

3. **AC#3 -- Subgraph Targeting**
   When I run `grava graph --root abc123def456`,
   Then only the subgraph reachable from (and including) `abc123def456` is displayed.

4. **AC#4 -- Isolated Nodes**
   Issues with no dependencies (isolated nodes) MUST be included in the full graph output (AC#1 and AC#2), not omitted.

## Tasks / Subtasks

- [ ] Task 1: Enhance the `graph` Command
  - [ ] 1.1 Support `--format` flag with values: `ascii` (default), `dot`, `json`.
  - [ ] 1.2 Support `--root` flag for subgraph scoping.

- [ ] Task 2: Implement Graph Renderers
  - [ ] 2.1 Use existing `pkg/graph` dag engine to load data.
  - [ ] 2.2 Implement `DotRenderer` (simple string template).
  - [ ] 2.3 Implement `ASCIIRenderer` (recursive tree printer or layered dag printer).
  - [ ] 2.4 Implement `JSONRenderer` (serialise nodes and edges).

- [ ] Task 3: Testing
  - [ ] 3.1 Unit tests for each renderer in `pkg/graph/render_test.go`.
  - [ ] 3.2 Regression test for `grava graph` command.

## Dev Notes

### ASCII Rendering
For large graphs, full ASCII DAG rendering is hard. For Phase 1, a simple tree-like expansion or a list of edges `[A] blocks [B]` is acceptable if full dag-to-ascii proves complex. However, priority should be on readability.

### Architecture Patterns
- Data should be loaded via the `Store` and hydrated into the `graph.Graph` structure.

### References
- [Source: _bmad-output/planning-artifacts/epics/epic-04-dependency-graph.md#Story-4.4]
