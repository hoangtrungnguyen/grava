-- Add 'archived' to allowed issue statuses for soft-delete support

-- +goose Up
ALTER TABLE issues
  DROP CONSTRAINT check_status,
  ADD CONSTRAINT check_status CHECK (status IN ('open', 'in_progress', 'blocked', 'closed', 'tombstone', 'deferred', 'pinned', 'archived'));
