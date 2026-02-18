-- Migration: Add audit/provenance columns to all tables
-- Based on Epic 1.1, Task 2.1

-- issues
ALTER TABLE issues
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown' COMMENT 'User or agent who created this row',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown' COMMENT 'User or agent who last modified this row',
  ADD COLUMN agent_model VARCHAR(128) COMMENT 'AI model identifier (e.g. gemini-2.5-pro, claude-4)';

-- dependencies
ALTER TABLE dependencies
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- events
ALTER TABLE events
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- child_counters
ALTER TABLE child_counters
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);

-- deletions
ALTER TABLE deletions
  ADD COLUMN created_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN updated_by VARCHAR(128) DEFAULT 'unknown',
  ADD COLUMN agent_model VARCHAR(128);
