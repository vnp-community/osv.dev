#!/usr/bin/env python3
"""
OpenVulnScan Agent — Linux/macOS installed package reporter.
Usage: python3 ovs-agent.py --api-key <KEY> --server https://api.yourdomain.com

Supports: apt/dpkg (Debian/Ubuntu), rpm/yum/dnf (RHEL/CentOS), homebrew (macOS)
"""

import argparse
import json
import os
import platform
import re
import subprocess
import sys
import socket
from datetime import datetime
from typing import List, Dict
try:
    import urllib.request as urlrequest
    import urllib.error as urlerror
except ImportError:
    print("Python 3.4+ required", file=sys.stderr)
    sys.exit(1)

VERSION = "1.0.0"


def get_installed_packages() -> List[Dict]:
    """Detect package manager and return list of installed packages."""
    packages = []
    system = platform.system().lower()

    # ── Debian / Ubuntu ────────────────────────────────────────────────────
    if shutil_which("dpkg-query"):
        try:
            out = subprocess.check_output(
                ["dpkg-query", "-W", "-f=${Package}\t${Version}\t${Architecture}\n"],
                stderr=subprocess.DEVNULL, text=True
            )
            for line in out.strip().splitlines():
                parts = line.split("\t")
                if len(parts) >= 2:
                    packages.append({
                        "name": parts[0],
                        "version": parts[1],
                        "ecosystem": "debian",
                        "architecture": parts[2] if len(parts) > 2 else ""
                    })
        except subprocess.CalledProcessError:
            pass

    # ── RPM (RHEL/CentOS/Fedora/Rocky) ────────────────────────────────────
    elif shutil_which("rpm"):
        try:
            out = subprocess.check_output(
                ["rpm", "-qa", "--queryformat", "%{NAME}\t%{VERSION}-%{RELEASE}\t%{ARCH}\n"],
                stderr=subprocess.DEVNULL, text=True
            )
            for line in out.strip().splitlines():
                parts = line.split("\t")
                if len(parts) >= 2:
                    packages.append({
                        "name": parts[0],
                        "version": parts[1],
                        "ecosystem": "rpm",
                        "architecture": parts[2] if len(parts) > 2 else ""
                    })
        except subprocess.CalledProcessError:
            pass

    # ── Homebrew (macOS) ──────────────────────────────────────────────────
    elif system == "darwin" and shutil_which("brew"):
        try:
            out = subprocess.check_output(
                ["brew", "list", "--versions"],
                stderr=subprocess.DEVNULL, text=True
            )
            for line in out.strip().splitlines():
                parts = line.split()
                if len(parts) >= 2:
                    packages.append({
                        "name": parts[0],
                        "version": parts[-1],
                        "ecosystem": "homebrew",
                        "architecture": ""
                    })
        except subprocess.CalledProcessError:
            pass

    return packages


def shutil_which(cmd: str) -> bool:
    """Return True if cmd is on PATH."""
    for path in os.environ.get("PATH", "").split(os.pathsep):
        if os.path.isfile(os.path.join(path, cmd)):
            return True
    return False


def get_system_info() -> Dict:
    """Collect hostname, IP, OS, and kernel info."""
    hostname = socket.gethostname()
    try:
        ip = socket.gethostbyname(hostname)
    except socket.gaierror:
        ip = "127.0.0.1"

    uname = platform.uname()
    return {
        "hostname": hostname,
        "ip_address": ip,
        "os_info": f"{uname.system} {uname.release} ({uname.machine})",
        "kernel_version": uname.release,
    }


def submit_report(server: str, api_key: str, report: Dict) -> Dict:
    """POST report to agent-service. Returns response JSON."""
    url = f"{server.rstrip('/')}/agents/report"
    data = json.dumps(report).encode("utf-8")
    req = urlrequest.Request(url, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("X-API-Key", api_key)
    req.add_header("User-Agent", f"ovs-agent/{VERSION}")

    try:
        with urlrequest.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urlerror.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        print(f"HTTP {e.code}: {body}", file=sys.stderr)
        sys.exit(1)
    except urlerror.URLError as e:
        print(f"Connection error: {e.reason}", file=sys.stderr)
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(description="OpenVulnScan Agent")
    parser.add_argument("--api-key", required=True, help="OVS API key")
    parser.add_argument("--server", default="https://localhost:8443", help="OVS API gateway URL")
    parser.add_argument("--dry-run", action="store_true", help="Print report without submitting")
    parser.add_argument("--version", action="version", version=f"ovs-agent {VERSION}")
    args = parser.parse_args()

    print(f"[{datetime.utcnow().isoformat()}Z] OpenVulnScan Agent v{VERSION} starting...")

    sysinfo = get_system_info()
    print(f"  Host: {sysinfo['hostname']} ({sysinfo['ip_address']})")
    print(f"  OS:   {sysinfo['os_info']}")

    packages = get_installed_packages()
    print(f"  Packages collected: {len(packages)}")

    report = {
        "hostname": sysinfo["hostname"],
        "ip_address": sysinfo["ip_address"],
        "os_info": sysinfo["os_info"],
        "kernel_version": sysinfo["kernel_version"],
        "packages": packages,
    }

    if args.dry_run:
        print("\n[DRY RUN] Report payload:")
        print(json.dumps(report, indent=2)[:2000] + ("..." if len(packages) > 10 else ""))
        return

    print(f"  Submitting to {args.server} ...")
    result = submit_report(args.server, args.api_key, report)
    print(f"  ✅ Report submitted: ID={result.get('report_id', 'n/a')} | CVEs={result.get('cve_count', 0)}")


if __name__ == "__main__":
    main()
