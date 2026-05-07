package maintenance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctorCheck_Icon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"PASS", "✅"},
		{"WARN", "⚠️ "},
		{"FAIL", "❌"},
		{"", "❌"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			c := doctorCheck{Status: tt.status}
			assert.Equal(t, tt.expected, c.icon())
		})
	}
}

func TestDoctorCheck_JSONFields(t *testing.T) {
	c := doctorCheck{
		Name:   "db-connectivity",
		Status: "PASS",
		Detail: "Dolt v1.0.0",
	}
	assert.Equal(t, "db-connectivity", c.Name)
	assert.Equal(t, "PASS", c.Status)
	assert.Equal(t, "Dolt v1.0.0", c.Detail)
}
