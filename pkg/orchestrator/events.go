package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// writeEvent inserts a structured audit event into the events table.
// oldVal and newVal are JSON-marshalled before storage.
// Errors are non-fatal — callers should log and continue.
func writeEvent(ctx context.Context, store dolt.Store, issueID, eventType, actor string, oldVal, newVal any) error {
	oldJSON, err := json.Marshal(oldVal)
	if err != nil {
		return fmt.Errorf("writeEvent: marshal old_value: %w", err)
	}
	newJSON, err := json.Marshal(newVal)
	if err != nil {
		return fmt.Errorf("writeEvent: marshal new_value: %w", err)
	}
	const q = `INSERT INTO events (issue_id, event_type, actor, old_value, new_value) VALUES (?, ?, ?, ?, ?)`
	_, err = store.ExecContext(ctx, q, issueID, eventType, actor, string(oldJSON), string(newJSON))
	if err != nil {
		return fmt.Errorf("writeEvent %s %s: %w", eventType, issueID, err)
	}
	return nil
}
