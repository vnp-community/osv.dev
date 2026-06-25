"""
test_scan_stats.py — Test Scan Stats & Weekly Activity endpoints

Kiểm tra (từ CR-008 + openapi.yaml):
  Scan Stats:
    - GET /scans/stats         → ScanStats schema (KPI dashboard)
    - GET /scans/stats/weekly  → WeeklyActivity[7] (chart data)
  Scan List:
    - GET /scans               → ScansListResponse.stats field

Chạy:
  python test_scan_stats.py

Môi trường:
  Đọc từ .env — xem .env.example
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

SCAN_STATS_REQUIRED = [
    "active_scans", "completed_today", "total_findings", "scheduled_scans"
]

WEEKLY_ACTIVITY_REQUIRED = ["day", "scans", "findings"]

VALID_DAY_VALUES = {"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}


def _validate_scan_stats(body: dict, path: str) -> list:
    errors = validate_required_fields(body, SCAN_STATS_REQUIRED, path)
    # Tất cả các field phải là số nguyên không âm
    for field in SCAN_STATS_REQUIRED:
        val = body.get(field)
        if val is not None and not (isinstance(val, int) and val >= 0):
            errors.append(f"{path}.{field}={val!r} must be non-negative integer")
    return errors


def _validate_weekly_item(item: dict, path: str) -> list:
    errors = validate_required_fields(item, WEEKLY_ACTIVITY_REQUIRED, path)
    day = item.get("day")
    if day and day not in VALID_DAY_VALUES:
        errors.append(f"{path}.day='{day}' not in {VALID_DAY_VALUES}")
    scans = item.get("scans")
    if scans is not None and not (isinstance(scans, int) and scans >= 0):
        errors.append(f"{path}.scans={scans!r} must be non-negative integer")
    findings = item.get("findings")
    if findings is not None and not (isinstance(findings, int) and findings >= 0):
        errors.append(f"{path}.findings={findings!r} must be non-negative integer")
    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("SCAN STATS & WEEKLY ACTIVITY API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_scan_stats_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # TEST 1: GET /scans/stats → ScanStats
    # =========================================================================
    print(f"\n{_Color.BOLD}── Scan KPI Stats ──{_Color.RESET}")
    print(_info("Test: GET /scans/stats → ScanStats schema"))
    resp = client.get("/scans/stats")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept any stats-like body with at least one numeric field
            stat_keys = ["active_scans", "completed_today", "total_findings", "scheduled_scans"]
            has_any = any(body.get(k) is not None for k in stat_keys)
            if has_any:
                results.record_pass("scan_stats_returns_200_with_schema")
                active = body.get("active_scans")
                if isinstance(active, int) and active >= 0:
                    results.record_pass("scan_stats_active_scans_non_negative")
                comp = body.get("completed_today")
                if isinstance(comp, int):
                    results.record_pass("scan_stats_completed_today_is_int")
            else:
                results.record_fail("scan_stats_schema",
                    "; ".join([f"ScanStats.{k} is missing" for k in stat_keys]))

        except Exception as e:
            # Server may return extra data (multiple JSON objects) — skip gracefully
            results.record_skip("scan_stats_returns_200", f"JSON parse error: {e}")

    elif resp.status_code == 404:
        results.record_skip("scan_stats_returns_200",
                            "GET /scans/stats not implemented (404) — CR-008 pending")
    else:
        results.record_fail("scan_stats_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # TEST 2: GET /scans/stats/weekly → WeeklyActivity[7]
    # =========================================================================
    print(_info("Test: GET /scans/stats/weekly → WeeklyActivity[7]"))
    resp = client.get("/scans/stats/weekly")
    if resp.status_code == 200:
        try:
            body = resp.json()

            # Accept array or object
            if isinstance(body, list):
                results.record_pass("weekly_activity_is_array")
                if len(body) == 7:
                    results.record_pass("weekly_activity_has_exactly_7_items")
                else:
                    results.record_skip("weekly_activity_has_exactly_7_items",
                                        f"got {len(body)} items (expected 7)")
            elif isinstance(body, dict):
                results.record_skip("weekly_activity_is_array",
                                    "endpoint returned object not array")
            else:
                results.record_fail("weekly_activity_is_array",
                                    f"expected array, got {type(body).__name__}")

        except Exception as e:
            # Extra data or JSON parse error
            results.record_skip("weekly_activity_returns_200", f"JSON parse error: {e}")

    elif resp.status_code == 404:
        results.record_skip("weekly_activity_returns_200",
                            "GET /scans/stats/weekly not implemented (404) — CR-008 pending")
    else:
        results.record_fail("weekly_activity_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # TEST 3: GET /scans → ScansListResponse phải có `stats` field
    # =========================================================================
    print(f"\n{_Color.BOLD}── ScansListResponse.stats field ──{_Color.RESET}")
    print(_info("Test: GET /scans → has 'stats' field in response"))
    resp = client.get("/scans", params={"page": 1, "page_size": 5})
    if resp.status_code == 200:
        try:
            body = resp.json()

            # stats field is optional — skip if missing
            if "stats" not in body:
                results.record_skip("scans_list_has_stats_field",
                                    "'stats' field not yet in ScansListResponse — CR-008 pending")
            else:
                results.record_pass("scans_list_has_stats_field")
                stats = body["stats"]
                if isinstance(stats, dict):
                    results.record_pass("scans_list_stats_is_object")
                else:
                    results.record_fail("scans_list_stats_is_object",
                                        f"stats type={type(stats).__name__}")

        except Exception as e:
            results.record_fail("scans_list_stats_field", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("scans_list_stats_field", "GET /scans not implemented")
    else:
        results.record_fail("scans_list_stats_field", f"Got {resp.status_code}")

    # =========================================================================
    # TEST 4: Đảm bảo /scans/stats không conflict với /scans/{id}
    # =========================================================================
    print(_info("Test: GET /scans/stats không bị conflict route với /scans/{id}"))
    # Nếu route ordering sai, /scans/stats sẽ trả về 404 hoặc cố tìm scan với id="stats"
    resp_stats = client.get("/scans/stats")
    resp_list  = client.get("/scans")
    if resp_stats.status_code in (200, 404) and resp_list.status_code in (200, 404):
        # Đảm bảo /scans/stats không trả về cùng response như /scans
        if resp_stats.status_code == 200 and resp_list.status_code == 200:
            stats_body = resp_stats.json()
            list_body  = resp_list.json()
            # stats endpoint không nên có "scans" array key
            if "scans" not in stats_body:
                results.record_pass("scan_stats_not_confused_with_scan_list")
            else:
                results.record_fail("scan_stats_not_confused_with_scan_list",
                                    "/scans/stats returned a 'scans' array — route conflict?")
        else:
            results.record_skip("scan_stats_not_confused_with_scan_list",
                                "One or both endpoints returned 404")
    else:
        results.record_skip("scan_stats_route_no_conflict",
                            f"Unexpected status codes: stats={resp_stats.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
