#!/usr/bin/env python3
"""
04_verify_scheduler.py — Kiểm tra scheduler status và trigger manual sync.
Route: https://c12.openledger.vn → gateway → data-service

Usage:
  python3 04_verify_scheduler.py
  python3 04_verify_scheduler.py --trigger kev
  python3 04_verify_scheduler.py --trigger NVD --wait 30
  python3 04_verify_scheduler.py --all        # trigger tất cả sources
"""

import sys
import time
import argparse
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *

SOURCES = ["kev", "NVD", "CIRCL", "CVE.ORG", "EXPLOITDB", "JVN", "EPSS", "CAPEC", "CWE", "NVD-CPE"]


def check_fetcher_registry():
    """GET /health → danh sách fetchers đã đăng ký."""
    status, data = api_get_status("/health")
    if status != 200:
        fail(f"Cannot reach gateway: HTTP {status} — {data.get('_error', '')}")
        return

    fetchers = data.get("fetchers", 0)
    if isinstance(fetchers, list):
        ok(f"Fetcher registry: {len(fetchers)} fetchers")
        for f in fetchers:
            print(f"      {C.GREEN}✓{C.RESET} {f}")
    elif isinstance(fetchers, int) and fetchers >= 9:
        ok(f"Fetcher registry: {fetchers} fetchers ✓")
    elif isinstance(fetchers, int) and fetchers > 0:
        warn(f"Fetchers: {fetchers} (expected 9)")
    else:
        warn("Fetchers not exposed in public /health (may need internal access)")


def check_kev_sync_status() -> int:
    """GET /api/v1/kev/sync/status → số lượng KEV entries."""
    status, data = api_get_status("/api/v1/kev/sync/status")
    if status == 0:
        fail(f"Cannot reach sync/status: {data.get('_error', '')}")
        return -1
    if status == 404:
        fail("GET /api/v1/kev/sync/status → 404 (check nginx routing for /api/v1/)")
        return -1

    count = data.get("count", 0)
    last_sync = data.get("last_sync", data.get("updated_at", "?"))

    if count >= 1600:
        ok(f"KEV: {count:,} entries ✓ | last_sync={last_sync}")
    elif count > 0:
        warn(f"KEV: {count} entries (expected ~1622) | last_sync={last_sync}")
    else:
        fail("KEV: 0 entries — sync not completed")
    return count


def trigger_sync(source: str) -> bool:
    """POST /admin/sync/{source} — trigger manual sync."""
    # Admin endpoint — cần auth
    if not _auth_token:
        token = login()
        if not token:
            warn(f"Cannot login to trigger sync for {source}")
            return False

    data = api_post(f"/admin/sync/{source}", auth=True)
    if data.get("_error"):
        # Thử endpoint khác nếu admin route không exposed
        data = api_post(f"/api/v1/sync/{source}", auth=True)

    if data.get("_status", 0) in (200, 202) or "triggered" in str(data).lower():
        ok(f"Sync triggered: {source} → {data}")
        return True
    else:
        fail(f"Trigger failed: {source} → {data}")
        info("Note: /admin/sync may not be exposed via nginx (internal-only)")
        return False


def main():
    parser = argparse.ArgumentParser(description="Check scheduler status via public API")
    parser.add_argument("--trigger", default=None,
                        metavar="SOURCE",
                        help=f"Trigger sync for source. Options: {', '.join(SOURCES)}")
    parser.add_argument("--all", action="store_true", help="Trigger sync for all sources")
    parser.add_argument("--wait", type=int, default=0,
                        help="Seconds to wait after trigger before re-checking")
    args = parser.parse_args()

    head("Scheduler Status & Control")
    print_env_summary()
    info(f"Route: {BASE_URL} → gateway → data-service scheduler")

    sub("1. Gateway Health")
    check_fetcher_registry()

    sub("2. KEV Sync Status")
    count = check_kev_sync_status()

    if args.trigger or args.all:
        sources = SOURCES if args.all else [args.trigger]
        sub(f"3. Triggering Sync: {', '.join(sources)}")
        for src in sources:
            trigger_sync(src)
            time.sleep(0.5)

        if args.wait > 0:
            info(f"Waiting {args.wait}s for sync to progress...")
            time.sleep(args.wait)
            sub("4. Post-trigger Status")
            check_kev_sync_status()

    sub("Summary")
    if count >= 1600:
        ok(f"Scheduler healthy: {count:,} KEV entries ✓")
    elif count > 0:
        warn(f"KEV partial: {count} entries (sync may be running)")
        info(f"Re-run: python3 04_verify_scheduler.py --trigger kev --wait 30")
    else:
        info("Useful commands:")
        print(f"""
  # Trigger KEV sync (qua API nếu endpoint exposed):
  python3 04_verify_scheduler.py --trigger kev

  # Xem logs trực tiếp trên server:
  ssh {SSH_USER}@{SERVER_IP} 'cd {COMPOSE_DIR} && {COMPOSE_F} logs -f --tail=50 osv-server 2>&1 | grep -E "scheduler|sync|KEV|NVD"'

  # Check PostgreSQL trực tiếp:
  ssh {SSH_USER}@{SERVER_IP} 'cd {COMPOSE_DIR} && {COMPOSE_F} exec -T postgres psql -U {POSTGRES_USER} -d {POSTGRES_DB} -c "SELECT COUNT(*) FROM kev_entries;"'
""")


if __name__ == "__main__":
    main()
