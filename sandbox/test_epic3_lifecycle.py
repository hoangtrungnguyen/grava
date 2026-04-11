import sys
import os
from grava_test_utils import run_grava, create_test_issue, cleanup_test_issue

def test_epic3_lifecycle():
    # 1. Create epic
    issue = create_test_issue("Lifecycle Epic Test")
    epic_id = issue["id"]
    
    try:
        # Step: Check initial history (should contain 'create')
        code, history = run_grava(["history", epic_id, "--json"])
        assert code == 0
        events = history.get("history", []) if isinstance(history, dict) else history
        assert any(e["event_type"] == "create" for e in events)

        # 2. Agent 1 claims epic
        code, claim_out = run_grava(["claim", epic_id, "--actor", "agent-1", "--json"])
        assert code == 0
        
        # 3. Agent 1 writes a wisp checkpoint
        # AC#3: Create issue -> claim -> write Wisp entries
        val1 = "step-1-init-complete"
        code, w1 = run_grava(["wisp", "write", epic_id, "checkpoint", val1, "--actor", "agent-1", "--json"])
        assert code == 0
        
        # 4. Agent 2 reads history and wisp entries before claiming
        # AC#3 requirement: "A second agent reads history before claiming → sees first agent's full context"
        code, history2 = run_grava(["history", epic_id, "--json"])
        assert code == 0
        events2 = history2.get("history", []) if isinstance(history2, dict) else history2
        
        # Verify Agent 1's claim and wisp_write are in history
        claim_found = False
        wisp_found = False
        for e in events2:
            if e["event_type"] == "claim" and e["actor"] == "agent-1":
                claim_found = True
            if e["event_type"] == "wisp_write" and e["actor"] == "agent-1":
                wisp_found = True
        
        assert claim_found, "Agent 1 claim not found in history"
        assert wisp_found, "Agent 1 wisp_write not found in history"
        
        # Verify Agent 2 can read the wisp entry
        code, r1 = run_grava(["wisp", "read", epic_id, "checkpoint", "--json"])
        assert code == 0
        assert r1["value"] == val1
        assert r1["written_by"] == "agent-1"

        # 5. Resolve epic
        code, update_epic_out = run_grava(["update", epic_id, "--status", "closed", "--actor", "agent-1", "--json"])
        assert code == 0

        # 6. Final history check
        code, history3 = run_grava(["history", epic_id, "--json"])
        assert code == 0
        events3 = history3.get("history", []) if isinstance(history3, dict) else history3

        # M4: Validate event sequence order (create → claim → wisp_write → status_change)
        types = [e["event_type"] for e in events3]
        filtered_types = [t for t in types if t in ("create", "claim", "wisp_write", "status_change")]

        print(f"DEBUG History Sequence: {filtered_types}")

        # Verify required events exist and appear in correct order
        assert len(filtered_types) >= 4, f"Expected at least 4 key events, got {len(filtered_types)}: {filtered_types}"

        # Check strict ordering (no backwards transitions)
        create_idx = next((i for i, t in enumerate(filtered_types) if t == "create"), -1)
        claim_idx = next((i for i, t in enumerate(filtered_types) if t == "claim"), -1)
        wisp_idx = next((i for i, t in enumerate(filtered_types) if t == "wisp_write"), -1)
        status_idx = next((i for i, t in enumerate(filtered_types) if t == "status_change"), -1)

        assert create_idx >= 0, "create event not found"
        assert claim_idx > create_idx, f"claim must follow create (create_idx={create_idx}, claim_idx={claim_idx})"
        assert wisp_idx > claim_idx, f"wisp_write must follow claim (claim_idx={claim_idx}, wisp_idx={wisp_idx})"
        assert status_idx > wisp_idx, f"status_change must follow wisp_write (wisp_idx={wisp_idx}, status_idx={status_idx})"
        
        print("test_epic3_lifecycle PASS")
        
    finally:
        cleanup_test_issue(epic_id)

if __name__ == "__main__":
    test_epic3_lifecycle()
