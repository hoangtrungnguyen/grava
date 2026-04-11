import sys
import threading
from grava_test_utils import run_grava, create_test_issue, cleanup_test_issue
import os
import time

def claim_issue(issue_id, actor, results, lock):
    code, out = run_grava(["claim", issue_id, "--actor", actor, "--json"])
    with lock:
        results.append((actor, code, out))

def test_rapid_claims():
    issue = create_test_issue("Rapid Claim Test Issue")
    issue_id = issue.get("id")
    if not issue_id:
        print("Failed to get issue ID:", issue)
        sys.exit(1)

    try:
        # M1: Verify initial state before concurrent operations (AC#1 precondition)
        code, initial_state = run_grava(["show", issue_id, "--json"])
        assert code == 0, f"Failed to query initial state: {initial_state}"
        assert initial_state.get("status") == "open", f"Expected initial status=open, got {initial_state.get('status')}"

        results = []
        results_lock = threading.Lock()
        t1 = threading.Thread(target=claim_issue, args=(issue_id, "agent-1", results, results_lock))
        t2 = threading.Thread(target=claim_issue, args=(issue_id, "agent-2", results, results_lock))
        
        t1.start()
        t2.start()
        
        # H3: Timeout protection (Exactly 5s total as per AC#1)
        start_time = os.times().elapsed if hasattr(os, 'times') else 0 # Simple fallback
        import time
        start_time = time.time()
        
        t1.join(timeout=5)
        remaining = max(0, 5 - (time.time() - start_time))
        t2.join(timeout=remaining)
        
        if t1.is_alive() or t2.is_alive():
            print(f"ERROR: Deadlock or timeout exceeded (5s). t1_alive: {t1.is_alive()}, t2_alive: {t2.is_alive()}")
            sys.exit(1)

        successes = 0
        failures = 0
        winner_actor = None

        for actor, code, out in results:
            if code == 0:
                successes += 1
                winner_actor = actor
                assert out.get("actor") == actor
                assert out.get("status") == "in_progress"
            else:
                failures += 1
                err = out.get("error", {}) if isinstance(out, dict) else {"message": str(out)}
                # H2: Enforce ALREADY_CLAIMED (AC#1 strictness)
                assert err.get("code") == "ALREADY_CLAIMED", f"Expected ALREADY_CLAIMED but got {err.get('code')}: {err.get('message')}"
                    
        # H1: Verify final DB state via 'grava show' (AC#1 consistency check)
        # Exactly one assignee must exist and status must be in_progress
        code, show_out = run_grava(["show", issue_id, "--json"])
        assert code == 0
        assert show_out.get("status") == "in_progress", f"Expected in_progress, got {show_out.get('status')}"
        assert show_out.get("assignee") == winner_actor, f"Expected assignee to be {winner_actor}, got {show_out.get('assignee')}"

        # Additional verification: only one claim event in history
        code, history = run_grava(["history", issue_id, "--json"])
        assert code == 0
        claim_events = [e for e in history if e["event_type"] == "claim"]
        assert len(claim_events) == 1, f"Expected exactly 1 claim event, found {len(claim_events)}"
        assert claim_events[0]["actor"] == winner_actor

        if successes == 1 and failures == 1:
            print("test_rapid_claims PASS")
        else:
            print(f"test_rapid_claims FAIL. Successes: {successes}, Failures: {failures}")
            print(results)
            sys.exit(1)
            
    finally:
        # M2: Cleanup
        cleanup_test_issue(issue_id)

if __name__ == "__main__":
    test_rapid_claims()
