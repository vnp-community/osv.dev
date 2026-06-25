"""
test_crash_report.py — Kiểm tra các endpoints bị lỗi theo bug report crash.md

Bug report từ: ui/specs/bugs/ui-crash/crash.md
Server: https://c12.openledger.vn

Danh sách lỗi được verify:
  - /api/v1/sla/overview              → 404 Not Found
  - /api/v2/cves/search               → 500 Internal Server Error
  - /api/v2/vendors                   → 404 Not Found
  - /api/v2/cwe                       → 404 Not Found
  - /api/v1/scans                     → 404 Not Found
  - /api/v1/scans?status=running      → 500 Internal Server Error
  - /api/v1/dashboard                 → check status
  - /api/v1/dashboard/sla             → check status

Script này sẽ:
  1. Thực hiện HTTP request đến từng endpoint
  2. Ghi nhận HTTP status thực tế
  3. Đánh giá: PASS (status đúng spec) / REGRESSED (vẫn còn lỗi) / FIXED (đã sửa)

Chạy:
  python test_crash_report.py

Cấu hình: Chỉnh API_BASE_URL_V1 / V2 trong .env trỏ về staging server.
"""

from __future__ import annotations

import sys
import time
from pathlib import Path
from typing import List, Optional, Tuple

sys.path.insert(0, str(Path(__file__).parent))

from base_client import APIClient, _Color
from config import Config


# ── Bug entry definition ──────────────────────────────────────────────────────

class BugEntry:
    def __init__(
        self,
        bug_id: str,
        description: str,
        method: str,
        path: str,
        v2: bool,
        params: Optional[dict],
        body: Optional[dict],
        expected_ok_status: int,
        reported_status: int,
    ):
        self.bug_id = bug_id
        self.description = description
        self.method = method
        self.path = path
        self.v2 = v2
        self.params = params
        self.body = body
        self.expected_ok_status = expected_ok_status
        self.reported_status = reported_status  # status lúc bug được report

        # Kết quả sau test
        self.actual_status: Optional[int] = None
        self.elapsed_ms: float = 0.0
        self.response_snippet: str = ""


# ── Danh sách bugs từ crash.md ────────────────────────────────────────────────

BUGS: List[BugEntry] = [
    BugEntry(
        bug_id="BUG-01",
        description="Dashboard SLA: GET /api/v1/sla/overview → 404",
        method="GET", path="/sla/overview", v2=False, params=None, body=None,
        expected_ok_status=200, reported_status=404,
    ),
    BugEntry(
        bug_id="BUG-02",
        description="CVE Search: POST /api/v2/cves/search → 500",
        method="POST", path="/cves/search", v2=True, params=None,
        body={"page": 1, "page_size": 5},
        expected_ok_status=200, reported_status=500,
    ),
    BugEntry(
        bug_id="BUG-03",
        description="Vendors: GET /api/v2/vendors → 404",
        method="GET", path="/vendors", v2=True, params={"limit": 5}, body=None,
        expected_ok_status=200, reported_status=404,
    ),
    BugEntry(
        bug_id="BUG-04",
        description="CWE List: GET /api/v2/cwe → 404",
        method="GET", path="/cwe", v2=True, params={"page": 1, "page_size": 10}, body=None,
        expected_ok_status=200, reported_status=404,
    ),
    BugEntry(
        bug_id="BUG-05",
        description="Scans List: GET /api/v1/scans → 404",
        method="GET", path="/scans", v2=False, params={"page": 1, "page_size": 10}, body=None,
        expected_ok_status=200, reported_status=404,
    ),
    BugEntry(
        bug_id="BUG-06",
        description="Scans Running: GET /api/v1/scans?status=running → 500",
        method="GET", path="/scans", v2=False, params={"status": "running"}, body=None,
        expected_ok_status=200, reported_status=500,
    ),
    BugEntry(
        bug_id="BUG-07",
        description="Dashboard: GET /api/v1/dashboard → check status",
        method="GET", path="/dashboard", v2=False, params=None, body=None,
        expected_ok_status=200, reported_status=0,  # 0 = chưa xác định
    ),
    BugEntry(
        bug_id="BUG-08",
        description="Dashboard SLA Full: GET /api/v1/dashboard/sla → 404",
        method="GET", path="/dashboard/sla", v2=False, params=None, body=None,
        expected_ok_status=200, reported_status=404,
    ),
    BugEntry(
        bug_id="BUG-09",
        description="Products: GET /api/v1/products → check status",
        method="GET", path="/products", v2=False, params={"page": 1}, body=None,
        expected_ok_status=200, reported_status=0,
    ),
    BugEntry(
        bug_id="BUG-10",
        description="Assets: GET /api/v1/assets → check status",
        method="GET", path="/assets", v2=False, params={"page": 1, "pageSize": 10}, body=None,
        expected_ok_status=200, reported_status=0,
    ),
    BugEntry(
        bug_id="BUG-11",
        description="KEV Catalog: GET /api/v2/kev → check status",
        method="GET", path="/kev", v2=True, params={"page": 1, "page_size": 10}, body=None,
        expected_ok_status=200, reported_status=0,
    ),
    BugEntry(
        bug_id="BUG-12",
        description="EPSS Top: GET /api/v2/epss/top → check status",
        method="GET", path="/epss/top", v2=True, params={"limit": 10}, body=None,
        expected_ok_status=200, reported_status=0,
    ),
]


def _status_color(status: int, expected: int) -> str:
    if status == expected:
        return f"{_Color.GREEN}{status}{_Color.RESET}"
    elif status >= 500:
        return f"{_Color.RED}{status}{_Color.RESET}"
    elif status == 404:
        return f"{_Color.YELLOW}{status}{_Color.RESET}"
    else:
        return f"{_Color.CYAN}{status}{_Color.RESET}"


def run_bug_checks(client: APIClient) -> Tuple[List[BugEntry], int, int, int]:
    """Chạy tất cả bug checks. Trả về (bugs, fixed, still_broken, new_issues)."""
    fixed = 0
    still_broken = 0
    new_issues = 0

    for bug in BUGS:
        t0 = time.time()
        try:
            if bug.method == "GET":
                resp = client.get(bug.path, v2=bug.v2, params=bug.params)
            elif bug.method == "POST":
                resp = client.post(bug.path, v2=bug.v2, body=bug.body)
            else:
                continue

            bug.actual_status = resp.status_code
            bug.elapsed_ms = (time.time() - t0) * 1000

            # Lấy snippet response để debug
            try:
                snippet = resp.text[:150].replace("\n", " ")
                bug.response_snippet = snippet
            except Exception:
                bug.response_snippet = "(binary or empty)"

            # Phân loại kết quả
            if resp.status_code == bug.expected_ok_status:
                if bug.reported_status in (404, 500):
                    fixed += 1  # Đã được sửa
            elif resp.status_code == bug.reported_status and bug.reported_status in (404, 500):
                still_broken += 1  # Vẫn còn lỗi
            elif resp.status_code >= 500 and bug.reported_status != 500:
                new_issues += 1  # Lỗi mới

        except Exception as e:
            bug.actual_status = -1
            bug.response_snippet = f"Exception: {e}"
            still_broken += 1

    return BUGS, fixed, still_broken, new_issues


def print_report(bugs: List[BugEntry], fixed: int, still_broken: int, new_issues: int) -> None:
    """In báo cáo chi tiết theo từng endpoint."""

    print(f"\n{_Color.BOLD}{'='*70}")
    print("  CRASH REPORT — Kết quả kiểm tra endpoints bị lỗi")
    print(f"  Server: {Config.API_BASE_URL_V1.replace('/api/v1', '')}")
    print(f"{'='*70}{_Color.RESET}")

    for bug in bugs:
        actual = bug.actual_status
        expected = bug.expected_ok_status
        reported = bug.reported_status

        # Xác định trạng thái
        if actual == expected:
            if reported in (404, 500):
                tag = f"{_Color.GREEN}[FIXED]     {_Color.RESET}"
            else:
                tag = f"{_Color.GREEN}[OK]        {_Color.RESET}"
        elif actual == reported and reported in (404, 500):
            tag = f"{_Color.RED}[BROKEN]    {_Color.RESET}"
        elif actual is not None and actual >= 500:
            tag = f"{_Color.RED}[ERROR]     {_Color.RESET}"
        elif actual == 401:
            tag = f"{_Color.YELLOW}[AUTH]      {_Color.RESET}"
        elif actual == 403:
            tag = f"{_Color.YELLOW}[FORBIDDEN] {_Color.RESET}"
        elif actual == -1:
            tag = f"{_Color.RED}[CONN_ERR]  {_Color.RESET}"
        else:
            tag = f"{_Color.CYAN}[CHECK]     {_Color.RESET}"

        # Hiện thị trạng thái
        actual_display = _status_color(actual, expected) if actual and actual > 0 else f"{_Color.RED}N/A{_Color.RESET}"

        print(f"\n  {tag} {_Color.BOLD}{bug.bug_id}{_Color.RESET} {bug.description}")

        method_path = f"{bug.method} {'v2' if bug.v2 else 'v1'}{bug.path}"
        if bug.params:
            qs = "&".join(f"{k}={v}" for k, v in bug.params.items())
            method_path += f"?{qs}"
        print(f"           {method_path}")
        print(f"           Reported: {_Color.YELLOW}{reported if reported else '?'}{_Color.RESET}  "
              f"Actual: {actual_display}  "
              f"Expected: {_Color.GREEN}{expected}{_Color.RESET}  "
              f"({bug.elapsed_ms:.0f}ms)")

        if bug.response_snippet and actual != expected:
            print(f"           Response: {bug.response_snippet[:120]}")

    # ── Summary ──────────────────────────────────────────────────────────────
    total = len(bugs)
    print(f"\n{_Color.BOLD}{'='*70}{_Color.RESET}")
    print(f"  {_Color.BOLD}SUMMARY{_Color.RESET}")
    print(f"{'='*70}")
    print(f"  Tổng endpoints kiểm tra : {total}")
    print(f"  {_Color.GREEN}Đã sửa (FIXED)          : {fixed}{_Color.RESET}")
    print(f"  {_Color.RED}Vẫn còn lỗi (BROKEN)    : {still_broken}{_Color.RESET}")
    if new_issues:
        print(f"  {_Color.RED}Lỗi mới (NEW ISSUE)     : {new_issues}{_Color.RESET}")
    ok_count = sum(
        1 for b in bugs
        if b.actual_status == b.expected_ok_status and b.reported_status not in (404, 500)
    )
    print(f"  {_Color.CYAN}Hoạt động đúng (OK)      : {ok_count}{_Color.RESET}")

    unresolved = [b for b in bugs
                  if b.actual_status != b.expected_ok_status and b.reported_status in (404, 500)]
    if unresolved:
        print(f"\n{_Color.RED}  CÁC ENDPOINT VẪN BỊ LỖI:{_Color.RESET}")
        for b in unresolved:
            print(f"    - {b.bug_id}: {b.description}  (HTTP {b.actual_status})")
    print(f"{'='*70}\n")


def main() -> int:
    print(f"\n{_Color.BOLD}{'='*70}")
    print("  OSV PLATFORM — CRASH REPORT VERIFICATION")
    print(f"  Checking known broken endpoints from crash.md")
    print(f"{'='*70}{_Color.RESET}")
    Config.dump()

    client = APIClient()
    if not client.login():
        print(f"\n{_Color.RED}✗ Login failed — cannot run checks.{_Color.RESET}")
        return 1

    bugs, fixed, still_broken, new_issues = run_bug_checks(client)
    print_report(bugs, fixed, still_broken, new_issues)

    # Exit code: 0 nếu không còn bug nào từ danh sách reported
    return 1 if still_broken > 0 else 0


if __name__ == "__main__":
    sys.exit(main())
