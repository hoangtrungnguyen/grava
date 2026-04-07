import subprocess
import os
import sys

def run_test(name, path):
    print(f"Running {name}...")
    result = subprocess.run(["python3", path], capture_output=True, text=True)
    if result.returncode == 0:
        print(f"✅ {name} PASSED")
        return True
    else:
        print(f"❌ {name} FAILED")
        print("Stdout:", result.stdout)
        print("Stderr:", result.stderr)
        return False

def main():
    root_dir = os.path.dirname(os.path.dirname(__file__))
    tests = [
        ("Rapid Claims", "sandbox/test_rapid_claims.py"),
        ("Crash Resume", "sandbox/test_crash_resume.py"),
        ("Epic 3 Lifecycle", "sandbox/test_epic3_lifecycle.py"),
    ]
    
    all_passed = True
    results = []
    
    for name, path in tests:
        success = run_test(name, os.path.join(root_dir, path))
        results.append((name, success))
        if not success:
            all_passed = False
            
    print("\n" + "="*30)
    print("SANDBOX TEST SUMMARY")
    print("="*30)
    for name, success in results:
        status = "PASS" if success else "FAIL"
        print(f"{name:20}: {status}")
    print("="*30)
    
    if all_passed:
        print("All Epic 3 scenarios passed!")
        sys.exit(0)
    else:
        print("Some scenarios failed.")
        sys.exit(1)

if __name__ == "__main__":
    main()
