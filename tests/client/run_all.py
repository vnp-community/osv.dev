"""
run_all.py — Chạy toàn bộ test suite và in báo cáo tổng hợp.

Cách dùng:
  python run_all.py                     # chạy tất cả
  python run_all.py auth dashboard      # chỉ chạy các module chỉ định
  python run_all.py --list              # liệt kê các module có sẵn
  python run_all.py --verbose           # bật verbose logging
  python run_all.py --no-stop-on-fail   # tiếp tục dù có module fail

Môi trường:
  Đọc cấu hình từ .env trong cùng thư mục (xem .env.example)
"""

from __future__ import annotations

import importlib
import os
import sys
import time
from pathlib import Path
from typing import Dict, List, Optional

# ── Thiết lập sys.path ───────────────────────────────────────────────────────
_here = Path(__file__).parent
sys.path.insert(0, str(_here))

from base_client import TestResults, _Color
from config import Config


# ── Danh sách module test theo thứ tự ưu tiên ───────────────────────────────

ALL_MODULES: Dict[str, str] = {
    "auth":                  "test_auth",
    "dashboard":             "test_dashboard",
    "cve":                   "test_cve_intelligence",
    "kev_epss":              "test_kev_epss",
    "taxonomy":              "test_taxonomy",
    "findings_scans":        "test_findings_scans",
    "assets_products":       "test_assets_products",
    "admin_notifications":   "test_admin_notifications",
    "ai_reports":            "test_ai_reports",
    # ── New modules: openapi.yaml updates (CR-008 to CR-014) ──
    "scan_stats":            "test_scan_stats",
    "webhook_deliveries":    "test_webhook_deliveries",
    "admin_extended":        "test_admin_extended",
    "public_stats":          "test_public_stats",
    "ai_triage_queue":       "test_ai_triage_queue",
    # ── Full endpoint coverage: all api_endpoints.md gaps ──
    "missing_endpoints":     "test_missing_endpoints",
}

MODULE_DESCRIPTIONS: Dict[str, str] = {
    "auth":                "Auth (login, refresh, MFA, OAuth, logout)",
    "dashboard":           "Dashboard (KPIs, risk trend, SLA)",
    "cve":                 "CVE Intelligence (search, semantic, detail, export)",
    "kev_epss":            "KEV Catalog & EPSS Analytics",
    "taxonomy":            "CWE/CAPEC Taxonomy & Vendor Browse",
    "findings_scans":      "Findings, Scans, SLA Config",
    "assets_products":     "Assets & Products",
    "admin_notifications": "Admin, Profile & Notifications",
    "ai_reports":          "AI Center, Reports & Integrations",
    # New modules (openapi.yaml updates)
    "scan_stats":          "Scan Stats & Weekly Activity (CR-008)",
    "webhook_deliveries":  "Webhook Deliveries, Retry & Hourly Stats (CR-009)",
    "admin_extended":      "Admin RBAC Matrix, User Schema, Settings, API Keys (CR-011,012)",
    "public_stats":        "Public Stats Endpoint — no auth required (CR-013)",
    "ai_triage_queue":     "AI Triage Queue — Full Schema + Human Decision (CR-014)",
    # Full coverage (all api_endpoints.md gaps)
    "missing_endpoints":   "Full Coverage — all remaining api_endpoints.md gaps (mutations, bulk, OAuth, etc.)",
}


def print_banner() -> None:
    print(f"\n{_Color.BOLD}{'='*65}")
    print("  OSV PLATFORM API — FULL TEST SUITE")
    print("  Kiểm tra dữ liệu API có đúng theo OpenAPI spec không")
    print(f"{'='*65}{_Color.RESET}")
    Config.dump()


def list_modules() -> None:
    print(f"\n{_Color.BOLD}Available test modules:{_Color.RESET}")
    for key, desc in MODULE_DESCRIPTIONS.items():
        print(f"  {key:<25} {desc}")
    print()


def run_module(module_name: str) -> Optional[TestResults]:
    """Import và chạy một test module. Trả về TestResults hoặc None nếu lỗi."""
    try:
        mod = importlib.import_module(module_name)
        if not hasattr(mod, "run_tests"):
            print(f"{_Color.RED}✗ Module '{module_name}' không có hàm run_tests(){_Color.RESET}")
            return None
        return mod.run_tests()
    except ModuleNotFoundError as e:
        print(f"{_Color.RED}✗ Module '{module_name}' không tìm thấy: {e}{_Color.RESET}")
        return None
    except Exception as e:
        print(f"{_Color.RED}✗ Module '{module_name}' bị exception: {e}{_Color.RESET}")
        import traceback
        traceback.print_exc()
        return None


def aggregate_results(module_results: List[tuple]) -> None:
    """In báo cáo tổng hợp từ tất cả các module."""
    total_passed = 0
    total_failed = 0
    total_skipped = 0
    all_failures: List[tuple] = []

    for module_key, result in module_results:
        if result is None:
            total_failed += 1
            all_failures.append((module_key, "Module load error"))
            continue
        total_passed += len(result.passed)
        total_failed += len(result.failed)
        total_skipped += len(result.skipped)
        for name, reason in result.failed:
            all_failures.append((f"{module_key}::{name}", reason))

    total = total_passed + total_failed + total_skipped

    print(f"\n{_Color.BOLD}{'='*65}")
    print("  TỔNG HỢP KẾT QUẢ TOÀN BỘ TEST SUITE")
    print(f"{'='*65}{_Color.RESET}")
    print(f"  Tổng số test    : {total}")
    print(f"  {_Color.GREEN}Passed          : {total_passed}{_Color.RESET}")
    print(f"  {_Color.RED}Failed          : {total_failed}{_Color.RESET}")
    print(f"  {_Color.YELLOW}Skipped         : {total_skipped}{_Color.RESET}")

    if total > 0:
        pass_rate = (total_passed / total) * 100
        print(f"  Pass rate       : {pass_rate:.1f}%")

    print(f"{'='*65}{_Color.RESET}")

    if all_failures:
        print(f"\n{_Color.BOLD}{_Color.RED}DANH SÁCH FAILURES:{_Color.RESET}")
        for name, reason in all_failures:
            print(f"  {_Color.RED}✗{_Color.RESET} {name}")
            print(f"      → {reason}")
    print()


def main() -> int:
    args = sys.argv[1:]

    # Xử lý flags
    verbose = "--verbose" in args
    stop_on_fail = "--no-stop-on-fail" not in args
    if "--verbose" in args:
        args.remove("--verbose")
    if "--no-stop-on-fail" in args:
        args.remove("--no-stop-on-fail")

    if "--list" in args:
        list_modules()
        return 0

    # Set verbose trong env
    if verbose:
        os.environ["VERBOSE"] = "true"

    # Xác định module cần chạy
    if args:
        # Người dùng chỉ định cụ thể
        modules_to_run = []
        for key in args:
            if key in ALL_MODULES:
                modules_to_run.append((key, ALL_MODULES[key]))
            else:
                print(f"{_Color.RED}Unknown module: '{key}'. Use --list to see available modules.{_Color.RESET}")
                return 1
    else:
        modules_to_run = list(ALL_MODULES.items())

    print_banner()

    total_start = time.time()
    module_results: List[tuple] = []
    had_failure = False

    for module_key, module_name in modules_to_run:
        desc = MODULE_DESCRIPTIONS.get(module_key, module_key)
        print(f"\n{_Color.BOLD}{_Color.MAGENTA}▶ Running: {desc}{_Color.RESET}")
        print(f"{_Color.MAGENTA}{'─'*65}{_Color.RESET}")

        result = run_module(module_name)
        module_results.append((module_key, result))

        if result and result.failed:
            had_failure = True
            if stop_on_fail:
                print(f"\n{_Color.YELLOW}⚠ Dừng lại do có failures (dùng --no-stop-on-fail để tiếp tục){_Color.RESET}")
                break

    elapsed = time.time() - total_start
    print(f"\n{_Color.CYAN}Tổng thời gian chạy: {elapsed:.2f}s{_Color.RESET}")

    aggregate_results(module_results)

    return 1 if had_failure else 0


if __name__ == "__main__":
    sys.exit(main())
