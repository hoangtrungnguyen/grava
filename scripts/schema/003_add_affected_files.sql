-- Migration: Add affected_files column to issues table
-- Based on User Request to track files relevant to an issue

ALTER TABLE issues
  ADD COLUMN affected_files JSON COMMENT 'List of files impacted by this issue';
