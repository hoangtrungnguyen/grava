package validation

import (
	"fmt"
	"strings"
	"time"
)

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
		return fmt.Errorf("invalid status: '%s'. Allowed: open, in_progress, closed, blocked", s)
	}
	return nil
}

// ValidatePriority checks if the priority is valid and returns its integer value.
func ValidatePriority(p string) (int, error) {
	normalized := strings.ToLower(strings.TrimSpace(p))
	val, ok := PriorityMap[normalized]
	if !ok {
		return -1, fmt.Errorf("invalid priority: '%s'. Allowed: critical, high, medium, low, backlog", p)
	}
	return val, nil
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
