-- Story 8.1: File Reservation & Concurrent Edit Safety (FR-ECS-1a)

-- +goose Up
CREATE TABLE file_reservations (
    id           VARCHAR(12)   NOT NULL PRIMARY KEY,
    project_id   VARCHAR(12)   NOT NULL,
    agent_id     VARCHAR(255)  NOT NULL,
    path_pattern VARCHAR(1024) NOT NULL,
    `exclusive`  BOOLEAN       NOT NULL DEFAULT TRUE,
    reason       TEXT,
    created_ts   DATETIME      NOT NULL DEFAULT (NOW()),
    expires_ts   DATETIME      NOT NULL,
    released_ts  DATETIME,

    INDEX idx_fr_project_active (project_id, released_ts, expires_ts)
);
