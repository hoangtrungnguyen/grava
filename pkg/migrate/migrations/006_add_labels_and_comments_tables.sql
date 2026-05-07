-- Add dedicated tables for issue labels and comments
-- Previously stored in the metadata JSON column; now normalized for query performance

-- +goose Up

-- Issue Labels: normalized label storage with uniqueness constraint
CREATE TABLE issue_labels (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    label VARCHAR(128) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(128),

    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    UNIQUE KEY unique_issue_label (issue_id, label),
    INDEX idx_issue_labels_issue_id (issue_id)
);

-- Issue Comments: timestamped discussion entries
CREATE TABLE issue_comments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    issue_id VARCHAR(32) NOT NULL,
    message TEXT NOT NULL,
    actor VARCHAR(128),
    agent_model VARCHAR(256),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    INDEX idx_issue_comments_issue_id (issue_id)
);

-- +goose Down
DROP TABLE IF EXISTS issue_comments;
DROP TABLE IF EXISTS issue_labels;
