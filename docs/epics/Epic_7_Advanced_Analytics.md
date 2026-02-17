# Epic 7 (Optional): Advanced Graph Analytics

**Goal:** Implement sophisticated graph analysis for project optimization (post-MVP enhancement).

**Success Criteria:**
- PageRank identifies foundational blocker tasks
- Betweenness centrality detects bottlenecks
- Critical path analysis calculates longest dependency chains

## User Stories

### 7.1 PageRank Implementation
**As a** project manager  
**I want to** identify which tasks are blocking the most downstream work  
**So that** I can prioritize foundational issues

**Acceptance Criteria:**
- PageRank algorithm runs on dependency graph
- Results cached and refreshed every 30 minutes
- `analytics pagerank --top 10` shows most critical blockers
- Visual output shows PageRank score distribution

### 7.2 Critical Path Analysis
**As a** project manager  
**I want to** calculate the longest chain of dependencies  
**So that** I understand the minimum project completion time

**Acceptance Criteria:**
- Critical path algorithm identifies longest unbrokable chain
- Path displayed with issue IDs and estimated durations
- Tasks on critical path flagged in `ready` output
- Critical path recomputed on dependency changes

### 7.3 Bottleneck Detection
**As a** team lead  
**I want to** identify tasks that bridge major work clusters  
**So that** I can assign senior engineers to critical junctures

**Acceptance Criteria:**
- Betweenness centrality calculated for all nodes
- High-centrality tasks flagged as bottlenecks
- Visualization shows graph with bottlenecks highlighted
- Alerts when new bottleneck tasks are created
