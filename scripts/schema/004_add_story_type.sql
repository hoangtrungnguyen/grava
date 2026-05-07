-- Add 'story' to allowed issue types

-- +goose Up
ALTER TABLE issues
  DROP CONSTRAINT check_issue_type,
  ADD CONSTRAINT check_issue_type CHECK (issue_type IN ('bug', 'feature', 'task', 'epic', 'chore', 'message', 'story'));
