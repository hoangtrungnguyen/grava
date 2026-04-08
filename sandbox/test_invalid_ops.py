from grava_test_utils import run_grava, create_test_issue
import sys

def test_invalid_ops():
    # 1. Claim already claimed
    issue = create_test_issue("Already Claimed Test")
    issue_id = issue["id"]
    run_grava(["claim", issue_id, "--actor", "agent-1"])
    code, out = run_grava(["claim", issue_id, "--actor", "agent-2", "--json"])
    assert code != 0
    assert out["error"]["code"] == "ALREADY_CLAIMED"
    
    # 2. Wisp read missing key
    code, out = run_grava(["wisp", "read", issue_id, "missing-key", "--json"])
    assert "error" in out
    assert out["error"]["code"] == "WISP_NOT_FOUND"
    
    # 3. Wisp read missing issue
    code, out = run_grava(["wisp", "read", "missing-id", "--json"])
    assert "error" in out
    assert out["error"]["code"] == "ISSUE_NOT_FOUND"
    
    # 4. Update status invalid transition
    # Use a fresh issue that is NOT claimed (assignee is NULL)
    issue2 = create_test_issue("Status Transition Test")
    issue2_id = issue2["id"]
    # Move to closed directly (assignee is still NULL)
    run_grava(["update", issue2_id, "--status", "closed"])
    # Now try to claim a CLOSED issue
    code, out = run_grava(["claim", issue2_id, "--actor", "agent-1", "--json"])
    assert code != 0
    # In claim.go: if status != 'open' -> INVALID_STATUS_TRANSITION
    assert out["error"]["code"] == "INVALID_STATUS_TRANSITION"
    
    print("test_invalid_ops PASS")

if __name__ == "__main__":
    test_invalid_ops()
