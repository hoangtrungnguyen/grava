import sys
import os
from grava_test_utils import run_grava

os.environ["GRAVA_DB_URL"] = "root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"

def test_crash_resume():
    # 1. Create a persistent Epic
    code, out = run_grava(["create", "--title", "Persistent Epic", "--type", "epic", "--json"])
    if code != 0:
        print("Failed to create epic:", out)
        sys.exit(1)
    epic_id = out.get("id")

    # 2. Create an ephemeral Wisp parented to the epic
    code, wisp_out = run_grava(["create", "--title", "Ephemeral Sandbox Wisp", "--type", "task", "--ephemeral", "--parent", epic_id, "--json"])
    if code != 0:
        print("Failed to create wisp:", wisp_out)
        sys.exit(1)
    wisp_id = wisp_out.get("id")

    # 3. Update the wisp's internal state (Checkpoint)
    checkpoint_text = "In progress. Checkpoint: sandbox init complete."
    code, update_out = run_grava(["update", wisp_id, "--desc", checkpoint_text, "--json"])
    if code != 0:
        print("Failed to update wisp:", update_out)
        sys.exit(1)

    # 4. Fetch the wisp again and verify state (Simulates crash-resume since CLI is stateless)
    code, show_out = run_grava(["show", wisp_id, "--json"])
    if code != 0:
        print("Failed to show wisp:", show_out)
        sys.exit(1)
    
    assert show_out.get("description") == checkpoint_text, f"Description mismatch! Got: {show_out.get('description')}"
    
    print("test_crash_resume PASS")
    sys.exit(0)

if __name__ == "__main__":
    test_crash_resume()
