from grava_test_utils import run_grava, create_test_issue, cleanup_test_issue
import sys
import time
import subprocess
import os

def run_sql(query):
    mysql_client = "/opt/homebrew/opt/mysql-client/bin/mysql"
    if not os.path.exists(mysql_client):
        # try common path
        mysql_client = "mysql" 
    
    db_url = os.environ.get("GRAVA_DB_URL", "root@tcp(127.0.0.1:3311)/test_grava?parseTime=true")
    
    # Simple parsing: root@tcp(127.0.0.1:3311)/test_grava
    # Extract host:port
    try:
        import re
        match = re.search(r'tcp\((.*?)\)', db_url)
        host_port = match.group(1) if match else "127.0.0.1:3311"
        host, port = host_port.split(":")
    except Exception:
        host, port = "127.0.0.1", "3311"

    cmd = [
        mysql_client,
        "-h", host,
        "-P", port,
        "-u", "root",
        "-D", "test_grava",
        "-e", query
    ]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        raise Exception(f"SQL failed: {res.stderr}")
    return res.stdout

def test_crash_resume():
    # 1. Agent 1 creates and claims an issue
    issue = create_test_issue("Crash-Resume Task")
    issue_id = issue["id"]
    
    try:
        code, c1 = run_grava(["claim", issue_id, "--actor", "agent-1", "--json"])
        assert code == 0, f"Agent 1 failed to claim: {c1}"
        
        # 2. Agent 1 makes partial progress (writes wisp)
        code, w1 = run_grava(["wisp", "write", issue_id, "checkpoint", "step-5-of-10", "--actor", "agent-1", "--json"])
        assert code == 0
        
        # 3. Simulated crash of Agent 1 
        print("Simulating Agent 1 crash (mocking TTL expiry)...")
        
        # SQL: Update wisp_heartbeat_at to 24 hours ago to bypass any TZ confusion
        sql_cmd = f"UPDATE issues SET wisp_heartbeat_at = DATE_SUB(NOW(), INTERVAL 24 HOUR) WHERE id = '{issue_id}'"
        run_sql(sql_cmd)
            
        # 4. Agent 2 attempts to claim the SAME task
        print("Agent 2 attempting to claim 'stale' task...")
        code, c2 = run_grava(["claim", issue_id, "--actor", "agent-2", "--json"])
        
        if code != 0:
            print(f"DEBUG: Claim failed with out: {c2}")
            
        assert code == 0, f"Agent 2 should have been able to claim stale task"
        assert c2["actor"] == "agent-2"
        
        # 5. Agent 2 reads Agent 1's wisp to resume
        code, r1 = run_grava(["wisp", "read", issue_id, "checkpoint", "--json"])
        assert code == 0
        assert r1["value"] == "step-5-of-10"
        assert r1["written_by"] == "agent-1"
        
        # 6. Agent 2 writes its own progress
        run_grava(["wisp", "write", issue_id, "checkpoint", "step-10-of-10 (completed)", "--actor", "agent-2"])
        
        # 7. Verify History shows BOTH agents
        code, history = run_grava(["history", issue_id, "--json"])
        assert code == 0
        
        actors = [e["actor"] for e in history]
        # Check for both agent actors
        assert "agent-1" in actors
        assert "agent-2" in actors
        
        print("test_crash_resume PASS")
        
    finally:
        cleanup_test_issue(issue_id)

if __name__ == "__main__":
    test_crash_resume()
