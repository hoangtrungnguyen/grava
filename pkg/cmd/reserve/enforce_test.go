package reserve

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "src/cmd/issues/create.go", "src/cmd/issues/create.go", true},
		{"glob star", "src/cmd/issues/*.go", "src/cmd/issues/create.go", true},
		{"glob star no match", "src/cmd/issues/*.go", "src/cmd/issues/sub/create.go", false},
		{"glob star no match ext", "src/cmd/issues/*.go", "src/cmd/issues/create.ts", false},
		{"different dir", "pkg/utils/*.go", "src/cmd/issues/create.go", false},
		{"wildcard single char", "src/?.go", "src/a.go", true},
		{"wildcard single char no match", "src/?.go", "src/ab.go", false},
		{"exact no match", "src/cmd/issues/create.go", "src/cmd/issues/update.go", false},
		{"invalid pattern fallback exact", "src/[invalid", "src/[invalid", true},
		{"invalid pattern fallback no match", "src/[invalid", "src/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
