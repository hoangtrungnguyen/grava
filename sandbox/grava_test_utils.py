import subprocess
import json
import uuid
import os
import time

GRAVA_BIN = os.path.join(os.path.dirname(__file__), "grava_test_bin")

def ensure_built():
    """Ensure grava test binary is built and up-to-date."""
    root_dir = os.path.dirname(os.path.dirname(__file__))
    
    needs_build = not os.path.exists(GRAVA_BIN)
    if not needs_build:
        # Check all .go files in cmd and pkg for cache invalidation
        max_mtime = 0
        for dirpath, _, filenames in os.walk(os.path.join(root_dir, "pkg")):
            for f in filenames:
                if f.endswith(".go"):
                    max_mtime = max(max_mtime, os.path.getmtime(os.path.join(dirpath, f)))
        for dirpath, _, filenames in os.walk(os.path.join(root_dir, "cmd")):
            for f in filenames:
                if f.endswith(".go"):
                    max_mtime = max(max_mtime, os.path.getmtime(os.path.join(dirpath, f)))
        
        if max_mtime > os.path.getmtime(GRAVA_BIN):
            needs_build = True
        
    if needs_build:
        print("Building grava binary for testing...")
        subprocess.run(["go", "build", "-o", GRAVA_BIN, "./cmd/grava"], cwd=root_dir, check=True)

def run_grava(args, check=False):
    """Run a grava command and return the parsed JSON output or raw string if not JSON."""
    ensure_built()
    cmd = [GRAVA_BIN] + args

    db_url = os.environ.get("GRAVA_DB_URL")
    if not db_url:
        # Default to 3311 for this environment since server is on 3311
        db_url = "root@tcp(127.0.0.1:3311)/test_grava?parseTime=true"

    if "--db-url" not in args:
        cmd += ["--db-url", db_url]

    # print(f"Running: {' '.join(cmd)}") # Removed loud debug print (L1)
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

    if check and result.returncode != 0:
        # Sanitize command for error output (hide --db-url credentials)
        sanitized_cmd = []
        skip_next = False
        for i, arg in enumerate(cmd):
            if skip_next:
                skip_next = False
                continue
            if arg == "--db-url":
                sanitized_cmd.append("--db-url")
                sanitized_cmd.append("<redacted>")
                skip_next = True
            else:
                sanitized_cmd.append(arg)
        raise RuntimeError(f"Command failed: {' '.join(sanitized_cmd)}\nStdout: {result.stdout}\nStderr: {result.stderr}")
    
    if result.returncode != 0:
        # Check if output is JSON, even if it failed
        try:
            return result.returncode, json.loads(result.stdout.strip())
        except json.JSONDecodeError:
            try:
                return result.returncode, json.loads(result.stderr.strip())
            except json.JSONDecodeError:
                return result.returncode, result.stderr.strip()
                
    out = result.stdout.strip()
    if not out:
        out = result.stderr.strip()
        
    try:
        return result.returncode, json.loads(out)
    except json.JSONDecodeError:
        return result.returncode, out

def create_test_issue(title=None):
    """Creates a temporary issue for testing."""
    if title is None:
        title = f"Test Issue {uuid.uuid4()}"
    code, res = run_grava(["create", "--title", title, "--type", "story", "--json"])
    if code != 0:
        raise RuntimeError(f"Failed to create test issue: {res}")
    return res

def cleanup_test_issue(issue_id):
    """Permanently delete the test issue using the Archive & Purge lifecycle."""
    try:
        # 1. Mark as archived (Story 2.6)
        run_grava(["update", issue_id, "--status", "archived", "--json"])
        # 2. Trigger hard delete via maintenance clear
        run_grava(["clear", "--json"])
    except Exception as e:
        # Log but don't fail tests due to cleanup errors
        print(f"Cleanup warning for {issue_id}: {e}")

