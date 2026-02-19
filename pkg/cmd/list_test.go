package cmd

import (
	"testing"
)

func TestParseSortFlag(t *testing.T) {
	tests := []struct {
		name     string
		sortStr  string
		expected string
		wantErr  bool
	}{
		{
			name:     "Empty sort string (default)",
			sortStr:  "",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Single field asc",
			sortStr:  "priority:asc",
			expected: "priority ASC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Single field desc",
			sortStr:  "priority:desc",
			expected: "priority DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Multiple fields",
			sortStr:  "priority:asc,created:desc",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:     "Format with spaces",
			sortStr:  " priority : asc , created : desc ",
			expected: "priority ASC, created_at DESC, id ASC",
			wantErr:  false,
		},
		{
			name:    "Invalid field",
			sortStr: "unknown:asc",
			wantErr: true,
		},
		{
			name:    "Invalid order",
			sortStr: "priority:up",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSortFlag(tt.sortStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSortFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseSortFlag() = %q, want %q", got, tt.expected)
			}
		})
	}
}
