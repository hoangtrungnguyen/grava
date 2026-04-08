from grava_test_utils import run_grava, create_test_issue
import sys

def test_subtasks():
    # 1. Create Parent Epic
    code, epic = run_grava(["create", "--title", "Parent Epic", "--type", "epic", "--json"])
    assert code == 0, f"Failed to create epic: {epic}"
    epic_id = epic["id"]
    
    # 2. Create Subtasks
    code, st1 = run_grava(["subtask", epic_id, "--title", "Subtask 1", "--json"])
    assert code == 0, f"Failed to create subtask 1: {st1}"
    st1_id = st1["id"]
    assert st1_id.startswith(epic_id), f"Expected hierarchical ID {st1_id} to start with {epic_id}"
    
    code, st2 = run_grava(["subtask", epic_id, "--title", "Subtask 2", "--json"])
    assert code == 0, f"Failed to create subtask 2: {st2}"
    st2_id = st2["id"]
    
    # 3. Verify Tree Visualization
    # The output contains ANSI sequences and issue IDs. We'll just check for simple strings.
    code, tree_out = run_grava(["show", epic_id, "--tree"])
    assert code == 0, f"Failed to show tree: {tree_out}"
    assert "Subtask 1" in tree_out
    assert "Subtask 2" in tree_out
    assert st1_id in tree_out
    assert st2_id in tree_out
    
    print("test_subtasks PASS")

if __name__ == "__main__":
    test_subtasks()
