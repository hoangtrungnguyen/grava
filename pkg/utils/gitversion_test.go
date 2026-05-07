package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndCheckGitVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "2.39.3 Apple Git",
			version: "git version 2.39.3 (Apple Git-146)",
			wantErr: false,
		},
		{
			name:    "2.17.0 minimum",
			version: "git version 2.17.0",
			wantErr: false,
		},
		{
			name:    "2.44.0",
			version: "git version 2.44.0",
			wantErr: false,
		},
		{
			name:    "3.0.0 future",
			version: "git version 3.0.0",
			wantErr: false,
		},
		{
			name:      "2.16.9 too old",
			version:   "git version 2.16.9",
			wantErr:   true,
			errSubstr: "below minimum required",
		},
		{
			name:      "1.9.0 very old",
			version:   "git version 1.9.0",
			wantErr:   true,
			errSubstr: "below minimum required",
		},
		{
			name:      "garbage input",
			version:   "not a version",
			wantErr:   true,
			errSubstr: "unexpected git version",
		},
		{
			name:      "empty string",
			version:   "",
			wantErr:   true,
			errSubstr: "unexpected git version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseAndCheckGitVersion(tt.version)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckGitVersion(t *testing.T) {
	// Real git should be >= 2.17 on any modern system
	err := CheckGitVersion()
	assert.NoError(t, err, "system git should meet minimum version requirement")
}
