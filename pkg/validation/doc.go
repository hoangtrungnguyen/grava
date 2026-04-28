// Package validation provides input validators for Grava issue fields.
//
// The package centralises the allow-lists and parsing rules used by every
// command and SQL writer that touches issue type, status, priority, or date
// ranges. Validators normalise input (lowercase + trim) before checking
// membership so callers can accept user input verbatim and still produce
// canonical values.
//
// Exposed maps (AllowedIssueTypes, AllowedStatuses, PriorityMap) are read by
// CLI flag completion and admin tooling; functions ValidateIssueType,
// ValidateStatus, ValidatePriority, and ValidateDateRange return descriptive
// errors suitable for surfacing directly to end users.
package validation
