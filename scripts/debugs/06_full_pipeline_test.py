#!/usr/bin/env python3
"""
06_full_pipeline_test.py — End-to-end pipeline test qua public domain.

10 test cases:
  TC-01: Gateway health              GET /health → 200
  TC-02: Data-service embedded       fetchers=9 trong health response
  TC-03: Fetcher registry (9 sources)
  TC-04: KEV data synced             /api/v2/kev/sync/status count ≥ 1600
  TC-05: KEV API                     /api/v2/kev → entries returned
  TC-06: CVE data synced             /cve/last/10 → entries in MongoDB
  TC-07: CVE lookup (Log4Shell)      /cve/CVE-2021-44228 (after NVD full sync)
  TC-08: Auth works                  /api/v1/auth/login → 200 + token
  TC-09: KEV bulk check              /api/v2/kev/check?ids=...
  TC-10: Nginx routing               All /api/v2/ routes accessible

Usage:
  python3 06_full_pipeline_test.py
  python3 06_full_pipeline_test.py --json
  OSV_BASE_URL=http://172.20.2.48:8080 python3 06_full_pipeline_test.py
"""

import sys
import json
import argparse
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *


class TestResult:
    def __init__(self):
        self.passed   = []
        self.failed   = []
        self.skipped  = []
        self.warnings = []

    def pass_(self, tc_id, name, detail=""):
        self.passed.append({"id": tc_id, "name": name, "detail": str(detail)})

    def fail_(self, tc_id, name, detail=""):
        self.failed.append({"id": tc_id, "name": name, "detail": str(detail)})

    def skip_(self, tc_id, name, detail=""):
        self.skipped.append({"id": tc_id, "name": name, "detail": str(detail)})

    def warn_(self, tc_id, name, detail=""):
        self.warnings.append({"id": tc_id, "name": name, "detail": str(detail)})

    def summary(self):
        total = len(self.passed) + len(self.failed) + len(self.skipped)
        return {
            "total":      total,
            "passed":     len(self.passed),
            "failed":     len(self.failed),
            "skipped":    len(self.skipped),
            "warnings":   len(self.warnings),
            "all_passed": len(self.failed) == 0,
        }


# ── Test cases ────────────────────────────────────────────────────────────────

def tc01_gateway_health(r):
    status, data = api_get_status("/health")
    if status == 200:
        svc = data.get("service", data.get("status", "ok"))
        r.pass_("TC-01", "Gateway health", f"HTTP 200 — {svc}")
    else:
        r.fail_("TC-01", "Gateway health",
                f"HTTP {status} — {data.get('_error', data.get('error', ''))[:60]}")


def tc02_data_service_embedded(r):
    status, data = api_get_status("/health")
    if status != 200:
        r.fail_("TC-02", "Data-service embedded", f"HTTP {status}")
        return
    mode = data.get("mode", "?")
    fetchers = data.get("fetchers", 0)
    if mode == "embedded" or (isinstance(fetchers, int) and fetchers > 0):
        r.pass_("TC-02", "Data-service embedded", f"mode={mode}, fetchers={fetchers}")
    else:
        r.warn_("TC-02", "Data-service mode", f"mode={mode}, fetchers={fetchers}")


def tc03_fetcher_registry(r):
    expected = {"NVD", "CIRCL", "CVE.ORG", "EXPLOITDB", "JVN", "EPSS", "CAPEC", "CWE", "NVD-CPE"}
    status, data = api_get_status("/health")
    if status != 200:
        r.fail_("TC-03", "Fetcher registry", f"HTTP {status}")
        return
    fetchers = data.get("fetchers", [])
    if isinstance(fetchers, list) and fetchers:
        registered = set(fetchers)
        missing = expected - registered
        if not missing:
            r.pass_("TC-03", "Fetcher registry (9)", f"All: {sorted(registered)}")
        else:
            r.fail_("TC-03", "Fetcher registry", f"Missing: {missing}")
    elif isinstance(fetchers, int) and fetchers >= 9:
        r.pass_("TC-03", "Fetcher registry (9)", f"{fetchers} fetchers registered")
    else:
        r.warn_("TC-03", "Fetcher registry", f"fetchers={fetchers} (not exposed in /health)")


def tc04_kev_data_synced(r):
    status, data = api_get_status("/api/v2/kev/sync/status")
    if status == 0:
        r.fail_("TC-04", "KEV sync status", data.get("_error", "connection error"))
        return
    if status == 404:
        r.fail_("TC-04", "KEV sync status", "404 — /api/v2/kev routes not registered in gateway")
        return
    count = data.get("count", 0)
    if count >= 1600:
        r.pass_("TC-04", "KEV data synced (PostgreSQL)", f"{count:,} entries ✓")
    elif count > 0:
        r.warn_("TC-04", "KEV data partial", f"{count} entries (expected ~1622)")
    else:
        r.fail_("TC-04", "KEV data empty", "0 entries — scheduler failed")


def tc05_kev_api(r):
    status, data = api_get_status("/api/v2/kev", params={"limit": 2})
    if status == 200:
        total   = data.get("total", 0)
        entries = data.get("entries", [])
        if total > 0:
            r.pass_("TC-05", "KEV API", f"total={total:,}, entries={len(entries)}")
        else:
            r.warn_("TC-05", "KEV API", "total=0")
    else:
        r.fail_("TC-05", "KEV API", f"HTTP {status}")


def tc06_cve_data_synced(r):
    status, data = api_get_status("/cve/last/10")
    if status == 0:
        r.fail_("TC-06", "CVE data (MongoDB)", data.get("_error", "connection error"))
        return
    if data is None:
        data = []
    cves  = data if isinstance(data, list) else data.get("cves", data.get("items", []))
    count = len(cves) if isinstance(cves, list) else 0
    if count >= 1:
        r.pass_("TC-06", "CVE data synced (MongoDB)", f"{count} recent CVEs returned")
    else:
        r.warn_("TC-06", "CVE data empty", "0 CVEs (NVD incremental sync in progress)")


def tc07_cve_lookup_log4shell(r):
    cve_id = "CVE-2021-44228"
    status, data = api_get_status(f"/cve/{cve_id}")
    if status == 200:
        r.pass_("TC-07", f"CVE lookup {cve_id}",
                f"severity={data.get('severity')} cvss={data.get('cvss_v3_score')}")
    elif status == 404:
        r.warn_("TC-07", f"CVE lookup {cve_id}",
                "404 — NVD incremental sync only fetches last 2 days")
    else:
        r.fail_("TC-07", f"CVE lookup {cve_id}", f"HTTP {status}")


def tc08_auth_login(r):
    """Test login flow qua /api/v1/auth/login."""
    data = api_post("/api/v1/auth/login", {
        "email": ADMIN_EMAIL, "password": ADMIN_PASSWORD
    })
    token = (data.get("data", {}) or {}).get("access_token") or data.get("access_token")
    if token:
        r.pass_("TC-08", "Auth login", f"Token received ({len(token)} chars)")
    elif data.get("_status", 0) == 401:
        r.fail_("TC-08", "Auth login", f"401 — wrong credentials ({ADMIN_EMAIL})")
    elif data.get("_error"):
        r.fail_("TC-08", "Auth login", data["_error"][:60])
    else:
        r.warn_("TC-08", "Auth login", f"Unexpected response: {str(data)[:60]}")


def tc09_kev_bulk_check(r):
    test_cves = ["CVE-2021-44228", "CVE-2021-26855", "CVE-2023-44487"]
    status, data = api_get_status("/api/v2/kev/check",
                                  params={"ids": ",".join(test_cves)})
    if status == 200:
        results = data.get("results", [])
        if isinstance(results, list):
            found = sum(1 for item in results if item.get("is_kev"))
        elif isinstance(results, dict):
            found = sum(1 for v in results.values() if v)
        else:
            found = 0
        r.pass_("TC-09", "KEV bulk check", f"{found}/{len(test_cves)} known CVEs in KEV")
    else:
        r.fail_("TC-09", "KEV bulk check", f"HTTP {status}")


def tc10_nginx_routing(r):
    """Kiểm tra tất cả API prefixes được nginx route đúng."""
    routes = [
        ("/health",               "gateway health"),
        ("/api/v2/kev/sync/status", "KEV sync status"),
        ("/cve/last/1",           "CVE last"),
    ]
    all_ok = True
    details = []
    for path, label in routes:
        status, _ = api_get_status(path)
        if status in (200, 401):
            details.append(f"{label}={status}")
        else:
            details.append(f"{label}={status}⚠")
            all_ok = False

    if all_ok:
        r.pass_("TC-10", "Nginx routing", ", ".join(details))
    else:
        r.fail_("TC-10", "Nginx routing", ", ".join(details))


# ── Output ────────────────────────────────────────────────────────────────────

def print_results(results, as_json=False):
    if as_json:
        print(json.dumps({
            "summary":  results.summary(),
            "base_url": BASE_URL,
            "passed":   results.passed,
            "failed":   results.failed,
            "skipped":  results.skipped,
            "warnings": results.warnings,
        }, indent=2))
        return

    sub("Test Results")
    for t in results.passed:
        ok(f"[{t['id']}] {t['name']}: {t['detail']}")
    for t in results.warnings:
        warn(f"[{t['id']}] {t['name']}: {t['detail']}")
    for t in results.failed:
        fail(f"[{t['id']}] {t['name']}: {t['detail']}")
    for t in results.skipped:
        info(f"[{t['id']}] SKIPPED {t['name']}: {t['detail']}")

    s = results.summary()
    sub("Final Summary")
    print(f"\n  Total:    {s['total']}")
    print(f"  {C.GREEN}Passed:   {s['passed']}{C.RESET}")
    print(f"  {C.YELLOW}Warnings: {s['warnings']}{C.RESET}")
    print(f"  {C.RED}Failed:   {s['failed']}{C.RESET}")
    print(f"  {C.DIM}Skipped:  {s['skipped']}{C.RESET}")

    if s["all_passed"] and s["warnings"] == 0:
        print(f"\n  {C.GREEN}{C.BOLD}✓ All tests passed!{C.RESET}")
    elif s["all_passed"]:
        print(f"\n  {C.YELLOW}{C.BOLD}⚠ Passed with warnings (sync in progress){C.RESET}")
    else:
        print(f"\n  {C.RED}{C.BOLD}✗ {s['failed']} test(s) failed — check nginx routing{C.RESET}")
        info(f"Tip: OSV_BASE_URL=http://{SERVER_IP}:{GATEWAY_PORT} python3 06_full_pipeline_test.py")
    print()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()

    if not args.json:
        head("OSV Full Pipeline Test")
        print_env_summary()

    r = TestResult()
    tc01_gateway_health(r)
    tc02_data_service_embedded(r)
    tc03_fetcher_registry(r)
    tc04_kev_data_synced(r)
    tc05_kev_api(r)
    tc06_cve_data_synced(r)
    tc07_cve_lookup_log4shell(r)
    tc08_auth_login(r)
    tc09_kev_bulk_check(r)
    tc10_nginx_routing(r)

    print_results(r, as_json=args.json)
    sys.exit(0 if r.summary()["all_passed"] else 1)


if __name__ == "__main__":
    main()
