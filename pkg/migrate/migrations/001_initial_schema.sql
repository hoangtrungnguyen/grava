-- Based on EPIC-1 and TASK-1-2

-- +goose Up
-- 1. Issues Table
-- The primary ledger for all project objectives.
CREATE TABLE issues (
    id VARCHAR(32) NOT NULL PRIMARY KEY COMMENT 'Hierarchical unique identifier (e.g. grava-a1b2.1)',
    title VARCHAR(255) NOT NULL COMMENT 'Concise summary',
    description LONGTEXT COMMENT 'Detailed acceptance criteria',
    status VARCHAR(32) NOT NULL DEFAULT 'open' COMMENT 'Operational state',
    priority INT NOT NULL DEFAULT 4 COMMENT '0=Critical to 4=Backlog',
    issue_type VARCHAR(32) NOT NULL DEFAULT 'task' COMMENT 'Category: bug, feature, task, epic, chore, message',
    assignee VARCHAR(128) COMMENT 'Agent or user identity',
    metadata JSON COMMENT 'Extensible schema-less payload',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    ephemeral BOOLEAN DEFAULT FALSE COMMENT 'If true, excluded from exports (Wisp)',
    await_type VARCHAR(32) COMMENT 'Gate condition type: gh:pr, timer, human',
    await_id VARCHAR(128) COMMENT 'Gate condition identifier',

    CONSTRAINT check_priority CHECK (priority BETWEEN 0 AND 4),
    CONSTRAINT check_status CHECK (status IN ('open', 'in_progress', 'blocked', 'closed', 'tombstone', 'deferred', 'pinned')),
    CONSTRAINT check_issue_type CHECK (issue_type IN ('bug', 'feature', 'task', 'epic', 'chore', 'message'))
);

-- 2. Dependencies Table
-- Defines the directed edges of the project knowledge graph.
CREATE TABLE dependencies (
    from_id VARCHAR(32) NOT NULL,
    to_id VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL COMMENT 'Semantic edge type (blocks, parent-child, etc.)',
    metadata JSON COMMENT 'Context for the relationship',

    PRIMARY KEY (from_id, to_id, type),
    FOREIGN KEY (from_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (to_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_to_id (to_id)
);

-- 3. Events (Audit) Table
-- Append-only ledger capturing atomic mutations for forensic observability.
CREATE TABLE events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    event_type VARCHAR(64) NOT NULL COMMENT 'mutation type: create, update, delete, transition',
    actor VARCHAR(128) NOT NULL COMMENT 'Agent or user identity',
    old_value JSON COMMENT 'State before mutation',
    new_value JSON COMMENT 'State after mutation',
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_issue_timestamp (issue_id, timestamp)
);

-- 4. Child Counters Table
-- Tracks the next available suffix for hierarchical IDs to ensure atomicity.
CREATE TABLE child_counters (
    parent_id VARCHAR(32) NOT NULL PRIMARY KEY COMMENT 'The parent ID (or root namespace)',
    next_child INT NOT NULL DEFAULT 1 COMMENT 'The next suffix integer to use'
);

-- 5. Deletions Table
-- Tombstone manifest for tracking deleted IDs to prevent resurrection.
CREATE TABLE deletions (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    deleted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reason TEXT,
    actor VARCHAR(128)
);
