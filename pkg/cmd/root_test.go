package cmd

import (
	"testing"
)

func TestIsReadOnlyCommand(t *testing.T) {
	// Read-only commands must not be recorded in cmd_audit_log.
	for _, name := range []string{"list", "show", "history", "ready", "blocked", "graph", "doctor", "cmd_history"} {
		if !isReadOnlyCommand(name) {
			t.Errorf("isReadOnlyCommand(%q) = false, want true", name)
		}
	}
	// Write commands must be recorded.
	for _, name := range []string{"create", "update", "claim", "comment", "label", "reserve"} {
		if isReadOnlyCommand(name) {
			t.Errorf("isReadOnlyCommand(%q) = true, want false", name)
		}
	}
}
