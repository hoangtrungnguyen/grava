import sys
import os
from grava_test_utils import run_grava

os.environ["GRAVA_DB_URL"] = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"

def test_epic3_lifecycle():
    # 1. Create epic
    code, epic_out = run_grava(["create", "--title", "Lifecycle Epic", "--type", "epic", "--json"])
    assert code == 0, f"Failed to create epic: {epic_out}"
    epic_id = epic_out.get("id")

    # 2. Claim epic
    code, claim_out = run_grava(["claim", epic_id, "--actor", "dev-agent", "--json"])
    assert code == 0, f"Failed to claim epic: {claim_out}"

    # 3. Create subtask wisp
    code, wisp_out = run_grava(["create", "--title", "Lifecycle Wisp", "--type", "task", "--ephemeral", "--parent", epic_id, "--json"])
    assert code == 0, f"Failed to create wisp: {wisp_out}"
    wisp_id = wisp_out.get("id")

    # 4. Resolve subtask
    code, update_wisp_out = run_grava(["update", wisp_id, "--status", "closed", "--json"])
    assert code == 0, f"Failed to update wisp: {update_wisp_out}"

    # 5. Merge/Close epic
    code, update_epic_out = run_grava(["update", epic_id, "--status", "closed", "--json"])
    assert code == 0, f"Failed to update epic: {update_epic_out}"

    # 6. Extract history
    code, history_out = run_grava(["history", epic_id, "--json"])
    assert code == 0, f"Failed to get history: {history_out}"
    
    # Assert progression (create -> claim -> closed)
    statuses = []
    events = history_out.get("history", []) if isinstance(history_out, dict) else history_out
    
    for event in events:
        if event.get("event_type") == "create" and "status" in event.get("details", {}):
            statuses.append(event["details"]["status"])
        elif event.get("event_type") in ("claim", "status_change") and ("status" in event.get("details", {}) or "value" in event.get("details", {})):
            statuses.append(event["details"].get("status") or event["details"].get("value"))

    assert "in_progress" in statuses, f"Epic history missing 'in_progress' transition: {statuses}\nEvents: {events}"
    assert "closed" in statuses, f"Epic history missing 'closed' transition: {statuses}"
    
    print("test_epic3_lifecycle PASS")
    sys.exit(0)

if __name__ == "__main__":
    test_epic3_lifecycle()
