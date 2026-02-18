# Skills Storage Implementation Plan for Grava

**Date**: 2026-02-18
**Status**: Planning
**Complexity**: Medium-High
**Estimated Epics**: 1-2 (depending on approach)

---

## Executive Summary

This document outlines three architectural approaches for adding "skills" storage to the Grava issue tracker, enabling skill-based task assignment and agent capability matching.

**Current State:**
- Grava uses Dolt (version-controlled SQL) with basic CRUD operations
- `issues` table has `assignee` field but no skill/capability concept
- `metadata` JSON field provides extensibility
- No existing skill management in Epics 1-8

**Goal:**
Enable agents to:
1. Declare their capabilities (skills)
2. Match tasks requiring specific skills
3. Support the "Ready Engine" with skill-based task eligibility

---

## Architecture Options

### Option 1: Lightweight - Metadata Field Approach

**Strategy**: Store skills as JSON in existing `issues.metadata` field.

#### Schema Changes
**NONE** - Uses existing infrastructure.

#### Data Structure
```json
{
  "required_skills": ["go", "sql", "dolt"],
  "skill_level": "intermediate",
  "optional_skills": ["docker", "kubernetes"]
}
```

#### Implementation Steps

**1. Documentation (1 hour)**
- Create `docs/schemas/issue_metadata_skills.md`
- Define JSON schema convention
- Document skill taxonomy (language, framework, tool, domain)

**2. CLI Extension (2-3 hours)**

File: `pkg/cmd/create.go`
```go
// Add flags
createCmd.Flags().StringSlice("require-skills", []string{}, "Required skills (comma-separated)")
createCmd.Flags().String("skill-level", "intermediate", "Required proficiency level")
```

File: `pkg/cmd/list.go`
```go
// Add filtering
listCmd.Flags().StringSlice("has-skills", []string{}, "Filter by required skills")
```

**3. Query Helpers (2 hours)**

File: `pkg/dolt/skills_query.go`
```go
// FindIssuesBySkill queries metadata JSON
func (c *Client) FindIssuesBySkill(ctx context.Context, skillName string) ([]string, error) {
    query := `
        SELECT id
        FROM issues
        WHERE JSON_CONTAINS(metadata->'$.required_skills', ?)
    `
    // Implementation
}
```

**4. Testing (1 hour)**
- Unit tests for JSON parsing
- Integration tests for skill filtering

#### Pros
- ✅ Zero schema changes
- ✅ Immediate implementation (1 day)
- ✅ Flexible structure
- ✅ No migration required

#### Cons
- ❌ No type safety or validation
- ❌ Poor query performance (no indexes on JSON)
- ❌ No relational integrity
- ❌ Difficult to manage skill catalog

#### When to Use
- MVP/prototyping phase
- < 1000 issues
- Simple skill tracking without advanced matching

---

### Option 2: Dedicated Skills Tables (RECOMMENDED)

**Strategy**: Full relational model with dedicated tables for skills, requirements, and agent capabilities.

#### Schema Changes

File: `scripts/schema/002_skills_schema.sql`

```sql
-- ============================================
-- Skills Catalog
-- ============================================
CREATE TABLE skills (
    id VARCHAR(32) PRIMARY KEY,              -- e.g., 'skill-go-lang'
    name VARCHAR(128) NOT NULL UNIQUE,       -- e.g., 'Go Programming'
    slug VARCHAR(128) NOT NULL UNIQUE,       -- e.g., 'go'
    category VARCHAR(64),                    -- 'language', 'framework', 'tool', 'domain'
    description TEXT,
    parent_skill_id VARCHAR(32),             -- For hierarchical skills (e.g., 'go' -> 'concurrency')
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_skill_id) REFERENCES skills(id) ON DELETE SET NULL,
    INDEX idx_category (category),
    INDEX idx_slug (slug)
);

-- ============================================
-- Issue Skill Requirements (many-to-many)
-- ============================================
CREATE TABLE issue_skills (
    issue_id VARCHAR(32),
    skill_id VARCHAR(32),
    proficiency_required VARCHAR(32) DEFAULT 'intermediate', -- 'beginner', 'intermediate', 'expert'
    is_required BOOLEAN DEFAULT TRUE,        -- TRUE=required, FALSE=optional/nice-to-have
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, skill_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    INDEX idx_skill (skill_id),
    INDEX idx_required (is_required)
);

-- ============================================
-- Agent Capabilities (for task matching)
-- ============================================
CREATE TABLE agent_skills (
    agent_id VARCHAR(128),                   -- Matches 'assignee' field format (e.g., 'claude-opus', 'gpt-4')
    skill_id VARCHAR(32),
    proficiency_level VARCHAR(32) DEFAULT 'intermediate', -- 'beginner', 'intermediate', 'expert'
    verified_at TIMESTAMP NULL,              -- NULL=self-reported, timestamp=verified
    success_count INT DEFAULT 0,             -- Track successful task completions
    failure_count INT DEFAULT 0,             -- Track failed attempts
    last_used_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (agent_id, skill_id),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    INDEX idx_agent (agent_id),
    INDEX idx_proficiency (proficiency_level)
);

-- ============================================
-- Skill Events (audit trail)
-- ============================================
CREATE TABLE skill_events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    entity_type VARCHAR(32) NOT NULL,        -- 'issue', 'agent', 'skill'
    entity_id VARCHAR(128) NOT NULL,
    skill_id VARCHAR(32),
    event_type VARCHAR(32) NOT NULL,         -- 'add', 'remove', 'update_proficiency', 'verify'
    old_value JSON,
    new_value JSON,
    actor VARCHAR(128),                      -- Who made the change
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_timestamp (timestamp)
);
```

#### Data Models

File: `pkg/models/skill.go`

```go
package models

import "time"

type Skill struct {
    ID            string    `db:"id"`
    Name          string    `db:"name"`
    Slug          string    `db:"slug"`
    Category      string    `db:"category"`
    Description   string    `db:"description"`
    ParentSkillID *string   `db:"parent_skill_id"`
    CreatedAt     time.Time `db:"created_at"`
    UpdatedAt     time.Time `db:"updated_at"`
}

type IssueSkill struct {
    IssueID             string    `db:"issue_id"`
    SkillID             string    `db:"skill_id"`
    ProficiencyRequired string    `db:"proficiency_required"`
    IsRequired          bool      `db:"is_required"`
    CreatedAt           time.Time `db:"created_at"`
}

type AgentSkill struct {
    AgentID          string     `db:"agent_id"`
    SkillID          string     `db:"skill_id"`
    ProficiencyLevel string     `db:"proficiency_level"`
    VerifiedAt       *time.Time `db:"verified_at"`
    SuccessCount     int        `db:"success_count"`
    FailureCount     int        `db:"failure_count"`
    LastUsedAt       *time.Time `db:"last_used_at"`
    CreatedAt        time.Time  `db:"created_at"`
    UpdatedAt        time.Time  `db:"updated_at"`
}

type SkillEvent struct {
    ID         int       `db:"id"`
    EntityType string    `db:"entity_type"`
    EntityID   string    `db:"entity_id"`
    SkillID    string    `db:"skill_id"`
    EventType  string    `db:"event_type"`
    OldValue   string    `db:"old_value"` // JSON
    NewValue   string    `db:"new_value"` // JSON
    Actor      string    `db:"actor"`
    Timestamp  time.Time `db:"timestamp"`
}

// Enums
const (
    ProficiencyBeginner     = "beginner"
    ProficiencyIntermediate = "intermediate"
    ProficiencyExpert       = "expert"

    CategoryLanguage  = "language"
    CategoryFramework = "framework"
    CategoryTool      = "tool"
    CategoryDomain    = "domain"
)
```

#### Data Access Layer

File: `pkg/dolt/skills_store.go`

```go
package dolt

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/yourusername/grava/pkg/models"
)

// SkillsStore extends Client with skill-related operations
type SkillsStore interface {
    // Skills CRUD
    CreateSkill(ctx context.Context, skill *models.Skill) error
    GetSkillByID(ctx context.Context, id string) (*models.Skill, error)
    GetSkillBySlug(ctx context.Context, slug string) (*models.Skill, error)
    ListSkills(ctx context.Context, category string) ([]*models.Skill, error)

    // Issue Skills
    AddIssueSkill(ctx context.Context, issueID, skillID, proficiency string, required bool) error
    RemoveIssueSkill(ctx context.Context, issueID, skillID string) error
    GetIssueSkills(ctx context.Context, issueID string) ([]*models.IssueSkill, error)

    // Agent Skills
    RegisterAgentSkill(ctx context.Context, agentID, skillID, proficiency string) error
    GetAgentSkills(ctx context.Context, agentID string) ([]*models.AgentSkill, error)
    UpdateAgentSkillProficiency(ctx context.Context, agentID, skillID, proficiency string) error

    // Matching & Discovery
    FindEligibleAgents(ctx context.Context, issueID string) ([]string, error)
    FindEligibleIssues(ctx context.Context, agentID string) ([]string, error)

    // Events
    LogSkillEvent(ctx context.Context, event *models.SkillEvent) error
}

// Implementation (sample method)
func (c *Client) CreateSkill(ctx context.Context, skill *models.Skill) error {
    query := `
        INSERT INTO skills (id, name, slug, category, description, parent_skill_id)
        VALUES (?, ?, ?, ?, ?, ?)
    `
    _, err := c.db.ExecContext(ctx, query,
        skill.ID, skill.Name, skill.Slug, skill.Category,
        skill.Description, skill.ParentSkillID)
    return err
}

func (c *Client) FindEligibleAgents(ctx context.Context, issueID string) ([]string, error) {
    query := `
        SELECT DISTINCT a.agent_id
        FROM agent_skills a
        WHERE a.skill_id IN (
            SELECT skill_id
            FROM issue_skills
            WHERE issue_id = ? AND is_required = TRUE
        )
        AND a.proficiency_level IN ('intermediate', 'expert')
        GROUP BY a.agent_id
        HAVING COUNT(DISTINCT a.skill_id) = (
            SELECT COUNT(*)
            FROM issue_skills
            WHERE issue_id = ? AND is_required = TRUE
        )
    `

    rows, err := c.db.QueryContext(ctx, query, issueID, issueID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var agents []string
    for rows.Next() {
        var agentID string
        if err := rows.Scan(&agentID); err != nil {
            return nil, err
        }
        agents = append(agents, agentID)
    }
    return agents, nil
}
```

#### CLI Implementation

**New Command Group**: `grava skill`

File: `pkg/cmd/skill.go`

```go
package cmd

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"
    "github.com/yourusername/grava/pkg/dolt"
    "github.com/yourusername/grava/pkg/idgen"
    "github.com/yourusername/grava/pkg/models"
)

var skillCmd = &cobra.Command{
    Use:   "skill",
    Short: "Manage skills catalog",
    Long:  "Create, list, and manage skills for agent capability tracking",
}

var skillCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new skill",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        category, _ := cmd.Flags().GetString("category")
        description, _ := cmd.Flags().GetString("desc")

        client, err := dolt.NewClient(doltDSN)
        if err != nil {
            return err
        }
        defer client.Close()

        idGen := idgen.NewStandardGenerator("skill")
        skillID := idGen.GenerateBaseID()
        slug := toSlug(name) // Implement slug converter

        skill := &models.Skill{
            ID:          skillID,
            Name:        name,
            Slug:        slug,
            Category:    category,
            Description: description,
        }

        if err := client.CreateSkill(context.Background(), skill); err != nil {
            return err
        }

        fmt.Printf("Created skill: %s (%s)\n", skill.Name, skill.ID)
        return nil
    },
}

var skillListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all skills",
    RunE: func(cmd *cobra.Command, args []string) error {
        category, _ := cmd.Flags().GetString("category")

        client, err := dolt.NewClient(doltDSN)
        if err != nil {
            return err
        }
        defer client.Close()

        skills, err := client.ListSkills(context.Background(), category)
        if err != nil {
            return err
        }

        fmt.Printf("%-20s %-30s %-15s\n", "ID", "Name", "Category")
        fmt.Println("-------------------------------------------------------------------")
        for _, s := range skills {
            fmt.Printf("%-20s %-30s %-15s\n", s.ID, s.Name, s.Category)
        }
        return nil
    },
}

func init() {
    rootCmd.AddCommand(skillCmd)

    skillCmd.AddCommand(skillCreateCmd)
    skillCreateCmd.Flags().String("category", "tool", "Skill category")
    skillCreateCmd.Flags().String("desc", "", "Skill description")

    skillCmd.AddCommand(skillListCmd)
    skillListCmd.Flags().String("category", "", "Filter by category")
}
```

**Extend `grava create`**

File: `pkg/cmd/create.go` (additions)

```go
// Add flags
createCmd.Flags().StringSlice("require-skills", []string{}, "Required skills (slugs, comma-separated)")
createCmd.Flags().String("skill-level", "intermediate", "Required proficiency level")

// In RunE function, after issue creation:
requiredSkills, _ := cmd.Flags().GetStringSlice("require-skills")
skillLevel, _ := cmd.Flags().GetString("skill-level")

for _, skillSlug := range requiredSkills {
    skill, err := client.GetSkillBySlug(ctx, skillSlug)
    if err != nil {
        return fmt.Errorf("skill '%s' not found: %w", skillSlug, err)
    }

    if err := client.AddIssueSkill(ctx, issueID, skill.ID, skillLevel, true); err != nil {
        return err
    }
}
```

**New Command**: `grava agent register-skill`

File: `pkg/cmd/agent.go`

```go
package cmd

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
    Use:   "agent",
    Short: "Manage agent capabilities",
}

var agentRegisterSkillCmd = &cobra.Command{
    Use:   "register-skill <agent-id> <skill-slug>",
    Short: "Register a skill for an agent",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        agentID := args[0]
        skillSlug := args[1]
        proficiency, _ := cmd.Flags().GetString("level")

        client, err := dolt.NewClient(doltDSN)
        if err != nil {
            return err
        }
        defer client.Close()

        skill, err := client.GetSkillBySlug(context.Background(), skillSlug)
        if err != nil {
            return fmt.Errorf("skill not found: %w", err)
        }

        err = client.RegisterAgentSkill(context.Background(), agentID, skill.ID, proficiency)
        if err != nil {
            return err
        }

        fmt.Printf("Registered skill '%s' for agent '%s' at '%s' level\n",
            skill.Name, agentID, proficiency)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(agentCmd)

    agentCmd.AddCommand(agentRegisterSkillCmd)
    agentRegisterSkillCmd.Flags().String("level", "intermediate", "Proficiency level")
}
```

#### Testing Strategy

File: `pkg/dolt/skills_store_test.go`

```go
package dolt

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/yourusername/grava/pkg/models"
)

func TestCreateSkill(t *testing.T) {
    client := setupTestDB(t)
    defer client.Close()

    skill := &models.Skill{
        ID:       "skill-go",
        Name:     "Go Programming",
        Slug:     "go",
        Category: models.CategoryLanguage,
    }

    err := client.CreateSkill(context.Background(), skill)
    assert.NoError(t, err)

    // Verify retrieval
    retrieved, err := client.GetSkillBySlug(context.Background(), "go")
    assert.NoError(t, err)
    assert.Equal(t, "Go Programming", retrieved.Name)
}

func TestFindEligibleAgents(t *testing.T) {
    client := setupTestDB(t)
    defer client.Close()

    // Setup: Create skills, issue, and agents
    // ... (create test data)

    // Test: Find agents who have all required skills
    agents, err := client.FindEligibleAgents(context.Background(), "issue-123")
    assert.NoError(t, err)
    assert.Contains(t, agents, "claude-opus")
    assert.NotContains(t, agents, "agent-without-skills")
}
```

File: `pkg/dolt/skills_store_integration_test.go`

```go
// +build integration

package dolt

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestSkillWorkflow_Integration(t *testing.T) {
    // Full workflow test:
    // 1. Create skills
    // 2. Create issue with skill requirements
    // 3. Register agent with skills
    // 4. Query eligible agents
    // 5. Verify skill events logged
}
```

#### Migration Script

File: `scripts/migrate_skills.sh`

```bash
#!/bin/bash

set -e

DOLT_DIR=".grava/dolt"
SCHEMA_FILE="scripts/schema/002_skills_schema.sql"

echo "Applying skills schema migration..."

dolt --data-dir "$DOLT_DIR" sql < "$SCHEMA_FILE"

echo "Creating initial skills catalog..."

dolt --data-dir "$DOLT_DIR" sql <<EOF
INSERT INTO skills (id, name, slug, category) VALUES
    ('skill-go', 'Go Programming', 'go', 'language'),
    ('skill-sql', 'SQL', 'sql', 'language'),
    ('skill-dolt', 'Dolt Database', 'dolt', 'tool'),
    ('skill-git', 'Git Version Control', 'git', 'tool'),
    ('skill-docker', 'Docker', 'docker', 'tool'),
    ('skill-kubernetes', 'Kubernetes', 'k8s', 'tool');
EOF

echo "Committing migration..."

dolt --data-dir "$DOLT_DIR" add .
dolt --data-dir "$DOLT_DIR" commit -m "Add skills schema and initial catalog"

echo "Migration complete!"
```

#### Pros
- ✅ Full ACID guarantees
- ✅ Efficient querying with indexes
- ✅ Supports complex matching logic
- ✅ Version-controlled via Dolt (branch/merge skills)
- ✅ Audit trail via `skill_events`
- ✅ Foreign key integrity
- ✅ Supports hierarchical skills (parent_skill_id)
- ✅ Track agent performance (success/failure counts)

#### Cons
- ❌ Requires schema migration
- ❌ More complex implementation (2-3 days)
- ❌ Need to manage skill catalog

#### When to Use
- Production deployments
- > 1000 issues
- Multi-agent coordination
- Advanced "Ready Engine" logic

---

### Option 3: Hybrid Approach

**Strategy**: Use metadata field for requirements, but create a normalized skills catalog for validation.

#### Schema Changes

File: `scripts/schema/002_skills_catalog.sql`

```sql
CREATE TABLE skills_catalog (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    slug VARCHAR(128) NOT NULL UNIQUE,
    category VARCHAR(64),
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_slug (slug)
);
```

#### Data Structure

**Issues table**: Keep `metadata` field with:
```json
{
  "required_skills": ["go", "sql"],
  "skill_levels": {
    "go": "expert",
    "sql": "intermediate"
  }
}
```

**Agent profiles**: Use `agent_skills` table (same as Option 2).

#### Implementation Steps

1. Create minimal `skills_catalog` table
2. CLI for managing catalog: `grava skill create`, `grava skill list`
3. Validate skill references in `grava create --require-skills`
4. Store requirements in `metadata` JSON
5. Future migration: Move to full Option 2 model

#### Pros
- ✅ Quick to implement (1-2 days)
- ✅ Skills catalog provides validation
- ✅ Flexible requirements storage
- ✅ Clear migration path to Option 2

#### Cons
- ❌ Partial denormalization (skills in JSON + catalog)
- ❌ Limited query performance
- ❌ Requires eventual migration

#### When to Use
- Incremental adoption
- Unsure about final requirements
- Want to test concept before full commitment

---

## Ready Engine Integration (Epic 2 Extension)

### Current Ready Engine Logic
(From Epic 2 - Graph Mechanics)

1. Check issue status = 'open'
2. Check all dependencies are 'closed'
3. Check `await_*` gates are satisfied
4. Return eligible issues ordered by priority

### Enhanced Logic with Skills

File: `pkg/engine/ready.go` (proposed)

```go
package engine

import (
    "context"

    "github.com/yourusername/grava/pkg/dolt"
)

type ReadyEngine struct {
    store dolt.Store
}

// FindReadyIssues returns issues eligible for assignment
func (e *ReadyEngine) FindReadyIssues(ctx context.Context, agentID string) ([]string, error) {
    // Step 1: Graph-based eligibility (existing logic)
    graphEligible, err := e.getGraphEligibleIssues(ctx)
    if err != nil {
        return nil, err
    }

    // Step 2: Skill-based filtering
    var skillEligible []string
    for _, issueID := range graphEligible {
        if e.agentHasRequiredSkills(ctx, agentID, issueID) {
            skillEligible = append(skillEligible, issueID)
        }
    }

    // Step 3: Apply prioritization
    return e.prioritizeIssues(ctx, skillEligible), nil
}

func (e *ReadyEngine) agentHasRequiredSkills(ctx context.Context, agentID, issueID string) bool {
    // Query: Does agent have all required skills at sufficient proficiency?
    query := `
        SELECT COUNT(*) = (
            SELECT COUNT(*)
            FROM issue_skills
            WHERE issue_id = ? AND is_required = TRUE
        ) AS has_all_skills
        FROM issue_skills req
        LEFT JOIN agent_skills agent ON req.skill_id = agent.skill_id AND agent.agent_id = ?
        WHERE req.issue_id = ? AND req.is_required = TRUE
        AND (
            (req.proficiency_required = 'beginner' AND agent.proficiency_level IN ('beginner', 'intermediate', 'expert'))
            OR (req.proficiency_required = 'intermediate' AND agent.proficiency_level IN ('intermediate', 'expert'))
            OR (req.proficiency_required = 'expert' AND agent.proficiency_level = 'expert')
        )
    `

    var hasAllSkills bool
    _ = e.store.QueryRow(ctx, query, issueID, agentID, issueID).Scan(&hasAllSkills)
    return hasAllSkills
}
```

---

## MCP Integration (Epic 6 Extension)

### New MCP Tools

File: `pkg/mcp/skills_tools.go` (proposed)

```go
package mcp

// Tool: register_skill
// Allows agents to declare their capabilities
type RegisterSkillInput struct {
    AgentID     string `json:"agent_id"`
    SkillSlug   string `json:"skill_slug"`
    Proficiency string `json:"proficiency"` // beginner, intermediate, expert
}

// Tool: find_eligible_tasks
// Returns issues the agent is qualified to work on
type FindEligibleTasksInput struct {
    AgentID string `json:"agent_id"`
    Limit   int    `json:"limit"`
}

type FindEligibleTasksOutput struct {
    Issues []EligibleIssue `json:"issues"`
}

type EligibleIssue struct {
    ID               string   `json:"id"`
    Title            string   `json:"title"`
    Priority         int      `json:"priority"`
    RequiredSkills   []string `json:"required_skills"`
    MatchScore       float64  `json:"match_score"` // 0-1, based on skill overlap
}

// Tool: analyze_skill_gap
// Identifies missing skills preventing task assignment
type AnalyzeSkillGapInput struct {
    AgentID string `json:"agent_id"`
}

type AnalyzeSkillGapOutput struct {
    MissingSkills      []SkillDemand `json:"missing_skills"`
    RecommendedLearning []string      `json:"recommended_learning"`
}

type SkillDemand struct {
    SkillName   string `json:"skill_name"`
    IssueCount  int    `json:"issue_count"`  // How many issues need this skill
    Priority    float64 `json:"priority"`    // Weighted by issue priorities
}
```

---

## Analytics & Reporting (Epic 7 Extension)

### Skill-Based Metrics

**1. Skill Demand Heatmap**
```sql
SELECT
    s.name AS skill_name,
    COUNT(DISTINCT i.id) AS issue_count,
    AVG(i.priority) AS avg_priority,
    SUM(CASE WHEN i.status = 'open' THEN 1 ELSE 0 END) AS open_count
FROM skills s
JOIN issue_skills isk ON s.id = isk.skill_id
JOIN issues i ON isk.issue_id = i.id
GROUP BY s.id
ORDER BY open_count DESC, avg_priority ASC
```

**2. Agent Skill Coverage**
```sql
SELECT
    a.agent_id,
    COUNT(DISTINCT a.skill_id) AS skill_count,
    COUNT(DISTINCT CASE WHEN a.proficiency_level = 'expert' THEN a.skill_id END) AS expert_skills,
    AVG(a.success_count * 1.0 / NULLIF(a.success_count + a.failure_count, 0)) AS success_rate
FROM agent_skills a
GROUP BY a.agent_id
```

**3. Skill Bottleneck Analysis**
```sql
-- Skills with high demand but low supply
SELECT
    s.name,
    demand.issue_count,
    supply.agent_count,
    (demand.issue_count * 1.0 / NULLIF(supply.agent_count, 0)) AS demand_supply_ratio
FROM skills s
LEFT JOIN (
    SELECT skill_id, COUNT(DISTINCT issue_id) AS issue_count
    FROM issue_skills
    WHERE is_required = TRUE
    GROUP BY skill_id
) demand ON s.id = demand.skill_id
LEFT JOIN (
    SELECT skill_id, COUNT(DISTINCT agent_id) AS agent_count
    FROM agent_skills
    WHERE proficiency_level IN ('intermediate', 'expert')
    GROUP BY skill_id
) supply ON s.id = supply.skill_id
ORDER BY demand_supply_ratio DESC
LIMIT 10
```

---

## Implementation Timeline

### Option 1: Metadata Approach
- **Day 1**: Documentation + CLI flags (4 hours)
- **Day 2**: Query helpers + tests (3 hours)
- **Total**: 1 day

### Option 2: Dedicated Tables (RECOMMENDED)
- **Week 1**: Schema + migrations + data models (2 days)
- **Week 2**: Data access layer + unit tests (2 days)
- **Week 3**: CLI commands + integration tests (2 days)
- **Week 4**: Ready Engine integration (1 day)
- **Total**: 7-10 days

### Option 3: Hybrid
- **Week 1**: Skills catalog + validation (1 day)
- **Week 2**: CLI + metadata storage (1 day)
- **Week 3**: Agent skills table + matching (1 day)
- **Total**: 3-4 days

---

## Risk Assessment

### Technical Risks

**Risk 1: Skill Taxonomy Proliferation**
- **Probability**: High
- **Impact**: Medium
- **Mitigation**:
  - Create governed skills catalog
  - Require approval for new skills
  - Periodic cleanup/consolidation

**Risk 2: Proficiency Level Subjectivity**
- **Probability**: High
- **Impact**: Medium
- **Mitigation**:
  - Track success/failure rates
  - Auto-adjust proficiency based on performance
  - Require verification for "expert" claims

**Risk 3: Query Performance at Scale**
- **Probability**: Medium
- **Impact**: High
- **Mitigation**:
  - Add indexes on foreign keys
  - Benchmark with 10k+ issues
  - Consider materialized views for agent eligibility

**Risk 4: Skill Catalog Maintenance**
- **Probability**: Medium
- **Impact**: Low
- **Mitigation**:
  - Auto-detect new skills from agent registrations
  - Periodic review workflow
  - Deprecate unused skills

### Operational Risks

**Risk 5: Migration Complexity**
- **Probability**: Medium (Option 2), Low (Option 1/3)
- **Impact**: High
- **Mitigation**:
  - Test migration on copy of production DB
  - Provide rollback script
  - Blue-green deployment

**Risk 6: Breaking Changes to Existing Workflows**
- **Probability**: Low
- **Impact**: High
- **Mitigation**:
  - Make skills optional (don't block issue creation)
  - Graceful degradation (fall back to manual assignment)
  - Feature flag for skill-based matching

---

## Decision Matrix

| Criteria                  | Option 1 | Option 2 | Option 3 |
|---------------------------|----------|----------|----------|
| Time to implement         | ✅ 1 day | ❌ 7-10 days | ⚠️ 3-4 days |
| Query performance         | ❌ Poor  | ✅ Excellent | ⚠️ Good |
| Type safety               | ❌ None  | ✅ Full | ⚠️ Partial |
| Relational integrity      | ❌ None  | ✅ Full | ⚠️ Partial |
| Audit trail               | ❌ Manual | ✅ Automated | ⚠️ Partial |
| Future extensibility      | ❌ Limited | ✅ High | ⚠️ Medium |
| Migration cost            | ✅ None  | ❌ High | ⚠️ Medium |
| Complexity                | ✅ Low   | ❌ High | ⚠️ Medium |
| Supports Ready Engine     | ⚠️ Basic | ✅ Full | ⚠️ Good |
| MCP integration readiness | ❌ Limited | ✅ Full | ⚠️ Good |

---

## Recommendation

**Choose Option 2 (Dedicated Skills Tables)** if:
- Building for production use
- Plan to support > 5 agents
- Need advanced task matching
- Want to integrate with Ready Engine (Epic 2)
- Timeline allows 1-2 weeks

**Choose Option 1 (Metadata)** if:
- Prototyping or MVP
- < 1000 issues
- Need immediate implementation (< 1 day)
- Unsure about long-term requirements

**Choose Option 3 (Hybrid)** if:
- Want to test concept quickly
- Need incremental adoption
- Plan to migrate to Option 2 later
- Want to minimize risk

---

## Seed Data (Starter Skills Catalog)

```sql
-- Programming Languages
INSERT INTO skills (id, name, slug, category) VALUES
    ('skill-go', 'Go Programming', 'go', 'language'),
    ('skill-python', 'Python', 'python', 'language'),
    ('skill-javascript', 'JavaScript', 'javascript', 'language'),
    ('skill-typescript', 'TypeScript', 'typescript', 'language'),
    ('skill-rust', 'Rust', 'rust', 'language'),
    ('skill-sql', 'SQL', 'sql', 'language');

-- Frameworks
INSERT INTO skills (id, name, slug, category) VALUES
    ('skill-react', 'React', 'react', 'framework'),
    ('skill-vue', 'Vue.js', 'vue', 'framework'),
    ('skill-django', 'Django', 'django', 'framework'),
    ('skill-fastapi', 'FastAPI', 'fastapi', 'framework');

-- Tools
INSERT INTO skills (id, name, slug, category) VALUES
    ('skill-git', 'Git Version Control', 'git', 'tool'),
    ('skill-docker', 'Docker', 'docker', 'tool'),
    ('skill-kubernetes', 'Kubernetes', 'k8s', 'tool'),
    ('skill-dolt', 'Dolt Database', 'dolt', 'tool'),
    ('skill-terraform', 'Terraform', 'terraform', 'tool'),
    ('skill-github-actions', 'GitHub Actions', 'gh-actions', 'tool');

-- Domains
INSERT INTO skills (id, name, slug, category) VALUES
    ('skill-ml', 'Machine Learning', 'ml', 'domain'),
    ('skill-devops', 'DevOps', 'devops', 'domain'),
    ('skill-security', 'Security Engineering', 'security', 'domain'),
    ('skill-data-eng', 'Data Engineering', 'data-eng', 'domain'),
    ('skill-frontend', 'Frontend Development', 'frontend', 'domain'),
    ('skill-backend', 'Backend Development', 'backend', 'domain');
```

---

## Next Steps

1. **Decision Point**: Choose Option 1, 2, or 3 based on timeline and requirements
2. **Create Epic**: Add to `docs/epics/Epic_9_Skills_Management.md`
3. **Break into Tasks**: Use Grava to track implementation tasks
4. **Implement**: Follow phased approach in chosen option
5. **Test**: Run full test suite before merging
6. **Document**: Update README and CLI reference

---

## References

- [Epic 1: Storage Substrate](Epic_1_Storage_Substrate.md) - Database layer
- [Epic 2: Graph Mechanics](Epic_2_Graph_Mechanics.md) - Ready Engine logic
- [Epic 6: MCP Integration](Epic_6_MCP_Integration.md) - Agent interface
- [Dolt Documentation](https://docs.dolthub.com/) - SQL schema and versioning

---

**Document Status**: Draft
**Last Updated**: 2026-02-18
**Next Review**: After Option selection
