-- Based on User Request to track files relevant to an issue

-- +goose Up
ALTER TABLE issues
  ADD COLUMN affected_files JSON COMMENT 'List of files impacted by this issue';
