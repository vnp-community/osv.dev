#!/usr/bin/env python3
"""
01_health_check.py — Kiểm tra health của toàn bộ stack OSV qua domain public.

Usage:
  python3 01_health_check.py
  OSV_BASE_URL=https://c12.openledger.vn python3 01_health_check.py
  OSV_BASE_URL=http://172.20.2.48:8080   python3 01_health_check.py  # fallback direct
"""

import sys
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *


def check(name: str, path: str, expect_key: str = "status",
          auth: bool = False, warn_on_401: bool = False):
    """Call an endpoint và hiển thị kết quả."""
    status, data = api_get_status(path, auth=auth)
    if data is None:
        data = {}
    err = data.get("_error", "") if isinstance(data, dict) else ""

    if status == 200:
        if isinstance(data, list):
            val = f"list with {len(data)} items"
        elif isinstance(data, dict):
            val = data.get(expect_key, list(data.values())[0] if data else "ok")
        else:
            val = str(data)
        ok(f"{name}: {status} — {val}")
        return data
    elif status == 401 and warn_on_401:
        warn(f"{name}: 401 Unauthorized (auth required — expected without token)")
        return data
    elif status == 0:
        fail(f"{name}: {err or 'connection error'}")
    else:
        fail(f"{name}: HTTP {status} — {data.get('error', data.get('_raw', ''))[:80]}")
    return None


def main():
    head("OSV Stack Health Check")
    print_env_summary()

    sub("1. Public Endpoint")
    gw = check("GET /health",         "/health")

    sub("2. API v1 — KEV Data-service (public)")
    check("GET /api/v1/kev/sync/status", "/api/v1/kev/sync/status", expect_key="count")

    sub("3. API v1 — CVE Routes (gateway, requires auth)")
    check("GET /api/v1/cve/last/5",  "/api/v1/cve/last/5",  expect_key="", auth=True)

    sub("4. CVE Data-service Routes (direct via nginx)")
    check("GET /cve/last/5",  "/cve/last/5",  expect_key="")
    check("GET /info",         "/info",         expect_key="")

    sub("5. Fetcher Registry")
    status, data = api_get_status("/health")
    if status == 200:
        fetchers = data.get("fetchers", 0)
        if isinstance(fetchers, list):
            ok(f"Fetchers ({len(fetchers)}): {', '.join(fetchers)}")
        elif isinstance(fetchers, int) and fetchers >= 9:
            ok(f"Fetcher registry: {fetchers} fetchers ✓")
        elif fetchers:
            warn(f"Fetchers: {fetchers} (expected 9 or list)")
        else:
            warn("Fetchers info not exposed via public /health")

    sub("Summary")
    if gw:
        ok(f"Gateway reachable via {BASE_URL}")
    else:
        fail(f"Cannot reach {BASE_URL}")
        info("Possible causes:")
        info("  1. Port 8080 bound to 127.0.0.1 (not accessible from nginx proxy)")
        info("  2. Nginx not routing /health → osv-server")
        info(f"  3. Try direct: OSV_BASE_URL=http://{SERVER_IP}:{GATEWAY_PORT} python3 01_health_check.py")
        sys.exit(1)


if __name__ == "__main__":
    main()
