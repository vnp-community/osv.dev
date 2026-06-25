#!/usr/bin/env python3
"""
run_all.py — Chạy tất cả debug scripts theo thứ tự và hiển thị báo cáo tổng hợp.

Usage:
  python3 run_all.py                     # chạy tất cả
  python3 run_all.py --quick             # chỉ chạy health + pipeline test
  OSV_SERVER=172.20.2.48 python3 run_all.py
"""

import sys
import os
import time
import subprocess
import argparse

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))


from typing import Tuple

def run_script(script: str, extra_args: list = None) -> Tuple[int, float]:
    """Run a script and return (exit_code, duration_seconds)."""
    cmd = [sys.executable, os.path.join(SCRIPT_DIR, script)] + (extra_args or [])
    env = os.environ.copy()
    start = time.time()
    result = subprocess.run(cmd, env=env)
    return result.returncode, time.time() - start


def main():
    parser = argparse.ArgumentParser(description="Run all OSV debug scripts")
    parser.add_argument("--quick", action="store_true", help="Quick mode: only health + pipeline")
    args = parser.parse_args()

    print("\n" + "="*70)
    print("  OSV Debug Suite — Full Verification Report")
    print(f"  Server: {os.getenv('OSV_SERVER', '172.20.2.48')}")
    print("="*70)

    scripts = [
        ("01_health_check.py",       "Stack Health Check",          []),
        ("02_verify_kev_data.py",    "KEV Data (PostgreSQL)",        []),
        ("03_verify_cve_data.py",    "CVE Data (MongoDB)",           []),
        ("04_verify_scheduler.py",   "Scheduler Status",             []),
        ("05_verify_stores.py",      "Data Stores (SSH+docker)",     []),
        ("06_full_pipeline_test.py", "Full Pipeline (BUG-002 test)", []),
    ]

    if args.quick:
        scripts = [s for s in scripts if s[0] in ("01_health_check.py", "06_full_pipeline_test.py")]

    results = []
    for script, name, extra in scripts:
        print(f"\n{'─'*70}")
        print(f"  ▶ Running: {name}")
        print(f"{'─'*70}")
        rc, dur = run_script(script, extra)
        results.append((script, name, rc, dur))

    print("\n" + "="*70)
    print("  SUMMARY")
    print("="*70)

    all_ok = True
    for script, name, rc, dur in results:
        status = "✓ PASS" if rc == 0 else "✗ FAIL"
        color  = "\033[92m" if rc == 0 else "\033[91m"
        reset  = "\033[0m"
        print(f"  {color}{status}{reset}  {name:<40} ({dur:.1f}s)")
        if rc != 0:
            all_ok = False

    print()
    if all_ok:
        print("  \033[92m\033[1mAll checks passed! BUG-002 fix verified.\033[0m")
    else:
        print("  \033[91m\033[1mSome checks failed. Review output above.\033[0m")
    print()


if __name__ == "__main__":
    main()
