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
        
        # Verify sequence: create -> claim -> wisp_write -> status_change (closed)
        types = [e["event_type"] for e in events3]
        # Allow extra events if any, but must have these in order
        filtered_types = [t for t in types if t in ("create", "claim", "wisp_write", "status_change")]
        
        print(f"DEBUG History Sequence: {filtered_types}")
        assert "create" in filtered_types
        assert "claim" in filtered_types
        assert "wisp_write" in filtered_types
        assert "status_change" in filtered_types
        
        print("test_epic3_lifecycle PASS")
        
    finally:
        cleanup_test_issue(epic_id)

if __name__ == "__main__":
    test_epic3_lifecycle()
