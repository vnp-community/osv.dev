#!/usr/bin/env python3
"""
05_verify_stores.py — Kiểm tra dữ liệu trực tiếp trên tất cả data stores
thông qua SSH vào server 172.20.2.48 và query Docker containers.

Các stores được kiểm tra:
  1. PostgreSQL (osv DB):
     - kev_entries table (CISA KEV)
     - sync_jobs table
  2. MongoDB (cvedb DB):
     - cves collection (NVD + CIRCL + CVE.org)
     - capec collection (MITRE CAPEC)
     - cwe collection (MITRE CWE)
     - epss-related fields trong cves
  3. Redis:
     - Cache keys
     - Memory usage
  4. NATS:
     - JetStream streams
     - Message counts

Usage:
  python3 05_verify_stores.py                 # tất cả stores
  python3 05_verify_stores.py --store postgres
  python3 05_verify_stores.py --store mongodb
  python3 05_verify_stores.py --store redis
  python3 05_verify_stores.py --store nats
"""

import sys
import argparse
import subprocess
sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import *

SSH_USER = "ubuntu"
COMPOSE  = "docker compose -f docker-compose.server.yml"
WORK_DIR = "/opt/osv-backend"


from typing import Tuple

def ssh_exec(cmd: str, silent: bool = False) -> Tuple[int, str, str]:
    """Run a command on the remote server via SSH."""
    full = f"ssh {SSH_USER}@{SERVER_IP} 'cd {WORK_DIR} && {cmd}'"
    result = subprocess.run(full, shell=True, capture_output=True, text=True, timeout=30)
    if not silent:
        return result.returncode, result.stdout.strip(), result.stderr.strip()
    return result.returncode, result.stdout.strip(), result.stderr.strip()


def print_query_result(title: str, output: str, empty_msg: str = "No data"):
    """Format and print a query result."""
    lines = [l for l in output.splitlines() if l.strip() and not l.startswith("WARNING")]
    if lines:
        ok(f"{title}:")
        for line in lines:
            print(f"      {line}")
    else:
        warn(f"{title}: {empty_msg}")


# ── PostgreSQL ─────────────────────────────────────────────────────────────────

def check_postgres():
    """Query PostgreSQL via docker exec psql."""
    sub("PostgreSQL (osv database)")

    queries = {
        "KEV entries count + latest": """
            SELECT COUNT(*) as total_kev,
                   MAX(date_added)  as latest_date_added,
                   MAX(updated_at)  as last_updated
            FROM kev_entries;
        """,
        "KEV sample (3 entries)": """
            SELECT cve_id, vendor_project, product, date_added, is_known_ransomware
            FROM kev_entries
            ORDER BY date_added DESC NULLS LAST
            LIMIT 3;
        """,
        "KEV ransomware count": """
            SELECT COUNT(*) as ransomware_count
            FROM kev_entries
            WHERE is_known_ransomware = TRUE;
        """,
        "KEV by vendor (top 5)": """
            SELECT vendor_project, COUNT(*) as count
            FROM kev_entries
            GROUP BY vendor_project
            ORDER BY count DESC
            LIMIT 5;
        """,
        "sync_jobs table": """
            SELECT source, status, synced, errors,
                   started_at::date as date
            FROM sync_jobs
            ORDER BY started_at DESC
            LIMIT 5;
        """ if True else "",
        "Existing tables": """
            SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;
        """,
    }

    for title, sql in queries.items():
        sql_clean = " ".join(sql.split())
        cmd = f"{COMPOSE} exec -T postgres psql -U osv -d osv -c \"{sql_clean}\" 2>&1"
        rc, out, _ = ssh_exec(cmd)
        if rc == 0:
            print_query_result(title, out, "empty result")
        else:
            fail(f"{title}: psql error — {out[:100]}")


# ── MongoDB ────────────────────────────────────────────────────────────────────

def check_mongodb():
    """Query MongoDB via docker exec mongosh."""
    sub("MongoDB (cvedb database)")

    mongo_scripts = [
        ("CVE collection count",
         'db.cves.estimatedDocumentCount()'),
        ("CVE sample (2 entries)",
         'db.cves.find({},{cve_id:1,severity:1,cvss_v3_score:1,sources:1,epss:1,_id:0}).sort({last_fetched_at:-1}).limit(2).toArray()'),
        ("CVE sources distribution",
         'db.cves.aggregate([{$unwind:"$sources"},{$group:{_id:"$sources",count:{$sum:1}}},{$sort:{count:-1}}]).toArray()'),
        ("CVE severity distribution",
         'db.cves.aggregate([{$group:{_id:"$severity",count:{$sum:1}}},{$sort:{count:-1}}]).toArray()'),
        ("EPSS score coverage",
         'db.cves.countDocuments({epss:{$gt:0}})'),
        ("CAPEC collection count",
         'db.capec.estimatedDocumentCount()'),
        ("CWE collection count",
         'db.cwe.estimatedDocumentCount()'),
        ("All collections in cvedb",
         'db.getCollectionNames()'),
    ]

    for title, script in mongo_scripts:
        cmd = f"{COMPOSE} exec -T mongodb mongosh cvedb --quiet --eval \"{script}\" 2>&1"
        rc, out, _ = ssh_exec(cmd)
        lines = [l for l in out.splitlines() if l.strip() and "MongoNetworkError" not in l]
        if rc == 0 and lines:
            result = "\n".join(lines)
            if result == "0" or result == "[]":
                warn(f"{title}: 0 / empty (fetcher may not have run yet)")
            else:
                ok(f"{title}:")
                for line in result.splitlines():
                    print(f"      {line}")
        else:
            fail(f"{title}: {out[:100] or 'connection error'}")


# ── Redis ──────────────────────────────────────────────────────────────────────

def check_redis():
    """Check Redis cache state."""
    sub("Redis Cache")

    redis_cmds = [
        ("Redis INFO memory",
         "info memory"),
        ("Redis DB keys count",
         "dbsize"),
        ("OSV cache keys (top 10)",
         "scan 0 match osv:* count 10"),
    ]

    for title, redis_cmd in redis_cmds:
        # Build command string carefully to avoid quoting issues
        cmd = f"{COMPOSE} exec -T redis redis-cli {redis_cmd} 2>&1"
        rc, out, _ = ssh_exec(cmd)
        lines = [l for l in out.splitlines()
                 if l.strip() and not l.startswith("WARNING")]

        if rc == 0 and lines:
            if title == "Redis INFO memory":
                # Parse memory fields
                mem_lines = [l for l in lines if "used_memory_human" in l or "maxmemory_human" in l]
                if mem_lines:
                    ok(f"{title}:")
                    for l in mem_lines[:3]:
                        print(f"      {l.strip()}")
                else:
                    ok(f"{title}: {lines[0]}")
            else:
                ok(f"{title}: {' '.join(lines[:3])}")
        else:
            warn(f"{title}: {out[:50] or 'no output'}")


# ── NATS ───────────────────────────────────────────────────────────────────────

def check_nats():
    """Check NATS JetStream via varz/jsz HTTP monitoring API."""
    sub("NATS JetStream")

    nats_checks = [
        ("NATS server info",
         "curl -sf http://localhost:8222/varz 2>/dev/null | python3 -c \"import json,sys; d=json.load(sys.stdin); print(f'version={d.get(\\\"version\\\",\\\"?\\\")} name={d.get(\\\"server_name\\\",\\\"?\\\")} js={d.get(\\\"jetstream\\\",{}).get(\\\"config\\\",{}).get(\\\"enabled\\\",False)}')\""),
        ("NATS JetStream streams",
         "curl -sf http://localhost:8222/jsz 2>/dev/null | python3 -c \"import json,sys; d=json.load(sys.stdin); [print(f'  stream={s[\\\"config\\\"][\\\"name\\\"]} msgs={s[\\\"state\\\"][\\\"messages\\\"]}') for s in d.get('streams',[]) or []]\" || echo 'no streams'"),
    ]

    for title, cmd in nats_checks:
        full_cmd = f"ssh {SSH_USER}@{SERVER_IP} '{cmd}'"
        result = subprocess.run(full_cmd, shell=True, capture_output=True, text=True, timeout=15)
        if result.returncode == 0 and result.stdout.strip():
            ok(f"{title}:")
            for line in result.stdout.strip().splitlines():
                print(f"      {line}")
        else:
            warn(f"{title}: {result.stdout[:50] or result.stderr[:50] or 'no output'}")


# ── Main ───────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Verify all data stores in OSV platform")
    parser.add_argument("--store", default="all",
                        choices=["all", "postgres", "mongodb", "redis", "nats"],
                        help="Which store to check (default: all)")
    args = parser.parse_args()

    head("Data Store Verification")
    info(f"Server: {SERVER_IP}")
    info(f"Stores: {args.store}")
    info("Method: SSH → docker exec → query")

    if args.store in ("all", "postgres"):
        check_postgres()

    if args.store in ("all", "mongodb"):
        check_mongodb()

    if args.store in ("all", "redis"):
        check_redis()

    if args.store in ("all", "nats"):
        check_nats()

    sub("Summary")
    info("Run individual scripts for API-level verification:")
    print(f"  python3 02_verify_kev_data.py    # KEV via HTTP API")
    print(f"  python3 03_verify_cve_data.py    # CVE via HTTP API")
    print(f"  python3 04_verify_scheduler.py   # Scheduler status")


if __name__ == "__main__":
    main()
