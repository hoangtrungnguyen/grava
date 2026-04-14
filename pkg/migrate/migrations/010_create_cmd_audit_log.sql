-- Story 9.1: Command History Ledger (FR14)

-- +goose Up
CREATE TABLE cmd_audit_log (
    id         VARCHAR(12)  NOT NULL PRIMARY KEY,
    command    VARCHAR(255) NOT NULL,
    actor      VARCHAR(255) NOT NULL DEFAULT 'unknown',
    args_json  TEXT,
    exit_code  INT          NOT NULL DEFAULT 0,
    created_at DATETIME     NOT NULL DEFAULT (NOW()),

    INDEX idx_cal_actor   (actor),
    INDEX idx_cal_created (created_at)
);
