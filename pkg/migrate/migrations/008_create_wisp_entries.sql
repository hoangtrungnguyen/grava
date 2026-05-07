-- Story 3.2: Wisp ephemeral state store

-- +goose Up
CREATE TABLE wisp_entries (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    key_name VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    written_by VARCHAR(128) NOT NULL,
    written_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_wisp_issue_key (issue_id, key_name),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_wisp_issue_id (issue_id)
);

ALTER TABLE issues ADD COLUMN wisp_heartbeat_at TIMESTAMP NULL;
