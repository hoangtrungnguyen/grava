package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IssueIDPattern is the canonical regex for a grava issue ID.
//
// Format: "grava-" + 4 lowercase hex chars, optionally followed by
// dot-separated decimal segments for child IDs.
//
// Examples:
//   - grava-a1b2          (top-level, base ID from GenerateBaseID)
//   - grava-a1b2.1        (first child)
//   - grava-a1b2.1.3      (nested child)
//
// Kept in sync with pkg/idgen/generator.go's GenerateBaseID + GenerateChildID.
var IssueIDPattern = regexp.MustCompile(`^grava-[a-f0-9]{4}(\.\d+)*$`)

// Allowed values for issue types and statuses
var (
	AllowedIssueTypes = map[string]bool{
		"task":    true,
		"bug":     true,
		"epic":    true,
		"story":   true,
		"feature": true,
		"chore":   true,
	}

	AllowedStatuses = map[string]bool{
		"open":        true,
		"in_progress": true,
		"closed":      true,
		"blocked":     true,
		"tombstone":   true,
		"archived":    true,
	}

	PriorityMap = map[string]int{
		"critical": 0,
		"high":     1,
		"medium":   2,
		"low":      3,
		"backlog":  4,
	}
)

// ValidateIssueType checks if the issue type is valid.
func ValidateIssueType(t string) error {
	normalized := strings.ToLower(strings.TrimSpace(t))
	if !AllowedIssueTypes[normalized] {
		return fmt.Errorf("invalid issue type: '%s'. Allowed: task, bug, epic, story, feature", t)
	}
	return nil
}

// ValidateStatus checks if the status is valid.
func ValidateStatus(s string) error {
	normalized := strings.ToLower(strings.TrimSpace(s))
	if !AllowedStatuses[normalized] {
		return fmt.Errorf("invalid status: '%s'. Allowed: open, in_progress, closed, blocked, archived", s)
	}
	return nil
}

// ValidatePriority checks if the priority is valid and returns its integer value.
// Accepts both named priorities (critical, high, medium, low, backlog) and
// numeric values (0-4). Returns an error with a clear message for out-of-range values.
func ValidatePriority(p string) (int, error) {
	normalized := strings.ToLower(strings.TrimSpace(p))
	val, ok := PriorityMap[normalized]
	if ok {
		return val, nil
	}

	// Try parsing as an integer for numeric priority values.
	if n, err := strconv.Atoi(normalized); err == nil {
		if n < 0 || n > 4 {
			return -1, fmt.Errorf("priority value %d is out of range (0-4). Allowed: 0=critical, 1=high, 2=medium, 3=low, 4=backlog", n)
		}
		return n, nil
	}

	return -1, fmt.Errorf("invalid priority: '%s'. Allowed: critical (0), high (1), medium (2), low (3), backlog (4)", p)
}

// ValidateIssueID checks that an issue ID matches the canonical grava format.
//
// Used by callers that mint IDs outside the standard idgen path — e.g.
// `grava create --id <value>`, where an external system (Plane mirror,
// migration, etc.) supplies the ID.
//
// Whitespace is trimmed before validation; the input is NOT lower-cased
// because the regex itself enforces lowercase hex.
func ValidateIssueID(id string) error {
	normalized := strings.TrimSpace(id)
	if normalized == "" {
		return fmt.Errorf("issue id is empty")
	}
	if !IssueIDPattern.MatchString(normalized) {
		return fmt.Errorf(
			"invalid issue id: '%s'. Expected format: grava-XXXX (4 lowercase hex chars), "+
				"optionally followed by '.<digit>' child segments (e.g. grava-a1b2.1.3)",
			id,
		)
	}
	return nil
}

// ValidateDateRange parses and validates a date range (inclusive).
// It returns the parsed start and end times.
// Date format must be YYYY-MM-DD.
func ValidateDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	layout := "2006-01-02"

	from, err := time.Parse(layout, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' date format: '%s'. Expected YYYY-MM-DD", fromStr)
	}

	to, err := time.Parse(layout, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' date format: '%s'. Expected YYYY-MM-DD", toStr)
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("'from' date (%s) cannot be after 'to' date (%s)", fromStr, toStr)
	}

	return from, to, nil
}
