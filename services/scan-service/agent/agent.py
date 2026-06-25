#!/usr/bin/env python3
"""
OpenVulnScan Agent — Remote host vulnerability reporter.
Runs on target hosts, collects installed packages + CVEs, reports to scan-service.

Usage:
    python3 agent.py --server https://your-scan-service:8080 --api-key ovs_xxxx
"""

import argparse
import hashlib
import json
import platform
import socket
import subprocess
import sys
import time
from datetime import datetime, timezone
from typing import Dict, List, Optional
import urllib.request
import urllib.error


def get_hostname() -> str:
    return socket.gethostname()


def get_os_info() -> Dict:
    return {
        "system": platform.system(),
        "release": platform.release(),
        "version": platform.version(),
        "machine": platform.machine(),
    }


def get_installed_packages() -> List[Dict]:
    """Detect installed packages using available package managers."""
    packages = []

    # Debian/Ubuntu: dpkg
    try:
        result = subprocess.run(
            ["dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n"],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode == 0:
            for line in result.stdout.strip().split("\n"):
                parts = line.split("\t")
                if len(parts) >= 2:
                    packages.append({"name": parts[0], "version": parts[1], "manager": "dpkg"})
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass

    # RHEL/CentOS: rpm
    if not packages:
        try:
            result = subprocess.run(
                ["rpm", "-qa", "--qf", "%{NAME}\t%{VERSION}\n"],
                capture_output=True, text=True, timeout=30
            )
            if result.returncode == 0:
                for line in result.stdout.strip().split("\n"):
                    parts = line.split("\t")
                    if len(parts) >= 2:
                        packages.append({"name": parts[0], "version": parts[1], "manager": "rpm"})
        except (FileNotFoundError, subprocess.TimeoutExpired):
            pass

    # Python packages
    try:
        result = subprocess.run(
            [sys.executable, "-m", "pip", "list", "--format=json"],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode == 0:
            pip_pkgs = json.loads(result.stdout)
            for pkg in pip_pkgs:
                packages.append({
                    "name": pkg["name"],
                    "version": pkg["version"],
                    "manager": "pip"
                })
    except Exception:
        pass

    return packages


def get_open_ports() -> List[Dict]:
    """List listening ports using ss or netstat."""
    ports = []
    try:
        result = subprocess.run(
            ["ss", "-tlnp"],
            capture_output=True, text=True, timeout=10
        )
        if result.returncode == 0:
            for line in result.stdout.split("\n")[1:]:
                parts = line.split()
                if len(parts) >= 4 and parts[0] in ("LISTEN",):
                    addr_port = parts[3].rsplit(":", 1)
                    if len(addr_port) == 2:
                        try:
                            ports.append({
                                "port": int(addr_port[1]),
                                "protocol": "tcp",
                                "state": "listen"
                            })
                        except ValueError:
                            pass
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    return ports


def build_report(scan_id: Optional[str] = None) -> Dict:
    """Build the agent report payload."""
    return {
        "agent_version": "1.0.0",
        "scan_id": scan_id,
        "hostname": get_hostname(),
        "reported_at": datetime.now(timezone.utc).isoformat(),
        "os": get_os_info(),
        "packages": get_installed_packages(),
        "open_ports": get_open_ports(),
    }


def send_report(report: Dict, server: str, api_key: str) -> bool:
    """Send the report to the scan-service agent endpoint."""
    url = f"{server.rstrip('/')}/agent/report"
    payload = json.dumps(report).encode("utf-8")

    req = urllib.request.Request(
        url,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "X-API-Key": api_key,
            "User-Agent": f"OpenVulnScan-Agent/1.0 ({platform.system()})",
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            print(f"[OK] Report sent. Status: {resp.status}", file=sys.stderr)
            return True
    except urllib.error.HTTPError as e:
        print(f"[ERROR] HTTP {e.code}: {e.reason}", file=sys.stderr)
        return False
    except urllib.error.URLError as e:
        print(f"[ERROR] Connection failed: {e.reason}", file=sys.stderr)
        return False


def main():
    parser = argparse.ArgumentParser(description="OpenVulnScan Agent")
    parser.add_argument("--server", required=True, help="Scan service base URL")
    parser.add_argument("--api-key", required=True, help="API key (ovs_ prefix)")
    parser.add_argument("--scan-id", help="Optional scan ID to associate with")
    parser.add_argument("--interval", type=int, default=0,
                        help="Run continuously every N seconds (0 = run once)")
    args = parser.parse_args()

    if not args.api_key.startswith("ovs_"):
        print("[ERROR] API key must start with 'ovs_'", file=sys.stderr)
        sys.exit(1)

    while True:
        print(f"[INFO] Collecting system info from {get_hostname()}...", file=sys.stderr)
        report = build_report(scan_id=args.scan_id)

        print(f"[INFO] Packages collected: {len(report['packages'])}", file=sys.stderr)
        print(f"[INFO] Open ports: {len(report['open_ports'])}", file=sys.stderr)

        success = send_report(report, args.server, args.api_key)

        if args.interval <= 0:
            sys.exit(0 if success else 1)

        time.sleep(args.interval)


if __name__ == "__main__":
    main()
