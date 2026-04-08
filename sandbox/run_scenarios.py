import subprocess
import os
import sys
import time

def run_test(name, path):
    print(f"Running {name}...")
    start = time.time()
    result = subprocess.run(["python3", path], capture_output=True, text=True)
    duration = time.time() - start
    if result.returncode == 0:
        print(f"✅ {name:25} PASSED ({duration:.2f}s)")
        return True
    else:
        print(f"❌ {name:25} FAILED ({duration:.2f}s)")
        print("--- Stdout ---")
        print(result.stdout)
        print("--- Stderr ---")
        print(result.stderr)
        return False

def main():
    root_dir = os.path.dirname(os.path.dirname(__file__))
    tests = [
        ("01 Happy Path",              "sandbox/test_happy_path.py"),
        ("02 Atomic Claim Race",       "sandbox/test_rapid_claims.py"),
        ("03 Crash & Resume (Wisp)",   "sandbox/test_crash_resume.py"),
        ("04 Issue Hierarchy",         "sandbox/test_subtasks.py"),
        ("05 Audit History",           "sandbox/test_audit_trail.py"),
        ("06 Delete vs Modify",        "sandbox/test_delete_vs_modify.py"),
        ("07 Invalid Operations",      "sandbox/test_invalid_ops.py"),
        ("08 Sequential Persistence",  "sandbox/test_epic3_lifecycle.py"),
    ]
    
    all_passed = True
    results = []
    
    print("\n" + "="*45)
    print("GRAVA SANDBOX VALIDATION SUITE")
    print("="*45)
    
    for name, path in tests:
        success = run_test(name, os.path.join(root_dir, path))
        results.append((name, success))
        if not success:
            all_passed = False
            
    print("\n" + "="*45)
    print("SANDBOX TEST SUMMARY")
    print("="*45)
    for name, success in results:
        status = "PASS" if success else "FAIL"
        print(f"{name:30}: {status}")
    print("="*45)
    
    if all_passed:
        print("All 8 baseline scenarios passed!")
        sys.exit(0)
    else:
        print("FAILURE: Some scenarios did not meet the gate.")
        sys.exit(1)

if __name__ == "__main__":
    main()
