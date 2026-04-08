from grava_test_utils import run_grava, create_test_issue
import sys

def test_delete_vs_modify():
    # 1. Create issue
    issue = create_test_issue("Delete vs Modify Test")
    issue_id = issue["id"]
    
    # 2. Delete issue (using 'drop' or 'update --status tombstone')
    # Let's check if 'drop' command exists
    code, out = run_grava(["drop", issue_id, "--json"])
    if code != 0:
        # Maybe drop isn't implemented or requires interactive confirmation?
        # The show command says status is 'tombstone' when deleted.
        # Let's try update status tombstone if drop fails
        print(f"Drop failed or not available, trying update status: {out}")
        code, out = run_grava(["update", issue_id, "--status", "tombstone", "--json"])
        assert code == 0, f"Failed to delete issue: {out}"
    
    # 3. Try to claim it
    code, claim_out = run_grava(["claim", issue_id, "--actor", "agent-2", "--json"])
    assert code != 0, "Claiming a deleted issue should fail"
    
    err_code = claim_out.get("error", {}).get("code")
    # Based on claim.go, it might be ISSUE_NOT_FOUND if we filter tombstone out, 
    # or INVALID_STATUS_TRANSITION if it finds it but status is tombstone.
    # Actually claim.go does: SELECT status FROM issues WHERE id = ?
    # It doesn't filter tombstone out in the query, but checks status != 'open'.
    assert err_code == "INVALID_STATUS_TRANSITION" or "tombstone" in str(claim_out).lower()
    
    print("test_delete_vs_modify PASS")

if __name__ == "__main__":
    test_delete_vs_modify()
