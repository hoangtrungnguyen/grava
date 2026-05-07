-- +goose Up
-- conflict_records: persists merge conflict entries detected by grava merge-driver.
-- Populated when grava conflicts resolve/dismiss is called; imported from
-- .grava/conflicts.json which the merge-driver writes without DB access.
CREATE TABLE conflict_records (
    id          VARCHAR(16)  NOT NULL PRIMARY KEY COMMENT 'Short hash: sha1(issue_id + field)[:8]',
    issue_id    VARCHAR(32)  NOT NULL COMMENT 'The issue ID involved in the conflict',
    field       VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Field name; empty string means whole-issue conflict',
    local_val   TEXT COMMENT 'JSON-encoded value on the current branch at conflict time',
    remote_val  TEXT COMMENT 'JSON-encoded value on the other branch at conflict time',
    status      VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending | resolved | dismissed',
    detected_at TIMESTAMP    NOT NULL COMMENT 'When the conflict was first detected',
    resolved_at TIMESTAMP    NULL COMMENT 'When the conflict was resolved or dismissed',
    resolution  VARCHAR(16)  NULL COMMENT 'ours | theirs | dismissed',

    CONSTRAINT check_status     CHECK (status IN ('pending', 'resolved', 'dismissed')),
    CONSTRAINT check_resolution CHECK (resolution IS NULL OR resolution IN ('ours', 'theirs', 'dismissed')),

    INDEX idx_conflict_status   (status),
    INDEX idx_conflict_issue_id (issue_id)
);

-- +goose Down
DROP TABLE IF EXISTS conflict_records;
