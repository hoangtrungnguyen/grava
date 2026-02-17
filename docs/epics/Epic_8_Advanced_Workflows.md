# Epic 8: Advanced Workflows (Rigs, Molecules, Compaction)

**Goal:** Implement enterprise-scale workflow automation, multi-repository support, and database maintenance.

**Success Criteria:**
- "Rigs" correctly route issues across multiple repositories
- "Molecules" successfully hydrate complex workflow templates
- "Compaction" prunes expired Wisps and Tombstones effectively
- System remains performant with >100,000 historical issues

## User Stories

### 8.1 Multi-Repo "Rigs" Support
**As a** platform engineer
**I want to** configure a single Grava instance to track issues across multiple repos
**So that** I have a unified view of the entire stack

**Acceptance Criteria:**
- `routes.jsonl` configuration file supported
- `grava create --rig=frontend` routes issue to correct directory
- Import/Sync logic respects rigorous boundaries
- Cross-repo dependencies (`fe-123` blocks `be-456`) function correctly

### 8.2 Molecule Templates
**As an** automation engineer
**I want to** define and spawn "Molecules" (workflow templates)
**So that** I can standardize complex, multi-step agent behaviors

**Acceptance Criteria:**
- `grava mol seed <type>` creates a tree of issues
- JSON template schema defined for Molecules
- "Patrol" molecule template (Scan -> Report -> Fix) implemented
- "Swarm" molecule template implemented

### 8.3 Database Compaction (Garbage Collection)
**As a** system administrator
**I want to** automatically prune temporary and deleted data
**So that** the database query performance remains high

**Acceptance Criteria:**
- `grava compact` command implemented
- Deletes `ephemeral` issues older than TTL
- Permanently removes `tombstone` issues older than 30 days
- Rebuilds/Optimizes Dolt tables after deletion
- Logs number of pruned rows to audit trail
