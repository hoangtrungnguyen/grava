-- Add work session tracking columns to issues table
-- Tracks when work starts and stops for cycle time measurement

ALTER TABLE issues ADD COLUMN started_at TIMESTAMP NULL DEFAULT NULL;
ALTER TABLE issues ADD COLUMN stopped_at TIMESTAMP NULL DEFAULT NULL;

-- Index on started_at for efficient querying of in-progress work
ALTER TABLE issues ADD INDEX idx_started_at (started_at);

-- Index on stopped_at for cycle time range queries (WHERE stopped_at BETWEEN ? AND ?)
ALTER TABLE issues ADD INDEX idx_stopped_at (stopped_at);
