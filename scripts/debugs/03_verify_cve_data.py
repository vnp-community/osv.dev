#!/usr/bin/env python3
"""
03_verify_cve_data.py — Kiểm tra dữ liệu CVE từ NVD/CIRCL qua public domain.
Route: https://c12.openledger.vn → nginx → gateway:8080 → MongoDB cvedb.cves

Usage:
  python3 03_verify_cve_data.py
  python3 03_verify_cve_data.py --cve CVE-2021-44228
  python3 03_verify_cve_data.py --cpe "cpe:2.3:a:apache:log4j:*"
  OSV_BASE_URL=http://172.20.2.48:8080 python3 03_verify_cve_data.py
"""

import sys
import argparse
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *

TEST_CVES = [
    "CVE-2021-44228",  # Log4Shell
    "CVE-2022-22965",  # SpringShell
    "CVE-2023-44487",  # HTTP/2 Rapid Reset
    "CVE-2024-21413",  # MS Outlook RCE
]


def check_mongo_info():
    """GET /info → thống kê collections trong MongoDB."""
    status, data = api_get_status("/info")
    if status == 200:
        ok("MongoDB collection info (/info):")
        for k, v in data.items():
            if isinstance(v, int):
                print(f"      {k}: {v:,}")
            elif not isinstance(v, (dict, list)):
                print(f"      {k}: {v}")
    elif status == 404:
        info("/info not exposed via nginx (check nginx conf)")
    else:
        warn(f"GET /info: HTTP {status}")


def check_recent_cves():
    """GET /cve/last/10 → 10 CVEs mới nhất trong MongoDB."""
    status, data = api_get_status("/cve/last/10")
    if status != 200:
        fail(f"GET /cve/last/10: HTTP {status} — {data.get('_error', '')[:60]}")
        return 0

    cves  = data if isinstance(data, list) else data.get("cves", data.get("items", []))
    count = len(cves) if isinstance(cves, list) else 0

    if count > 0:
        ok(f"Last 10 CVEs: {count} entries from MongoDB")
        for cve in cves[:3]:
            cve_id   = cve.get("cve_id", cve.get("id", "?"))
            severity = cve.get("severity", "?")
            score    = cve.get("cvss_v3_score", 0)
            sources  = cve.get("sources", [])
            epss     = cve.get("epss", 0)
            print(f"      {cve_id} | sev={severity} | cvss={score} | epss={epss:.3f} | src={sources}")
    else:
        warn("Last 10 CVEs: 0 results (MongoDB may be empty — NVD sync running)")
    return count


def check_cve_timeframe(frame: str):
    """GET /cve/recent/{today|week|month}."""
    status, data = api_get_status(f"/cve/recent/{frame}")
    cves  = data if isinstance(data, list) else data.get("cves", data.get("items", []))
    count = len(cves) if isinstance(cves, list) else 0

    if status == 200 and count > 0:
        ok(f"/cve/recent/{frame}: {count} CVEs")
    elif status == 200:
        warn(f"/cve/recent/{frame}: 0 results")
    else:
        warn(f"/cve/recent/{frame}: HTTP {status}")


def check_cve_lookup(cve_id: str) -> bool:
    """GET /cve/{cve_id}."""
    status, data = api_get_status(f"/cve/{cve_id}")
    if status == 200:
        ok(f"GET /cve/{cve_id} → 200")
        print(f"      severity: {data.get('severity', '?')}")
        print(f"      cvss_v3:  {data.get('cvss_v3_score', '?')}")
        print(f"      epss:     {data.get('epss', '?')}")
        print(f"      sources:  {data.get('sources', [])}")
        return True
    elif status == 404:
        warn(f"GET /cve/{cve_id} → 404 (not synced yet)")
    else:
        fail(f"GET /cve/{cve_id} → HTTP {status}")
    return False


def check_cpe_search(cpe: str):
    """GET /cve/search?cpe=..."""
    status, data = api_get_status("/cve/search", params={
        "cpe": cpe, "limit": 5, "mode": "lax"
    })
    cves  = data if isinstance(data, list) else data.get("cves", data.get("items", []))
    count = len(cves) if isinstance(cves, list) else 0

    if status == 200 and count > 0:
        ok(f"CPE search: {count} CVEs for '{cpe[:40]}'")
        for cve in cves[:2]:
            print(f"      {cve.get('cve_id', '?')} | sev={cve.get('severity', '?')}")
    elif status == 200:
        warn(f"CPE search: 0 results (NVD-CPE not synced yet)")
    else:
        warn(f"CPE search: HTTP {status}")


def check_epss_top():
    """GET /api/v1/epss/top → CVEs với EPSS score cao."""
    status, data = api_get_status("/api/v1/epss/top",
                                  params={"limit": 5, "min_epss": 0.5})
    if status == 200:
        cves = data.get("cves", [])
        if cves:
            ok(f"EPSS top (score ≥ 0.5): {len(cves)} CVEs")
            for cve in cves[:3]:
                score = cve.get("epss_score", cve.get("epss", 0))
                print(f"      {cve.get('cve_id', '?')} | epss={score:.3f}")
        else:
            warn("EPSS top: empty (EPSS sync not complete yet)")
    else:
        warn(f"EPSS top: HTTP {status}")


def main():
    parser = argparse.ArgumentParser(description="Verify CVE data via public domain")
    parser.add_argument("--cve", default=None, help="Single CVE ID to lookup")
    parser.add_argument("--cpe", default=None, help="CPE string to search")
    args = parser.parse_args()

    head("CVE Data Verification (MongoDB)")
    print_env_summary()
    info(f"Route: {BASE_URL} → gateway → MongoDB {MONGO_DB}.cves")

    if args.cve:
        sub(f"CVE Lookup: {args.cve}")
        check_cve_lookup(args.cve)
        return

    if args.cpe:
        sub(f"CPE Search: {args.cpe}")
        check_cpe_search(args.cpe)
        return

    sub("1. MongoDB Collection Info")
    check_mongo_info()

    sub("2. Last 10 CVEs")
    cve_count = check_recent_cves()

    sub("3. Recent CVEs by Timeframe")
    for frame in ["today", "week", "month"]:
        check_cve_timeframe(frame)

    sub("4. Known CVE Lookups")
    found = sum(1 for c in TEST_CVES if check_cve_lookup(c))

    sub("5. CPE Search (Apache Log4j)")
    check_cpe_search("cpe:2.3:a:apache:log4j:*:*:*:*:*:*:*:*")

    sub("6. EPSS Top CVEs")
    check_epss_top()

    sub("Summary")
    if found > 0:
        ok(f"CVE data: {found}/{len(TEST_CVES)} test CVEs found in MongoDB")
    elif cve_count > 0:
        warn(f"CVEs in DB: {cve_count} (incremental sync — test CVEs may be older)")
    else:
        warn("No CVEs found — NVD incremental sync only fetches last 2 days")
        info("Full sync happens on daily schedule (~10-30 min)")


if __name__ == "__main__":
    main()
