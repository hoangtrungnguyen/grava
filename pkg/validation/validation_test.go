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
		{"BLOCKED", true},  // Case insensitive
		{"archived", true}, // Soft-delete status (Story 2.6)
		{"ARCHIVED", true}, // Case insensitive
		{"done", false},    // "done" is not a valid status in our schema (it's "closed")
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
		{"0", 0, false},      // Numeric critical
		{"1", 1, false},      // Numeric high
		{"2", 2, false},      // Numeric medium
		{"3", 3, false},      // Numeric low
		{"4", 4, false},      // Numeric backlog
		{"-1", -1, true},     // Out of range (negative)
		{"5", -1, true},      // Out of range (too high)
		{"99", -1, true},     // Out of range (way too high)
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

func TestValidateIssueID(t *testing.T) {
	tests := []struct {
		input       string
		expectedErr bool
	}{
		// Valid: legacy top-level IDs (4 lowercase hex chars after `grava-`).
		// Pre-2026-05 grava emitted 4 hex; existing DBs still use this width.
		{"grava-a1b2", false},
		{"grava-0000", false},
		{"grava-ffff", false},
		{"grava-abcd", false},
		{"  grava-a1b2  ", false}, // surrounding whitespace is trimmed
		// Valid: current top-level IDs (8 lowercase hex chars after `grava-`).
		// 2026-05+ grava emits 8 hex for ~4.29B combinations.
		{"grava-a1b2c3d4", false},
		{"grava-00000000", false},
		{"grava-ffffffff", false},
		{"grava-deadbeef", false},
		{"  grava-a1b2c3d4  ", false}, // whitespace trim works on 8-hex too
		// Valid: child IDs (both widths).
		{"grava-a1b2.1", false},
		{"grava-a1b2.1.3", false},
		{"grava-a1b2.42.100.7", false},
		{"grava-a1b2c3d4.1", false},
		{"grava-a1b2c3d4.1.3", false},
		{"grava-deadbeef.42.100.7", false},
		// Invalid: empty / whitespace-only.
		{"", true},
		{"   ", true},
		// Invalid: wrong prefix.
		{"grava_a1b2", true},
		{"a1b2", true},
		{"plane-a1b2", true},
		// Invalid: wrong hex length. Only exactly 4 OR exactly 8 hex chars
		// are accepted — intermediate widths (5/6/7) and 9+ are rejected so
		// we don't silently accept truncated or padded IDs.
		{"grava-a1b", true},        // too short (3)
		{"grava-a1b2c", true},      // 5 hex
		{"grava-a1b2c3", true},     // 6 hex
		{"grava-a1b2c3d", true},    // 7 hex
		{"grava-a1b2c3d4e", true},  // 9 hex
		{"grava-a1b2c3d4ef", true}, // 10 hex
		// Invalid: uppercase hex (idgen always emits lowercase).
		{"grava-A1B2", true},
		{"grava-A1B2C3D4", true},
		// Invalid: non-hex chars.
		{"grava-zzzz", true},
		{"grava-a1b!", true},
		{"grava-zzzzzzzz", true},
		{"grava-a1b2c3d!", true},
		// Invalid: child segment is not a positive integer.
		{"grava-a1b2.", true},
		{"grava-a1b2.a", true},
		{"grava-a1b2..1", true},
		{"grava-a1b2.1.", true},
	}

	for _, tt := range tests {
		err := ValidateIssueID(tt.input)
		if (err != nil) != tt.expectedErr {
			t.Errorf("ValidateIssueID(%q) expected error: %v, got: %v",
				tt.input, tt.expectedErr, err)
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
