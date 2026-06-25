#!/usr/bin/env python3
"""
02_verify_kev_data.py — Kiểm tra dữ liệu KEV qua public domain.
Route: https://c12.openledger.vn → nginx → gateway:8080 → PostgreSQL kev_entries

Usage:
  python3 02_verify_kev_data.py
  python3 02_verify_kev_data.py --cve CVE-2021-44228
  OSV_BASE_URL=http://172.20.2.48:8080 python3 02_verify_kev_data.py
"""

import sys
import argparse
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *

KNOWN_KEV_CVES = [
    "CVE-2021-44228",  # Log4Shell
    "CVE-2021-26855",  # ProxyLogon
    "CVE-2021-34527",  # PrintNightmare
    "CVE-2022-30190",  # Follina
    "CVE-2023-44487",  # HTTP/2 Rapid Reset
    "CVE-2024-3400",   # PAN-OS
]


def check_kev_count() -> int:
    status, data = api_get_status("/api/v1/kev/sync/status")
    if status == 0:
        fail(f"Cannot reach API: {data.get('_error', '')}")
        return -1
    count = data.get("count", 0)
    if count >= 1600:
        ok(f"KEV count (PostgreSQL): {count:,} entries ✓")
    elif count > 0:
        warn(f"KEV count: {count} (expected ~1622, sync may be incomplete)")
    else:
        fail("KEV count: 0 — sync failed or endpoint not exposed")
    return count


def check_kev_list():
    status, data = api_get_status("/api/v1/kev", params={"limit": 5, "page": 0})
    if status != 200:
        fail(f"GET /api/v1/kev: HTTP {status} — {data.get('_error', data.get('error', ''))[:60]}")
        return

    entries = data.get("entries", [])
    total   = data.get("total", 0)
    stats   = data.get("stats", {})

    ok(f"KEV list: total={total:,}, page={len(entries)} entries")
    if stats.get("unmitigated_in_platform", -1) >= 0:
        info(f"Unmitigated in platform: {stats['unmitigated_in_platform']}")

    for e in entries[:3]:
        cve_id    = e.get("cve_id", "?")
        vendor    = e.get("vendor_project", "?")
        ransomware = " 🔴[RANSOMWARE]" if e.get("is_known_ransomware") else ""
        print(f"      {cve_id} — {vendor}{ransomware}")


def check_known_cves():
    ids_param = ",".join(KNOWN_KEV_CVES)
    status, data = api_get_status(f"/api/v1/kev/check", params={"ids": ids_param})
    if status != 200:
        fail(f"KEV bulk check: HTTP {status}")
        return

    results = data.get("results", {})
    found   = [c for c, v in results.items() if v]
    missing = [c for c, v in results.items() if not v]

    ok(f"KEV bulk check: {len(found)}/{len(KNOWN_KEV_CVES)} known CVEs found")
    for c in found:
        print(f"      {C.GREEN}✓{C.RESET} {c}")
    for c in missing:
        print(f"      {C.RED}✗{C.RESET} {c}")


def check_kev_detail(cve_id: str):
    status, data = api_get_status(f"/api/v1/kev/{cve_id}")
    if status == 200:
        ok(f"GET /api/v1/kev/{cve_id} → 200")
        print(f"      vendor:   {data.get('vendor_project', '?')}")
        print(f"      product:  {data.get('product', '?')}")
        print(f"      added:    {data.get('date_added', '?')}")
        action = data.get("required_action", "")
        if action:
            print(f"      action:   {action[:80]}...")
    elif status == 404:
        warn(f"GET /api/v1/kev/{cve_id} → 404 (not in KEV catalog)")
    else:
        fail(f"GET /api/v1/kev/{cve_id} → HTTP {status}")


def check_kev_stats():
    status, data = api_get_status("/api/v1/kev/stats")
    if status == 200:
        ok("KEV stats:")
        for k, v in data.items():
            if not isinstance(v, (list, dict)):
                print(f"      {k}: {v}")
    else:
        warn(f"KEV stats: HTTP {status}")


def check_ransomware():
    status, data = api_get_status("/api/v1/kev/ransomware", params={"limit": 3})
    if status == 200:
        entries = data.get("entries", [])
        total   = data.get("total", 0)
        if total > 0:
            ok(f"Ransomware KEV entries: {total}")
            for e in entries:
                print(f"      {e.get('cve_id', '?')} — {e.get('ransomware_campaign_use', '?')}")
        else:
            warn("Ransomware entries: 0")
    else:
        warn(f"Ransomware endpoint: HTTP {status}")


def main():
    parser = argparse.ArgumentParser(description="Verify KEV data via public domain")
    parser.add_argument("--cve", default=None, help="Single CVE to lookup")
    args = parser.parse_args()

    head("KEV Data Verification")
    print_env_summary()
    info(f"Route: {BASE_URL} → gateway → PostgreSQL kev_entries")

    if args.cve:
        sub(f"KEV lookup: {args.cve}")
        check_kev_detail(args.cve)
        return

    sub("1. KEV Entry Count (PostgreSQL)")
    count = check_kev_count()

    sub("2. KEV List API")
    check_kev_list()

    sub("3. Known High-Profile CVE Check")
    check_known_cves()

    sub("4. KEV Detail — Log4Shell")
    check_kev_detail("CVE-2021-44228")

    sub("5. KEV Stats")
    check_kev_stats()

    sub("6. Ransomware Entries")
    check_ransomware()

    sub("Summary")
    if count > 0:
        ok(f"KEV data: {count:,} entries ✓")
    elif count == -1:
        fail(f"API unreachable — check nginx routing or use: OSV_BASE_URL=http://{SERVER_IP}:{GATEWAY_PORT}")
    else:
        fail("KEV data empty — scheduler may not have run")


if __name__ == "__main__":
    main()
