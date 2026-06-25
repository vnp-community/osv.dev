#!/usr/bin/env python3
"""
03_verify_seed_data.py — Fetch data from the backend and compare with the
locally generated seed data to detect backend bugs.

Verification strategy
---------------------
For each domain, the script:
  1. Reads the original seed data (./data/<domain>/*.json)
  2. Reads the ID map (./data/output/id_map.json) to resolve server IDs
  3. Fetches the corresponding resources via GET API
  4. Compares key fields — flags mismatches, missing items, extra fields
  5. Writes a detailed reconciliation report to ./data/output/verify_report.json

Exit codes:
  0 — all checks passed
  1 — some checks failed (possible backend bugs)
  2 — could not connect / auth failed

Usage:
    python 03_verify_seed_data.py [--env .env] [--data ./data] [--domain users,findings,...]
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Any

# ---------------------------------------------------------------------------
# Local imports
# ---------------------------------------------------------------------------
sys.path.insert(0, str(Path(__file__).parent))
from seed_config import SeedConfig  # noqa: E402
from seed_client import SeedClient  # noqa: E402

logger = logging.getLogger("seed.verify")


# ---------------------------------------------------------------------------
# Data structures
# ---------------------------------------------------------------------------


@dataclass
class Mismatch:
    """A single field-level discrepancy between expected and actual values."""
    domain: str
    resource_id: str
    field: str
    expected: Any
    actual: Any
    note: str = ""


@dataclass
class VerifyResult:
    domain: str
    checked: int = 0
    passed: int = 0
    failed: int = 0
    missing_on_server: int = 0
    skipped: int = 0
    mismatches: list[Mismatch] = field(default_factory=list)



@dataclass
class Report:
    results: list[VerifyResult] = field(default_factory=list)

    @property
    def total_mismatches(self) -> int:
        return sum(len(r.mismatches) for r in self.results)

    @property
    def total_missing(self) -> int:
        return sum(r.missing_on_server for r in self.results)

    def summary(self) -> str:
        total = sum(r.checked for r in self.results)
        passed = sum(r.passed for r in self.results)
        failed = sum(r.failed for r in self.results)
        return (
            f"checked={total}, passed={passed}, failed={failed}, "
            f"missing_on_server={self.total_missing}, "
            f"field_mismatches={self.total_mismatches}"
        )


# ---------------------------------------------------------------------------
# ID Map loader
# ---------------------------------------------------------------------------


class IdMap:
    def __init__(self, path: Path) -> None:
        self._map: dict[str, str] = {}
        if path.exists():
            self._map = json.loads(path.read_text(encoding="utf-8"))
        else:
            logger.warning("ID map not found at %s — using local IDs as-is", path)

    def get(self, local_id: str) -> str:
        return self._map.get(local_id, local_id)

    def has(self, local_id: str) -> bool:
        return local_id in self._map


# ---------------------------------------------------------------------------
# Helper: load JSON seed file
# ---------------------------------------------------------------------------


def load_json(path: Path) -> list[dict]:
    if not path.exists():
        logger.warning("Seed file not found: %s — skipping verification", path)
        return []
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, dict):
        data = list(data.values())[0] if len(data) == 1 else [data]
    return data  # type: ignore[return-value]


# ---------------------------------------------------------------------------
# Field comparison helpers
# ---------------------------------------------------------------------------

# Fields to skip when comparing (server-managed, timestamps, internal)
_SKIP_FIELDS = {
    "id", "_id", "created_at", "updated_at", "last_reviewed",
    "last_status_update", "last_used_at", "hash_code",
    "numerical_severity", "prod_numeric_grade", "finding_count",
    "sla_expiration_date", "days_until_sla", "percent_complete",
    "key", "api_key", "key_hash", "password_enc", "api_key_enc",
    "hashed_password", "mfa_totp_secret", "failed_login_attempts",
    "refresh_token_hash",
}

# Tolerant string fields — compare lowercase/strip
_TOLERATE_CASE = {
    "severity", "business_criticality", "platform", "lifecycle",
    "origin", "engagement_type", "status", "auth_provider",
    "role", "scope",
}


def _normalize(val: Any, field_name: str) -> Any:
    if isinstance(val, str):
        v = val.strip()
        if field_name in _TOLERATE_CASE:
            return v.lower()
        return v
    if isinstance(val, list):
        return sorted(str(x) for x in val)
    return val


def compare_field(
    domain: str,
    resource_id: str,
    field_name: str,
    expected: Any,
    actual: Any,
    mismatches: list[Mismatch],
) -> bool:
    """Return True if values match; record a Mismatch otherwise."""
    if field_name in _SKIP_FIELDS:
        return True
    # Treat None and "" as equivalent for optional fields
    exp = expected if expected is not None else ""
    act = actual if actual is not None else ""

    exp_norm = _normalize(exp, field_name)
    act_norm = _normalize(act, field_name)

    if exp_norm == act_norm:
        return True

    mismatches.append(
        Mismatch(
            domain=domain,
            resource_id=resource_id,
            field=field_name,
            expected=expected,
            actual=actual,
        )
    )
    return False


def compare_item(
    domain: str,
    resource_id: str,
    expected: dict,
    actual: dict,
    fields: list[str],
    mismatches: list[Mismatch],
) -> int:
    """Compare *fields* between expected and actual dicts. Returns number of failures."""
    failures = 0
    for f in fields:
        if f in _SKIP_FIELDS:
            continue
        exp_val = expected.get(f)
        act_val = actual.get(f)
        if not compare_field(domain, resource_id, f, exp_val, act_val, mismatches):
            failures += 1
            logger.warning(
                "  MISMATCH [%s] %s.%s: expected=%r, got=%r",
                domain, resource_id[:12], f, exp_val, act_val,
            )
    return failures


# ---------------------------------------------------------------------------
# Domain verifiers
# ---------------------------------------------------------------------------


def verify_users(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="users")
    report.results.append(vr)

    users = load_json(data_dir / "identity" / "users.json")
    if not users:
        return

    check_fields = ["email", "username", "role", "is_active", "is_verified"]
    logger.info("=== Verifying users (%d) ===", len(users))

    for user in users:
        local_id = user["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v1/admin/users/{server_id}")
        if not resp.ok:
            # Fallback: search by email
            resp2 = client.get("/api/v1/admin/users", params={"q": user["email"], "page_size": 5})
            found = None
            if resp2.ok:
                body = resp2.json()
                items = body if isinstance(body, list) else body.get("users", body.get("data", []))
                for u in items:
                    if u.get("email") == user["email"]:
                        found = u
                        break
            if not found:
                logger.error("  ✗ user %s not found on server (HTTP %d)", user["email"], resp.status_code)
                vr.missing_on_server += 1
                vr.failed += 1
                vr.mismatches.append(
                    Mismatch(domain="users", resource_id=local_id, field="<resource>",
                             expected="exists", actual="404")
                )
                continue
            actual = found
        else:
            body = resp.json()
            actual = body.get("data", body) if isinstance(body, dict) else body

        failures = compare_item("users", server_id, user, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ user %s — all fields match", user["email"])
        else:
            vr.failed += 1


def verify_product_types(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="product_types")
    report.results.append(vr)

    pts = load_json(data_dir / "products" / "product_types.json")
    if not pts:
        return

    check_fields = ["name", "description", "critical_product", "key_product"]
    logger.info("=== Verifying product types (%d) ===", len(pts))

    for pt in pts:
        local_id = pt["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v2/product-types/{server_id}")
        if not resp.ok:
            logger.error("  ✗ product_type '%s' not found (HTTP %d)", pt["name"], resp.status_code)
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="product_types", resource_id=server_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body
        failures = compare_item("product_types", server_id, pt, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ product_type '%s'", pt["name"])
        else:
            vr.failed += 1


def verify_products(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="products")
    report.results.append(vr)

    products = load_json(data_dir / "products" / "products.json")
    if not products:
        return

    check_fields = [
        "name", "description", "business_criticality", "platform",
        "lifecycle", "external_audience", "internet_accessible",
    ]
    logger.info("=== Verifying products (%d) ===", len(products))

    for prod in products:
        local_id = prod["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v2/products/{server_id}")
        if not resp.ok:
            logger.error("  ✗ product '%s' not found (HTTP %d)", prod["name"], resp.status_code)
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="products", resource_id=server_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body
        failures = compare_item("products", server_id, prod, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ product '%s'", prod["name"])
        else:
            vr.failed += 1


def verify_engagements(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="engagements")
    report.results.append(vr)

    engagements = load_json(data_dir / "products" / "engagements.json")
    if not engagements:
        return

    check_fields = ["name", "engagement_type", "status", "version"]
    logger.info("=== Verifying engagements (%d) ===", len(engagements))

    for eng in engagements:
        local_id = eng["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v2/engagements/{server_id}")
        if not resp.ok:
            logger.error("  ✗ engagement '%s' not found (HTTP %d)", eng["name"], resp.status_code)
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="engagements", resource_id=server_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body
        failures = compare_item("engagements", server_id, eng, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
        else:
            vr.failed += 1


def verify_findings(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="findings")
    report.results.append(vr)

    findings = load_json(data_dir / "findings" / "findings.json")
    if not findings:
        return

    check_fields = [
        "title", "severity", "active", "false_positive",
        "component_name", "component_version",
    ]
    # Verify SLA field presence
    sla_missing_count = 0

    logger.info("=== Verifying findings (%d) ===", len(findings))
    for finding in findings:
        local_id = finding["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v2/findings/{server_id}")
        if not resp.ok:
            logger.error(
                "  ✗ finding '%s' not found (HTTP %d)",
                finding.get("title", "")[:40], resp.status_code,
            )
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="findings", resource_id=server_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body
        failures = compare_item("findings", server_id, finding, actual, check_fields, vr.mismatches)

        # Check SLA expiration date is auto-computed (SEED-003.4 acceptance criteria)
        if not actual.get("sla_expiration_date"):
            sla_missing_count += 1
            vr.mismatches.append(
                Mismatch(
                    domain="findings",
                    resource_id=server_id,
                    field="sla_expiration_date",
                    expected="<auto-computed>",
                    actual=None,
                    note="Backend should auto-compute SLA from product SLA configuration (SEED-003.4)",
                )
            )
            failures += 1

        if failures == 0:
            vr.passed += 1
        else:
            vr.failed += 1

    if sla_missing_count:
        logger.warning(
            "  ⚠ %d findings missing sla_expiration_date — backend may not auto-compute SLA (SEED-003.4)",
            sla_missing_count,
        )


def verify_sla_configurations(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="sla_configurations")
    report.results.append(vr)

    configs = load_json(data_dir / "sla" / "sla_configurations.json")
    if not configs:
        return

    check_fields = ["name", "critical", "high", "medium", "low"]
    # Field name mapping: seed uses *_days suffix, API may return plain names
    field_aliases = {
        "critical": ["critical", "critical_days"],
        "high": ["high", "high_days"],
        "medium": ["medium", "medium_days"],
        "low": ["low", "low_days"],
    }
    logger.info("=== Verifying SLA configurations (%d) ===", len(configs))

    for cfg in configs:
        local_id = cfg["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v2/sla-configurations/{server_id}")
        if not resp.ok:
            logger.error("  ✗ sla_config '%s' not found (HTTP %d)", cfg["name"], resp.status_code)
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="sla_configurations", resource_id=server_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body

        # Resolve field aliases
        resolved_actual: dict = {}
        for canonical, aliases in field_aliases.items():
            for alias in aliases:
                if alias in actual:
                    resolved_actual[canonical] = actual[alias]
                    break

        merged_actual = {**actual, **resolved_actual}
        # Translate seed field names
        expected_translated = {
            "name": cfg["name"],
            "critical": cfg["critical_days"],
            "high": cfg["high_days"],
            "medium": cfg["medium_days"],
            "low": cfg["low_days"],
        }

        failures = compare_item(
            "sla_configurations", server_id, expected_translated, merged_actual,
            check_fields, vr.mismatches,
        )
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ sla_config '%s'", cfg["name"])
        else:
            vr.failed += 1


def verify_assets(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="assets")
    report.results.append(vr)

    assets = load_json(data_dir / "assets" / "assets.json")
    if not assets:
        return

    check_fields = ["ip_address", "hostname", "os"]
    logger.info("=== Verifying assets (%d) ===", len(assets))

    for asset in assets:
        local_id = asset["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        resp = client.get(f"/api/v1/assets/{server_id}")
        if not resp.ok:
            # Fallback: search by IP
            resp2 = client.get("/api/v1/assets", params={"query": asset["ip_address"], "limit": 5})
            found = None
            if resp2.ok:
                body2 = resp2.json()
                items = body2 if isinstance(body2, list) else body2.get("assets", body2.get("data", []))
                for a in items:
                    if a.get("ip_address") == asset["ip_address"]:
                        found = a
                        break
            if not found:
                logger.error("  ✗ asset %s not found (HTTP %d)", asset["ip_address"], resp.status_code)
                vr.missing_on_server += 1
                vr.failed += 1
                vr.mismatches.append(
                    Mismatch(domain="assets", resource_id=local_id, field="<resource>",
                             expected="exists", actual=str(resp.status_code))
                )
                continue
            actual = found
        else:
            body = resp.json()
            actual = body.get("data", body) if isinstance(body, dict) else body

        failures = compare_item("assets", server_id, asset, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ asset %s", asset["ip_address"])
        else:
            vr.failed += 1


def verify_webhooks(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="webhooks")
    report.results.append(vr)

    webhooks = load_json(data_dir / "notifications" / "webhooks.json")
    if not webhooks:
        return

    check_fields = ["url", "description"]
    logger.info("=== Verifying webhooks (%d) ===", len(webhooks))

    # Fetch list from server
    resp = client.get("/api/v1/webhooks")
    if not resp.ok:
        logger.error("  ✗ GET /api/v1/webhooks failed: HTTP %d", resp.status_code)
        vr.failed += len(webhooks)
        return

    body = resp.json()
    server_webhooks = body if isinstance(body, list) else body.get("webhooks", body.get("data", []))
    server_url_map = {w.get("url", ""): w for w in server_webhooks}

    for wh in webhooks:
        vr.checked += 1
        server_wh = server_url_map.get(wh["url"])
        if not server_wh:
            logger.error("  ✗ webhook '%s' not found on server", wh["url"][:50])
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="webhooks", resource_id=wh["_id"], field="<resource>",
                         expected="exists", actual="not_found")
            )
        else:
            failures = compare_item("webhooks", wh["_id"], wh, server_wh, check_fields, vr.mismatches)
            if failures == 0:
                vr.passed += 1
            else:
                vr.failed += 1


# ---------------------------------------------------------------------------
# Additional verifiers (SEED-004, SEED-005, SEED-006, CR-014)
# ---------------------------------------------------------------------------


def verify_agents(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="agents")
    report.results.append(vr)

    agents = load_json(data_dir / "agents" / "agents.json")
    if not agents:
        return

    check_fields = ["name", "hostname", "ip_address", "os"]
    logger.info("=== Verifying agents (%d) ===", len(agents))

    # Try listing all agents
    resp = client.get("/api/v1/agents")
    server_agents: list[dict] = []
    if resp.ok:
        body = resp.json()
        server_agents = body if isinstance(body, list) else body.get("agents", body.get("data", []))
    server_hostname_map = {a.get("hostname", ""): a for a in server_agents}

    for agent in agents:
        local_id = agent["_id"]
        server_id = id_map.get(local_id)
        vr.checked += 1

        # Try by server ID first, fallback to hostname lookup
        actual = None
        if server_id:
            resp2 = client.get(f"/api/v1/agents/{server_id}")
            if resp2.ok:
                body2 = resp2.json()
                actual = body2.get("data", body2) if isinstance(body2, dict) else body2
        if not actual:
            actual = server_hostname_map.get(agent["hostname"])

        if not actual:
            logger.error("  ✗ agent '%s' not found on server", agent["hostname"])
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="agents", resource_id=local_id, field="<resource>",
                         expected="exists", actual="not_found")
            )
            continue

        failures = compare_item("agents", server_id or local_id, agent, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ agent '%s'", agent["hostname"])
        else:
            vr.failed += 1


def verify_custom_cves(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="custom_cves")
    report.results.append(vr)

    cves = load_json(data_dir / "cves" / "custom_cves.json")
    if not cves:
        return

    check_fields = ["id", "summary", "severity"]
    logger.info("=== Verifying custom CVEs (%d) ===", len(cves))

    for cve in cves:
        local_id = cve["_id"]
        cve_id = cve["id"]
        vr.checked += 1

        resp = client.get(f"/api/v2/cve/{cve_id}")
        if not resp.ok:
            logger.error("  ✗ custom_cve '%s' not found (HTTP %d)", cve_id, resp.status_code)
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="custom_cves", resource_id=cve_id, field="<resource>",
                         expected="exists", actual=str(resp.status_code))
            )
            continue

        body = resp.json()
        actual = body.get("data", body) if isinstance(body, dict) else body
        failures = compare_item("custom_cves", cve_id, cve, actual, check_fields, vr.mismatches)
        if failures == 0:
            vr.passed += 1
            logger.info("  ✓ custom_cve '%s'", cve_id)
        else:
            vr.failed += 1


def verify_scheduled_scans(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="scheduled_scans")
    report.results.append(vr)

    scans = load_json(data_dir / "scans" / "scheduled_scans.json")
    if not scans:
        return

    check_fields = ["cron_expr", "scan_type"]
    logger.info("=== Verifying scheduled scans (%d) ===", len(scans))

    resp = client.get("/api/v1/scans/scheduled")
    if not resp.ok:
        logger.error("  ✗ GET /api/v1/scans/scheduled failed: HTTP %d", resp.status_code)
        vr.failed += len(scans)
        for scan in scans:
            vr.mismatches.append(
                Mismatch(domain="scheduled_scans", resource_id=scan["_id"], field="<resource>",
                         expected="exists", actual=f"HTTP {resp.status_code}")
            )
        return

    body = resp.json()
    server_scans = body if isinstance(body, list) else body.get("scheduled_scans", body.get("data", []))
    server_cron_map = {s.get("cron_expr", ""): s for s in server_scans}

    for scan in scans:
        vr.checked += 1
        server_scan = server_cron_map.get(scan["cron_expr"])
        if not server_scan:
            server_id = id_map.get(scan["_id"])
            if server_id:
                resp2 = client.get(f"/api/v1/scans/scheduled/{server_id}")
                if resp2.ok:
                    body2 = resp2.json()
                    server_scan = body2.get("data", body2) if isinstance(body2, dict) else body2

        if not server_scan:
            logger.error("  ✗ scheduled_scan '%s' not found on server", scan["cron_expr"])
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="scheduled_scans", resource_id=scan["_id"], field="<resource>",
                         expected="exists", actual="not_found")
            )
        else:
            failures = compare_item("scheduled_scans", scan["_id"], scan, server_scan,
                                    check_fields, vr.mismatches)
            if failures == 0:
                vr.passed += 1
                logger.info("  ✓ scheduled_scan '%s'", scan["cron_expr"])
            else:
                vr.failed += 1


def verify_subscriptions(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="subscriptions")
    report.results.append(vr)

    subs = load_json(data_dir / "notifications" / "subscriptions.json")
    if not subs:
        return

    check_fields = ["type", "value", "min_severity"]
    logger.info("=== Verifying subscriptions (%d) ===", len(subs))

    resp = client.get("/api/v2/subscriptions")
    if not resp.ok:
        logger.error("  ✗ GET /api/v2/subscriptions failed: HTTP %d", resp.status_code)
        vr.failed += len(subs)
        return

    body = resp.json()
    server_subs = body if isinstance(body, list) else body.get("subscriptions", body.get("data", []))
    # Build composite key map: (type, value)
    server_key_map = {(s.get("type", ""), s.get("value", "")): s for s in server_subs}

    for sub in subs:
        vr.checked += 1
        key = (sub["type"], sub.get("value", ""))
        server_sub = server_key_map.get(key)
        if not server_sub:
            logger.error("  ✗ subscription type=%s value=%s not found", sub["type"], sub.get("value", ""))
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="subscriptions", resource_id=sub["_id"], field="<resource>",
                         expected="exists", actual="not_found")
            )
        else:
            failures = compare_item("subscriptions", sub["_id"], sub, server_sub,
                                    check_fields, vr.mismatches)
            if failures == 0:
                vr.passed += 1
                logger.info("  ✓ subscription type=%s value=%s", sub["type"], sub.get("value", ""))
            else:
                vr.failed += 1


def verify_notification_rules(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    vr = VerifyResult(domain="notification_rules")
    report.results.append(vr)

    rules = load_json(data_dir / "notifications" / "notification_rules.json")
    if not rules:
        return

    check_fields = ["sla_breach"]
    logger.info("=== Verifying notification rules (%d) ===", len(rules))

    for rule in rules:
        local_id = rule["_id"]
        server_id = id_map.get(local_id)
        product_server_id = id_map.get(rule["product_id"])
        vr.checked += 1

        actual = None
        if server_id:
            resp = client.get(f"/api/v2/notification-rules/{server_id}")
            if resp.ok:
                body = resp.json()
                actual = body.get("data", body) if isinstance(body, dict) else body

        if not actual and product_server_id:
            resp2 = client.get("/api/v2/notification-rules", params={"product_id": product_server_id})
            if resp2.ok:
                body2 = resp2.json()
                items = body2 if isinstance(body2, list) else body2.get("rules", body2.get("data", []))
                if items:
                    actual = items[0]

        if not actual:
            logger.error("  ✗ notification_rule for product %s not found",
                         product_server_id or rule["product_id"])
            vr.missing_on_server += 1
            vr.failed += 1
            vr.mismatches.append(
                Mismatch(domain="notification_rules", resource_id=local_id, field="<resource>",
                         expected="exists", actual="not_found")
            )
        else:
            failures = compare_item("notification_rules", local_id, rule, actual,
                                    check_fields, vr.mismatches)
            if failures == 0:
                vr.passed += 1
            else:
                vr.failed += 1


def verify_ai_triage(
    client: SeedClient,
    data_dir: Path,
    id_map: IdMap,
    report: Report,
) -> None:
    """Verify AI triage queue — checks that items exist and have the right schema (CR-014)."""
    vr = VerifyResult(domain="ai_triage")
    report.results.append(vr)

    triage_items = load_json(data_dir / "ai" / "triage_queue.json")
    if not triage_items:
        return

    logger.info("=== Verifying AI triage queue (%d items) ===", len(triage_items))

    # Fetch the triage queue from the server
    resp = client.get("/api/v1/ai/triage/queue", params={"page_size": 100})
    if not resp.ok:
        logger.error("  ✗ GET /api/v1/ai/triage/queue failed: HTTP %d", resp.status_code)
        vr.failed += len(triage_items)
        for item in triage_items:
            vr.mismatches.append(
                Mismatch(domain="ai_triage", resource_id=item["_id"], field="<resource>",
                         expected="exists", actual=f"HTTP {resp.status_code}")
            )
        return

    body = resp.json()
    server_items = body.get("items", [])
    stats = body.get("stats", {})

    # Check CR-014 schema: stats block must be present
    if not stats:
        vr.mismatches.append(
            Mismatch(domain="ai_triage", resource_id="queue", field="stats",
                     expected="present", actual="missing",
                     note="CR-014: stats block must be in response")
        )
        vr.failed += 1
    else:
        required_stat_fields = ["pending", "accepted_today", "avg_confidence", "false_positive_rate"]
        for sf in required_stat_fields:
            if sf not in stats:
                vr.mismatches.append(
                    Mismatch(domain="ai_triage", resource_id="queue", field=f"stats.{sf}",
                             expected="present", actual="missing",
                             note="CR-014 stats schema")
                )
                vr.failed += 1
        if vr.failed == 0:
            logger.info("  ✓ AI triage stats schema OK: %s", stats)

    # Build finding_id map from server items
    server_finding_map = {i.get("finding_id", ""): i for i in server_items}

    # Check each seeded item
    for item in triage_items:
        local_id = item["_id"]
        finding_server_id = id_map.get(item["finding_id"])
        vr.checked += 1

        server_item = server_finding_map.get(finding_server_id or item["finding_id"])

        if not server_item:
            # Queue may not have this item (triage pipeline not triggered) — warn not error
            logger.warning("  ⚠ ai_triage for finding %s not in queue — pipeline may not have run",
                           finding_server_id or item["finding_id"])
            vr.missing_on_server += 1
            vr.skipped += 1
            continue

        # Check ai_result schema (CR-014.2)
        ai_result = server_item.get("ai_result")
        if not ai_result:
            vr.mismatches.append(
                Mismatch(domain="ai_triage", resource_id=local_id, field="ai_result",
                         expected="present", actual="missing",
                         note="CR-014: each item must have ai_result object")
            )
            vr.failed += 1
        else:
            for rf in ["remarks", "confidence", "justification", "actions", "generated_at"]:
                if rf not in ai_result:
                    vr.mismatches.append(
                        Mismatch(domain="ai_triage", resource_id=local_id, field=f"ai_result.{rf}",
                                 expected="present", actual="missing",
                                 note="CR-014 ai_result schema")
                    )
                    vr.failed += 1
            vr.passed += 1
            logger.info("  ✓ ai_triage for finding %s — %s (%.0f%%)",
                        finding_server_id or item["finding_id"],
                        ai_result.get("remarks", "?"),
                        ai_result.get("confidence", 0) * 100)


# ---------------------------------------------------------------------------
# Report rendering
# ---------------------------------------------------------------------------


def render_report(report: Report, output_path: Path) -> None:
    """Serialize the full report to JSON."""
    data = {
        "summary": report.summary(),
        "total_mismatches": report.total_mismatches,
        "total_missing": report.total_missing,
        "domains": [
            {
                "domain": r.domain,
                "checked": r.checked,
                "passed": r.passed,
                "failed": r.failed,
                "skipped": r.skipped,
                "missing_on_server": r.missing_on_server,
                "mismatches": [asdict(m) for m in r.mismatches],
            }
            for r in report.results
        ],

    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")
    logger.info("Report written → %s", output_path)


def print_bug_report(report: Report) -> None:
    """Print a human-readable bug report to stdout."""
    print("\n" + "=" * 70)
    print("BACKEND VERIFICATION REPORT")
    print("=" * 70)
    print(f"Summary: {report.summary()}")
    print()

    for vr in report.results:
        if not vr.mismatches and vr.missing_on_server == 0:
            print(f"  ✓ [{vr.domain}] {vr.passed}/{vr.checked} passed")
        else:
            print(f"  ✗ [{vr.domain}] {vr.passed}/{vr.checked} passed, "
                  f"{vr.failed} failed, {vr.missing_on_server} missing")
            for m in vr.mismatches:
                extra = f" ({m.note})" if m.note else ""
                print(f"      • {m.resource_id[:12]}::{m.field} — "
                      f"expected={m.expected!r}, got={m.actual!r}{extra}")

    print()
    if report.total_mismatches == 0 and report.total_missing == 0:
        print("✅ All checks passed — backend data matches seed data.")
    else:
        print(
            f"⚠️  Found {report.total_mismatches} field mismatch(es) and "
            f"{report.total_missing} missing resource(s)."
        )
        print("   Review the JSON report for details and file backend bug reports.")
    print("=" * 70 + "\n")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Verify seed data against backend")
    p.add_argument("--env", default=None, help="Path to .env file")
    p.add_argument("--data", default=None, help="Seed data directory (overrides SEED_DATA_DIR)")
    p.add_argument(
        "--domain",
        default="",
        help=(
            "Comma-separated list of domains to verify. "
            "Options: users,product_types,products,engagements,findings,sla,assets,webhooks,"
            "agents,custom_cves,scheduled_scans,subscriptions,notification_rules,ai_triage. Default: all"
        ),
    )
    return p.parse_args()


_ALL_DOMAINS = [
    "users", "product_types", "products", "engagements",
    "findings", "sla", "assets", "webhooks",
    "agents", "custom_cves", "scheduled_scans",
    "subscriptions", "notification_rules", "ai_triage",
]

_VERIFIERS = {
    "users": verify_users,
    "product_types": verify_product_types,
    "products": verify_products,
    "engagements": verify_engagements,
    "findings": verify_findings,
    "sla": verify_sla_configurations,
    "assets": verify_assets,
    "webhooks": verify_webhooks,
    "agents": verify_agents,
    "custom_cves": verify_custom_cves,
    "scheduled_scans": verify_scheduled_scans,
    "subscriptions": verify_subscriptions,
    "notification_rules": verify_notification_rules,
    "ai_triage": verify_ai_triage,
}


def main() -> None:
    args = parse_args()
    cfg = SeedConfig(env_file=args.env)
    cfg.validate()
    log = cfg.setup_logging()

    data_dir: Path = Path(args.data) if args.data else cfg.seed_data_dir
    output_dir: Path = cfg.seed_output_dir
    id_map_path = output_dir / "id_map.json"
    report_path = output_dir / "verify_report.json"

    domains = (
        [d.strip() for d in args.domain.split(",") if d.strip()]
        if args.domain
        else _ALL_DOMAINS
    )

    invalid = [d for d in domains if d not in _ALL_DOMAINS]
    if invalid:
        log.error("Unknown domain(s): %s. Valid: %s", invalid, _ALL_DOMAINS)
        sys.exit(1)

    client = SeedClient(cfg)

    # Health & auth
    if not client.health_check():
        log.error("Gateway at %s is unreachable. Aborting.", cfg.gateway_url)
        sys.exit(2)
    log.info("Gateway health check OK")
    client.authenticate()

    id_map = IdMap(id_map_path)
    report = Report()

    log.info("=== Starting verification for domains: %s ===", domains)

    for domain in domains:
        verifier = _VERIFIERS[domain]
        verifier(client, data_dir, id_map, report)

    render_report(report, report_path)
    print_bug_report(report)

    exit_code = 0 if (report.total_mismatches == 0 and report.total_missing == 0) else 1
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
