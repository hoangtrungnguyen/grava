import sys
import threading
from grava_test_utils import run_grava, create_test_issue
import os

os.environ["GRAVA_DB_URL"] = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"

def claim_issue(issue_id, actor, results):
    code, out = run_grava(["claim", issue_id, "--actor", actor, "--json"])
    results.append((actor, code, out))

def test_rapid_claims():
    issue = create_test_issue("Rapid Claim Test Issue")
    issue_id = issue.get("id")
    if not issue_id:
        print("Failed to get issue ID:", issue)
        sys.exit(1)

    results = []
    t1 = threading.Thread(target=claim_issue, args=(issue_id, "agent-1", results))
    t2 = threading.Thread(target=claim_issue, args=(issue_id, "agent-2", results))
    
    t1.start()
    t2.start()
    
    t1.join()
    t2.join()

    successes = 0
    failures = 0
    for actor, code, out in results:
        if code == 0:
            successes += 1
            if out.get("actor") != actor or out.get("status") != "in_progress":
                print(f"Unexpected out: {out}")
            assert out.get("actor") == actor
            assert out.get("status") == "in_progress"
        else:
            failures += 1
            err = out.get("error", {}) if isinstance(out, dict) else str(out)
            if isinstance(err, dict):
                print(f"Error Code: {err.get('code')}")
                assert err.get("code") in ("CLAIM_CONFLICT", "DB_COMMIT_FAILED")
            else:
                assert "ALREADY_CLAIMED" in err or "already claimed" in err.lower() or "conflict" in err.lower()
                
    if successes == 1 and failures == 1:
        print("test_rapid_claims PASS")
        sys.exit(0)
    else:
        print(f"test_rapid_claims FAIL. Successes: {successes}, Failures: {failures}")
        print(results)
        sys.exit(1)

if __name__ == "__main__":
    test_rapid_claims()
