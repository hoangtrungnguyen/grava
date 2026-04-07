import subprocess
import json
import uuid
import os
import time

GRAVA_BIN = os.path.join(os.path.dirname(__file__), "grava_test_bin")

def ensure_built():
    """Ensure grava test binary is built."""
    if not os.path.exists(GRAVA_BIN):
        print("Building grava binary for testing...")
        root_dir = os.path.dirname(os.path.dirname(__file__))
        subprocess.run(["go", "build", "-o", GRAVA_BIN, "./cmd/grava"], cwd=root_dir, check=True)

def run_grava(args, check=False):
    """Run a grava command and return the parsed JSON output or raw string if not JSON."""
    ensure_built()
    cmd = [GRAVA_BIN] + args
    if "GRAVA_DB_URL" in os.environ and "--db-url" not in args:
        cmd += ["--db-url", os.environ["GRAVA_DB_URL"]]
    print(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True)
    
    if check and result.returncode != 0:
        raise RuntimeError(f"Command failed: {' '.join(cmd)}\nStdout: {result.stdout}\nStderr: {result.stderr}")
    
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
    pass # we might need to delete it from db or dolt 
