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

// TestIsSelfAuditingCommand pins the contract that `commit` records its own
// audit row inline (and is therefore skipped by PersistentPostRunE), but
// other write commands are NOT self-auditing — they still rely on PostRunE
// to write their audit rows. See grava-ff4b for context.
func TestIsSelfAuditingCommand(t *testing.T) {
	// commit must self-audit so cmd_audit_log ends clean.
	if !isSelfAuditingCommand("commit") {
		t.Errorf("isSelfAuditingCommand(\"commit\") = false, want true")
	}
	// Everything else must NOT self-audit — they rely on PostRunE.
	for _, name := range []string{"create", "update", "claim", "comment", "label", "reserve", "import", "export"} {
		if isSelfAuditingCommand(name) {
			t.Errorf("isSelfAuditingCommand(%q) = true, want false", name)
		}
	}
}
