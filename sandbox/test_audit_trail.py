from grava_test_utils import run_grava, create_test_issue
import sys

def test_audit_trail():
    # 1. Create issue
    issue = create_test_issue("Audit Trail Test")
    issue_id = issue["id"]
    
    # 2. Perform operations
    run_grava(["claim", issue_id, "--actor", "agent-audit"])
    run_grava(["wisp", "write", issue_id, "checkpoint", "v1", "--actor", "agent-audit"])
    run_grava(["comment", issue_id, "Test comment", "--actor", "agent-audit"])
    run_grava(["update", issue_id, "--status", "closed", "--actor", "agent-audit"])
    
    # 3. Check history
    code, history = run_grava(["history", issue_id, "--json"])
    assert code == 0, f"Failed to get history: {history}"
    
    # History should be a list of events
    assert len(history) >= 4
    
    event_types = [e["event_type"] for e in history]
    # Check for known event types in events.go
    assert "claim" in event_types
    assert "wisp_write" in event_types
    assert "create" in event_types
    # Status change from open to in_progress (claim) and in_progress to closed
    # Actually claim might just be a 'claim' event. Update uses 'update'.
    
    print("test_audit_trail PASS")

if __name__ == "__main__":
    test_audit_trail()
    sys.exit(0)
