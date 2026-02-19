package validation

import (
	"testing"
	"time"
)

func TestValidateIssueType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"task", true},
		{"bug", true},
		{"epic", true},
		{"FEATURE", true}, // Case insensitive
		{" invalid ", false},
		{"", false},
	}

	for _, tt := range tests {
		err := ValidateIssueType(tt.input)
		if (err == nil) != tt.expected {
			t.Errorf("ValidateIssueType(%q) expected success: %v, got error: %v", tt.input, tt.expected, err)
		}
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"open", true},
		{"in_progress", true},
		{"closed", true},
		{"BLOCKED", true}, // Case insensitive
		{"done", false},   // "done" is not a valid status in our schema (it's "closed")
		{"", false},
	}

	for _, tt := range tests {
		err := ValidateStatus(tt.input)
		if (err == nil) != tt.expected {
			t.Errorf("ValidateStatus(%q) expected success: %v, got error: %v", tt.input, tt.expected, err)
		}
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		input       string
		expectedInt int
		expectedErr bool
	}{
		{"critical", 0, false},
		{"high", 1, false},
		{"MEDIUM", 2, false}, // Case insensitive
		{"low", 3, false},
		{"backlog", 4, false},
		{"urgent", -1, true}, // Invalid
		{"", -1, true},
	}

	for _, tt := range tests {
		val, err := ValidatePriority(tt.input)
		if (err != nil) != tt.expectedErr {
			t.Errorf("ValidatePriority(%q) expected error: %v, got: %v", tt.input, tt.expectedErr, err)
		}
		if !tt.expectedErr && val != tt.expectedInt {
			t.Errorf("ValidatePriority(%q) expected value: %d, got: %d", tt.input, tt.expectedInt, val)
		}
	}
}

func TestValidateDateRange(t *testing.T) {
	tests := []struct {
		from        string
		to          string
		expectedErr bool
	}{
		{"2023-01-01", "2023-01-31", false},
		{"2023-01-01", "2023-01-01", false}, // Same day is valid
		{"2023-01-31", "2023-01-01", true},  // From > To
		{"2023-01-01", "invalid", true},
		{"invalid", "2023-01-01", true},
	}

	for _, tt := range tests {
		start, end, err := ValidateDateRange(tt.from, tt.to)
		if (err != nil) != tt.expectedErr {
			t.Errorf("ValidateDateRange(%q, %q) expected error: %v, got: %v", tt.from, tt.to, tt.expectedErr, err)
		}
		if !tt.expectedErr {
			if start.IsZero() || end.IsZero() {
				t.Error("Exepcted valid time objects, got zero value")
			}
		}
	}

	// Verify time parsing correctness
	start, _, _ := ValidateDateRange("2023-01-01", "2023-01-02")
	expectedDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	if !start.Equal(expectedDate) {
		t.Errorf("Date parsing failed. Got %v, expected %v", start, expectedDate)
	}
}
