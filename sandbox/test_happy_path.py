import threading
from grava_test_utils import run_grava, create_test_issue
import os
import sys

def claim_and_work(issue_id, actor, results):
    # 1. Claim issue
    code, out = run_grava(["claim", issue_id, "--actor", actor, "--json"])
    if code != 0:
        results.append((actor, False, f"Failed to claim: {out}"))
        return
    
    # 2. Add a comment (simulating work)
    code, comment_out = run_grava(["comment", issue_id, f"Work by {actor}", "--actor", actor, "--json"])
    if code != 0:
        results.append((actor, False, f"Failed to comment: {comment_out}"))
        return
        
    # 3. Close issue
    code, close_out = run_grava(["update", issue_id, "--status", "closed", "--actor", actor, "--json"])
    if code != 0:
        results.append((actor, False, f"Failed to close: {close_out}"))
        return
        
    results.append((actor, True, "Success"))

def test_happy_path():
    # Setup: 2 separate tasks
    issue1 = create_test_issue("Happy Path Task 1")
    issue2 = create_test_issue("Happy Path Task 2")
    
    issue1_id = issue1["id"]
    issue2_id = issue2["id"]
    
    results = []
    t1 = threading.Thread(target=claim_and_work, args=(issue1_id, "agent-1", results))
    t2 = threading.Thread(target=claim_and_work, args=(issue2_id, "agent-2", results))
    
    t1.start()
    t2.start()
    
    t1.join()
    t2.join()
    
    for actor, success, msg in results:
        if not success:
            print(f"{actor} failed: {msg}")
            sys.exit(1)
            
    print("test_happy_path PASS")
    sys.exit(0)

if __name__ == "__main__":
    test_happy_path()
