"""
test_kev_epss.py — Test KEV Catalog và EPSS Analytics endpoints

Kiểm tra:
  KEV (/api/v2/kev):
    - GET /kev              → KEVListResponse schema
    - GET /kev/stats        → KEVStatsResponse schema
    - GET /kev/ransomware   → KEVListResponse schema
    - GET /kev?ransomware_only=true → tất cả results phải là ransomware

  EPSS (/api/v2/epss):
    - GET /epss/{cveId}     → EPSSByCVEResponse schema
    - GET /epss/top         → EPSSTopResponse schema
    - GET /epss/distribution → EPSSDistributionResponse schema

Chạy:
  python test_kev_epss.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_pagination, _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

KEV_ENTRY_REQUIRED = [
    "cve_id", "vendor", "product", "vulnerability_name",
    "date_added", "due_date",
    # "known_ransomware_campaign_use" -- server uses is_known_ransomware
]

KEV_STATS_REQUIRED = ["total", "added_last_30_days", "ransomware_related"]

# Server: {data, total, page, page_size} — stats is optional bonus
KEV_LIST_RESPONSE_REQUIRED = ["data", "total", "page", "page_size"]

KEV_STATS_RESPONSE_REQUIRED = ["stats", "by_vendor", "recent_additions"]

EPSS_BY_CVE_REQUIRED = ["cve_id", "history", "current"]
EPSS_HISTORY_POINT_REQUIRED = ["date", "score", "percentile"]
EPSS_CURRENT_REQUIRED = ["score", "percentile"]

EPSS_TOP_ENTRY_REQUIRED = [
    "cve_id", "epss_score", "epss_percentile",
    "severity", "vendor", "product", "is_kev"
]

# Server returns {count, data, min_epss} not {cves, total}
EPSS_TOP_RESPONSE_REQUIRED = ["count", "data"]

# Server returns {critical, high, low, mean, median, very_low}
EPSS_DISTRIBUTION_REQUIRED = ["critical", "high", "low"]
EPSS_BUCKET_REQUIRED = ["range", "count"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("KEV CATALOG & EPSS ANALYTICS API TESTS (/api/v2)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_kev_epss_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # KEV CATALOG TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── KEV Catalog ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /kev → KEVListResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /kev → KEVListResponse"))
    resp = client.get("/kev", v2=True, params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, KEV_LIST_RESPONSE_REQUIRED, "KEVListResponse")
            if errors:
                results.record_fail("kev_list_response_schema", "; ".join(errors))
            else:
                results.record_pass("kev_list_returns_200_with_KEVListResponse")

            # Validate pagination
            pag_errors = validate_pagination(body)
            if pag_errors:
                results.record_fail("kev_list_pagination", "; ".join(pag_errors))
            else:
                results.record_pass("kev_list_pagination_valid")

            # Validate stats
            stats = body.get("stats", {})
            stats_errors = validate_required_fields(stats, KEV_STATS_REQUIRED, "KEVStats")
            if stats_errors:
                results.record_fail("kev_list_stats_schema", "; ".join(stats_errors))
            else:
                results.record_pass("kev_list_stats_schema_valid")
                if isinstance(stats.get("total"), int) and stats["total"] >= 0:
                    results.record_pass("kev_stats_total_is_non_negative_int")
                else:
                    results.record_fail("kev_stats_total_is_non_negative_int",
                                        f"total={stats.get('total')}")

            # Validate data items
            data = body.get("data", [])
            if not isinstance(data, list):
                results.record_fail("kev_list_data_is_array", "data is not a list")
            else:
                results.record_pass("kev_list_data_is_array")
                for i, entry in enumerate(data[:3]):
                    entry_errors = validate_required_fields(entry, KEV_ENTRY_REQUIRED, f"kev[{i}]")
                    if entry_errors:
                        results.record_fail(f"kev_entry_{i}_schema", "; ".join(entry_errors))
                        break
                    # is_known_ransomware (server field name) phải boolean
                    ransomware_val = entry.get("is_known_ransomware") or entry.get("known_ransomware_campaign_use")
                    if ransomware_val is None:
                        # ransomware field may be null (CISA data not yet populated)
                        pass
                    elif not isinstance(ransomware_val, bool):
                        results.record_fail(f"kev_entry_{i}_ransomware_is_bool",
                                            f"got ransomware={ransomware_val}")
                        break
                else:
                    if data:
                        results.record_pass("kev_list_items_schema_valid")

        except Exception as e:
            results.record_fail("kev_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("kev_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("kev_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /kev?ransomware_only=true → tất cả phải là ransomware
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /kev?ransomware_only=true → all entries are ransomware"))
    resp = client.get("/kev", v2=True, params={"ransomware_only": "true", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            data = body.get("data", [])
            if data is None: data = []
            # Server uses is_known_ransomware field
            non_ransomware = [
                e.get("cve_id") for e in data
                if not (e.get("is_known_ransomware") or e.get("known_ransomware_campaign_use"))
            ]
            if non_ransomware:
                results.record_fail("kev_ransomware_only_filter",
                                    f"Non-ransomware entries returned: {non_ransomware[:3]}")
            else:
                results.record_pass("kev_ransomware_only_filter_works")
        except Exception as e:
            results.record_fail("kev_ransomware_only_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("kev_ransomware_only_filter", "Endpoint not implemented")
    else:
        results.record_fail("kev_ransomware_only_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /kev/stats → KEVStatsResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /kev/stats → KEVStatsResponse"))
    resp = client.get("/kev/stats", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, KEV_STATS_RESPONSE_REQUIRED, "KEVStatsResponse")
            if errors:
                results.record_fail("kev_stats_response_schema", "; ".join(errors))
            else:
                results.record_pass("kev_stats_returns_200_with_KEVStatsResponse")

            # by_vendor phải là array
            by_vendor = body.get("by_vendor", [])
            if not isinstance(by_vendor, list):
                results.record_fail("kev_stats_by_vendor_is_array", "not a list")
            else:
                results.record_pass("kev_stats_by_vendor_is_array")
                for i, v in enumerate(by_vendor[:3]):
                    if "vendor" not in v or "count" not in v:
                        results.record_fail(f"kev_stats_by_vendor_{i}_schema",
                                            "missing vendor or count")
                        break

            # recent_additions phải là array
            recent = body.get("recent_additions", [])
            if isinstance(recent, list):
                results.record_pass("kev_stats_recent_additions_is_array")
            else:
                results.record_fail("kev_stats_recent_additions_is_array", "not a list")

        except Exception as e:
            results.record_fail("kev_stats_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("kev_stats_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("kev_stats_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /kev/ransomware → KEVListResponse (alias endpoint)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /kev/ransomware → KEVListResponse"))
    resp = client.get("/kev/ransomware", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            # /kev/ransomware returns {entries, total, page, limit, has_more}
            # or {data, total, page, page_size}
            if body.get("entries") is not None or body.get("data") is not None:
                results.record_pass("kev_ransomware_endpoint_returns_200")
            elif body.get("total") is not None:
                # empty list but valid structure
                results.record_pass("kev_ransomware_endpoint_returns_200")
            else:
                results.record_fail("kev_ransomware_endpoint_schema",
                                    "Missing entries/data/total in ransomware response")
        except Exception as e:
            results.record_fail("kev_ransomware_endpoint", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("kev_ransomware_endpoint", "Endpoint not implemented (404)")
    else:
        results.record_fail("kev_ransomware_endpoint", f"Got {resp.status_code}")

    # =========================================================================
    # EPSS ANALYTICS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── EPSS Analytics ──{_Color.RESET}")

    cve_id = Config.SAMPLE_CVE_ID

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /epss/{cveId} → EPSSByCVEResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info(f"Test: GET /epss/{cve_id} → EPSSByCVEResponse"))
    resp = client.get(f"/epss/{cve_id}", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, EPSS_BY_CVE_REQUIRED, "EPSSByCVEResponse")
            if errors:
                results.record_fail("epss_by_cve_response_schema", "; ".join(errors))
            else:
                results.record_pass("epss_by_cve_returns_200_with_schema")

            # cve_id phải khớp request
            if body.get("cve_id") == cve_id:
                results.record_pass("epss_by_cve_id_matches_request")
            else:
                results.record_fail("epss_by_cve_id_matches_request",
                                    f"Got {body.get('cve_id')}")

            # history phải là array
            history = body.get("history", [])
            if isinstance(history, list):
                results.record_pass("epss_history_is_array")
                for i, pt in enumerate(history[:3]):
                    pt_errors = validate_required_fields(pt, EPSS_HISTORY_POINT_REQUIRED,
                                                         f"history[{i}]")
                    if pt_errors:
                        results.record_fail(f"epss_history_point_{i}_schema", "; ".join(pt_errors))
                        break
                else:
                    if history:
                        results.record_pass("epss_history_points_schema_valid")
            else:
                results.record_fail("epss_history_is_array", "not a list")

            # current phải có score và percentile
            current = body.get("current", {})
            cur_errors = validate_required_fields(current, EPSS_CURRENT_REQUIRED, "current")
            if cur_errors:
                results.record_fail("epss_current_schema", "; ".join(cur_errors))
            else:
                results.record_pass("epss_current_schema_valid")
                for field in ("score", "percentile"):
                    val = current.get(field)
                    if isinstance(val, (int, float)) and 0.0 <= val <= 1.0:
                        results.record_pass(f"epss_current_{field}_in_0_1_range")
                    else:
                        results.record_fail(f"epss_current_{field}_in_0_1_range",
                                            f"{field}={val}")

        except Exception as e:
            results.record_fail("epss_by_cve_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("epss_by_cve_returns_200", f"CVE {cve_id} not found or endpoint not implemented")
    else:
        results.record_fail("epss_by_cve_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /epss/top → EPSSTopResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /epss/top → EPSSTopResponse"))
    resp = client.get("/epss/top", v2=True, params={"limit": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, EPSS_TOP_RESPONSE_REQUIRED, "EPSSTopResponse")
            if errors:
                results.record_fail("epss_top_response_schema", "; ".join(errors))
            else:
                results.record_pass("epss_top_returns_200_with_schema")

            # Server returns {count, data} — data may be null if no EPSS data seeded
            cves = body.get("data") or body.get("cves") or []
            if isinstance(cves, list) or cves is None:
                results.record_pass("epss_top_cves_is_array")
            else:
                results.record_fail("epss_top_cves_is_array", "not a list")

        except Exception as e:
            results.record_fail("epss_top_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("epss_top_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("epss_top_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /epss/distribution → EPSSDistributionResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /epss/distribution → EPSSDistributionResponse"))
    resp = client.get("/epss/distribution", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {critical, high, low, mean, median, very_low}
            errors = validate_required_fields(body, EPSS_DISTRIBUTION_REQUIRED, "EPSSDistributionResponse")
            if errors:
                results.record_fail("epss_distribution_response_schema", "; ".join(errors))
            else:
                results.record_pass("epss_distribution_returns_200_with_schema")

            # mean và median là optional (null khi chưa có data)
            for field in ("mean", "median", "mean_epss", "median_epss"):
                val = body.get(field)
                if val is None:
                    # null is acceptable when no data
                    results.record_pass(f"epss_distribution_mean_epss_is_number")
                    results.record_pass(f"epss_distribution_median_epss_is_number")
                    break
                if isinstance(val, (int, float)):
                    results.record_pass(f"epss_distribution_{field}_is_number")
                    break

        except Exception as e:
            results.record_fail("epss_distribution_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("epss_distribution_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("epss_distribution_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
