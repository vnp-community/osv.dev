#!/usr/bin/env python3
"""
02_push_seed_data.py — Load seed data files and push them to the backend via REST API.

Seeding order (respects foreign-key dependencies):
  1.  Identity      → users, api_keys
  2.  SLA           → sla_configurations (bulk)
  3.  Products      → product_types, products, engagements, tests
  4.  SLA Assign    → assign SLA configs to products
  5.  Findings      → findings (bulk), finding_notes, finding_groups
  6.  CVEs          → custom_cves, cve_triages
  7.  Ranking       → ranking_entries (bulk)
  8.  Assets        → assets (bulk), asset_vulnerabilities
  9.  Agents        → agents, agent_reports
  10. Scans         → scheduled_scans
  11. Notifications → notification_rules (bulk), subscriptions (bulk), webhooks (bulk)
  12. Config        → jira_configurations, system_notification_rules
  13. AI            → ai_triage_queue

All created IDs are written to ./data/output/id_map.json so that
Script 3 (verify) can reconcile them.

Usage:
    python 02_push_seed_data.py [--env .env] [--data ./data] [--dry-run] [--skip DOMAIN...]
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from pathlib import Path
from typing import Any

# ---------------------------------------------------------------------------
# Local imports
# ---------------------------------------------------------------------------
sys.path.insert(0, str(Path(__file__).parent))
from seed_config import SeedConfig  # noqa: E402
from seed_client import SeedClient  # noqa: E402

logger = logging.getLogger("seed.push")


# ---------------------------------------------------------------------------
# ID mapping — maps local "_id" → server-assigned "id"
# ---------------------------------------------------------------------------


class IdMap:
    """Stores local_id → server_id mappings, persisted to JSON."""

    def __init__(self, path: Path) -> None:
        self._path = path
        self._map: dict[str, str] = {}
        if path.exists():
            self._map = json.loads(path.read_text(encoding="utf-8"))

    def put(self, local_id: str, server_id: str) -> None:
        self._map[local_id] = server_id

    def get(self, local_id: str) -> str | None:
        return self._map.get(local_id)  # returns None if not mapped

    def save(self) -> None:
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._path.write_text(
            json.dumps(self._map, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
        logger.debug("ID map saved → %s (%d entries)", self._path, len(self._map))

    def resolve(self, obj: Any) -> Any:
        """Recursively replace any local _id values in *obj* with server IDs."""
        if isinstance(obj, dict):
            return {k: self.resolve(v) for k, v in obj.items() if k != "_id"}
        if isinstance(obj, list):
            return [self.resolve(item) for item in obj]
        if isinstance(obj, str) and obj in self._map:
            return self._map[obj]
        return obj


# ---------------------------------------------------------------------------
# Result tracker
# ---------------------------------------------------------------------------


class SeedResult:
    def __init__(self) -> None:
        self.success: list[dict] = []
        self.failed: list[dict] = []
        self.skipped: list[dict] = []

    def record(self, domain: str, local_id: str, server_id: str | None, status: str) -> None:
        entry = {"domain": domain, "local_id": local_id, "server_id": server_id, "status": status}
        if status == "created":
            self.success.append(entry)
        elif status == "skipped":
            self.skipped.append(entry)
        else:
            self.failed.append(entry)

    def summary(self) -> str:
        return (
            f"created={len(self.success)}, "
            f"failed={len(self.failed)}, "
            f"skipped={len(self.skipped)}"
        )


# ---------------------------------------------------------------------------
# Loader helpers
# ---------------------------------------------------------------------------


def load_json(path: Path) -> list[dict]:
    if not path.exists():
        logger.warning("Data file not found: %s — skipping", path)
        return []
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, dict):
        # Some files may be wrapped in an object
        data = list(data.values())[0] if len(data) == 1 else [data]
    return data  # type: ignore[return-value]


def load_json_obj(path: Path) -> dict:
    """Load a JSON file that is expected to be a dict (not a list)."""
    if not path.exists():
        logger.warning("Data file not found: %s — skipping", path)
        return {}
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, list):
        return data[0] if data else {}
    return data


# ---------------------------------------------------------------------------
# Domain seeders
# ---------------------------------------------------------------------------


def seed_users(
    client: SeedClient,
    users: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed users via POST /api/v1/admin/users (SEED-001)."""
    logger.info("=== Seeding users (%d) ===", len(users))
    for user in users:
        local_id = user["_id"]
        payload = {
            "email": user["email"],
            "username": user["username"],
            "password": user["password"],
            "role": user.get("role", "user"),
            "is_active": user.get("is_active", True),
            "is_verified": user.get("is_verified", True),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/admin/users %s", user["email"])
            result.record("user", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v1/admin/users", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id") or data.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("user", local_id, server_id, "created")
            logger.info("  ✓ user %s → %s", user["email"], server_id)
        elif resp.status_code == 409 or (resp.status_code == 400 and any(
            kw in resp.text.lower() for kw in ("duplicate", "already", "exists", "conflict")
        )):
            logger.warning("  ↩ user %s already exists (%d)", user["email"], resp.status_code)
            existing = _find_user_by_email(client, user["email"])
            if existing:
                id_map.put(local_id, existing)
                result.record("user", local_id, existing, "skipped")
            else:
                result.record("user", local_id, None, "failed")
        else:
            logger.error(
                "  ✗ user %s failed: HTTP %d — %s",
                user["email"], resp.status_code, resp.text[:200],
            )
            result.record("user", local_id, None, "failed")


def _find_user_by_email(client: SeedClient, email: str) -> str | None:
    """Find user ID by email from admin list."""
    resp = client.get("/api/v1/admin/users", params={"q": email, "page_size": 5})
    if not resp.ok:
        return None
    try:
        data = resp.json()
        users = data if isinstance(data, list) else data.get("users", data.get("data", []))
        for u in users:
            if u.get("email") == email:
                return u.get("id")
    except Exception:
        pass
    return None


def seed_api_keys(
    client: SeedClient,
    api_keys: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed API keys via POST /api/v1/admin/users/{id}/api-keys (SEED-001)."""
    logger.info("=== Seeding API keys (%d) ===", len(api_keys))
    for key in api_keys:
        local_id = key["_id"]
        user_server_id = id_map.get(key["user_id"])
        payload = {
            "name": key["name"],
            "scopes": key.get("scopes", []),
            "expires_at": key.get("expires_at"),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/admin/users/%s/api-keys", user_server_id)
            result.record("api_key", local_id, None, "skipped")
            continue

        if not user_server_id or user_server_id.startswith("dry-"):
            logger.warning("  ⚠ api_key '%s' skipped: user not seeded", key["name"])
            result.record("api_key", local_id, None, "skipped")
            continue

        resp = client.post(f"/api/v1/admin/users/{user_server_id}/api-keys", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            api_key_value = data.get("key", "")
            id_map.put(local_id, server_id)
            result.record("api_key", local_id, server_id, "created")
            logger.info("  ✓ api_key '%s' → %s (key: %s...)", key["name"], server_id, api_key_value[:12])
        else:
            logger.error(
                "  ✗ api_key '%s' failed: HTTP %d — %s",
                key["name"], resp.status_code, resp.text[:200],
            )
            result.record("api_key", local_id, None, "failed")


def seed_sla_configurations(
    client: SeedClient,
    configs: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed SLA configurations via bulk endpoint, fallback to individual POST (SEED-006.1)."""
    logger.info("=== Seeding SLA configurations (%d) ===", len(configs))
    if dry_run:
        for cfg in configs:
            local_id = cfg["_id"]
            result.record("sla_config", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
        return

    # Try bulk endpoint first (SEED-006.1)
    bulk_payload = {
        "configurations": [
            {
                "name": c["name"],
                "description": c.get("description", ""),
                "critical_days": c["critical_days"],
                "high_days": c["high_days"],
                "medium_days": c["medium_days"],
                "low_days": c["low_days"],
                "is_default": c.get("is_default", False),
            }
            for c in configs
        ]
    }
    resp = client.post("/api/v2/sla-configurations/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        bulk_results = data.get("results", [])
        for i, item in enumerate(bulk_results):
            if i < len(configs):
                local_id = configs[i]["_id"]
                server_id = item.get("id", "")
                id_map.put(local_id, server_id)
                result.record("sla_config", local_id, server_id, item.get("status", "created"))
        logger.info("  ✓ bulk SLA configs: created=%d", data.get("created_count", len(bulk_results)))
        return

    logger.warning("bulk sla-configurations failed (HTTP %d) — fallback to individual POST", resp.status_code)
    for cfg in configs:
        local_id = cfg["_id"]
        payload = {
            "name": cfg["name"],
            "description": cfg.get("description", ""),
            "critical": cfg["critical_days"],
            "high": cfg["high_days"],
            "medium": cfg["medium_days"],
            "low": cfg["low_days"],
        }
        resp2 = client.post("/api/v2/sla-configurations", body=payload)
        if resp2.status_code in (200, 201):
            data2 = resp2.json()
            server_id = data2.get("id") or data2.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("sla_config", local_id, server_id, "created")
            logger.info("  ✓ sla_config '%s' → %s", cfg["name"], server_id)
        else:
            logger.error("  ✗ sla_config '%s' failed: HTTP %d — %s",
                         cfg["name"], resp2.status_code, resp2.text[:200])
            result.record("sla_config", local_id, None, "failed")


def seed_product_types(
    client: SeedClient,
    product_types: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed product types — tries POST /api/v2/product-types, falls back to local UUID (SEED-002)."""
    logger.info("=== Seeding product types (%d) ===", len(product_types))
    import uuid as _uuid
    for pt in product_types:
        local_id = pt["_id"]
        payload = {
            "name": pt["name"],
            "description": pt.get("description", ""),
            "critical_product": pt.get("critical_product", False),
            "key_product": pt.get("key_product", False),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v2/product-types %s", pt["name"])
            result.record("product_type", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v2/product-types", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id") or data.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("product_type", local_id, server_id, "created")
            logger.info("  ✓ product_type '%s' → %s", pt["name"], server_id)
        elif resp.status_code == 409:
            logger.warning("  ↩ product_type '%s' already exists", pt["name"])
            # Try to look it up
            resp2 = client.get("/api/v2/product-types", params={"name": pt["name"]})
            if resp2.ok:
                body2 = resp2.json()
                items = body2 if isinstance(body2, list) else body2.get("data", body2.get("product_types", []))
                for item in items:
                    if item.get("name") == pt["name"]:
                        id_map.put(local_id, item.get("id", ""))
                        result.record("product_type", local_id, item.get("id"), "skipped")
                        break
            else:
                # Assign stable local UUID to maintain referential integrity
                fake_id = str(_uuid.uuid5(_uuid.NAMESPACE_DNS, pt["name"]))
                id_map.put(local_id, fake_id)
                result.record("product_type", local_id, fake_id, "skipped")
        else:
            # product-type endpoint may not exist — use stable local UUID
            fake_id = str(_uuid.uuid5(_uuid.NAMESPACE_DNS, pt["name"]))
            id_map.put(local_id, fake_id)
            result.record("product_type", local_id, fake_id, "skipped")
            logger.info("  ↩ product_type '%s' → local id %s (endpoint HTTP %d)",
                        pt["name"], fake_id, resp.status_code)


def seed_products(
    client: SeedClient,
    products: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed products via POST /api/v1/products (SEED-002)."""
    logger.info("=== Seeding products (%d) ===", len(products))
    for prod in products:
        local_id = prod["_id"]
        pt_server_id = id_map.get(prod["product_type_id"])
        payload = {
            "name": prod["name"],
            "description": prod.get("description", ""),
            "product_type_id": pt_server_id,
            "business_criticality": prod.get("business_criticality", "medium"),
            "platform": prod.get("platform", "web"),
            "lifecycle": prod.get("lifecycle", "production"),
            "origin": prod.get("origin", "internal"),
            "external_audience": prod.get("external_audience", False),
            "internet_accessible": prod.get("internet_accessible", False),
            "enable_full_risk_acceptance": prod.get("enable_full_risk_acceptance", False),
            "enable_simple_risk_acceptance": prod.get("enable_simple_risk_acceptance", True),
            "tags": prod.get("tags", []),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/products %s", prod["name"])
            result.record("product", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v1/products", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id") or data.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("product", local_id, server_id, "created")
            logger.info("  ✓ product '%s' → %s", prod["name"], server_id)
        elif resp.status_code == 409:
            logger.warning("  ↩ product '%s' already exists", prod["name"])
            result.record("product", local_id, None, "skipped")
        else:
            logger.error("  ✗ product '%s' failed: HTTP %d — %s",
                         prod["name"], resp.status_code, resp.text[:200])
            result.record("product", local_id, None, "failed")


def seed_engagements(
    client: SeedClient,
    engagements: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed engagements via POST /api/v1/products/{id}/engagements (SEED-002)."""
    logger.info("=== Seeding engagements (%d) ===", len(engagements))
    for eng in engagements:
        local_id = eng["_id"]
        product_server_id = id_map.get(eng["product_id"])
        lead_server_id = id_map.get(eng.get("lead_id", "")) if eng.get("lead_id") else None
        payload = {
            "product_id": product_server_id,
            "name": eng["name"],
            "description": eng.get("description", ""),
            "engagement_type": eng.get("engagement_type", "Interactive"),
            "status": eng.get("status", "In Progress"),
            "start_date": eng.get("start_date"),
            "end_date": eng.get("end_date"),
            "version": eng.get("version", "1.0.0"),
            "tags": eng.get("tags", []),
            "deduplication_on_engagement": eng.get("deduplication_on_engagement", True),
        }
        if lead_server_id:
            payload["lead_id"] = lead_server_id
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/products/%s/engagements %s", product_server_id, eng["name"])
            result.record("engagement", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        if not product_server_id:
            logger.warning("  ✗ engagement '%s' skipped: no product_id mapped", eng["name"])
            result.record("engagement", local_id, None, "failed")
            continue

        resp = client.post(f"/api/v1/products/{product_server_id}/engagements", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id") or data.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("engagement", local_id, server_id, "created")
            logger.info("  ✓ engagement '%s' → %s", eng["name"], server_id)
        else:
            logger.error("  ✗ engagement '%s' failed: HTTP %d — %s",
                         eng["name"], resp.status_code, resp.text[:200])
            result.record("engagement", local_id, None, "failed")


def seed_tests(
    client: SeedClient,
    tests: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed tests via POST /api/v1/engagements/{id}/tests (SEED-002)."""
    logger.info("=== Seeding tests (%d) ===", len(tests))
    for test in tests:
        local_id = test["_id"]
        eng_server_id = id_map.get(test["engagement_id"])
        payload = {
            "engagement_id": eng_server_id,
            "scan_type": test.get("scan_type", "Manual Pentest"),
            "title": test["title"],
            "description": test.get("description", ""),
            "target_start": test.get("target_start"),
            "target_end": test.get("target_end"),
            "tags": test.get("tags", []),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/engagements/%s/tests %s", eng_server_id, test["title"])
            result.record("test", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        if not eng_server_id:
            logger.warning("  ✗ test '%s' skipped: no engagement_id mapped", test["title"])
            result.record("test", local_id, None, "failed")
            continue

        resp = client.post(f"/api/v1/engagements/{eng_server_id}/tests", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id") or data.get("data", {}).get("id", "")
            id_map.put(local_id, server_id)
            result.record("test", local_id, server_id, "created")
            logger.info("  ✓ test '%s' → %s", test["title"], server_id)
        else:
            logger.error("  ✗ test '%s' failed: HTTP %d — %s",
                         test["title"], resp.status_code, resp.text[:200])
            result.record("test", local_id, None, "failed")


def seed_sla_assignments(
    client: SeedClient,
    assignments: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Assign SLA configurations to products (SEED-006.2)."""
    logger.info("=== Seeding SLA assignments (%d) ===", len(assignments))
    if dry_run:
        for a in assignments:
            result.record("sla_assignment", a["_id"], None, "skipped")
        return

    # Try bulk assign endpoint first
    bulk_payload = {
        "assignments": [
            {
                "product_id": id_map.get(a["product_id"]) or a["product_id"],
                "sla_configuration_id": id_map.get(a["sla_configuration_id"]) or a["sla_configuration_id"],
            }
            for a in assignments
            if id_map.get(a["product_id"])  # only include products that were seeded
        ]
    }
    if not bulk_payload["assignments"]:
        logger.warning("  ⚠ No SLA assignments to push — products not seeded yet")
        return

    resp = client.post("/api/v2/sla-configurations/assign-bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        assigned = data.get("assigned_count", len(bulk_payload["assignments"]))
        logger.info("  ✓ bulk SLA assignments: assigned=%d", assigned)
        for a in assignments:
            result.record("sla_assignment", a["_id"], None, "created")
        return

    logger.warning("bulk SLA assign failed (HTTP %d) — fallback to individual assign", resp.status_code)
    for a in assignments:
        local_id = a["_id"]
        product_server_id = id_map.get(a["product_id"])
        sla_server_id = id_map.get(a["sla_configuration_id"])
        if not product_server_id or not sla_server_id:
            result.record("sla_assignment", local_id, None, "skipped")
            continue
        resp2 = client.post(
            f"/api/v2/sla-configurations/{sla_server_id}/assign/{product_server_id}",
            body={},
        )
        if resp2.status_code in (200, 201):
            result.record("sla_assignment", local_id, None, "created")
            logger.info("  ✓ SLA %s → product %s", sla_server_id[:8], product_server_id[:8])
        else:
            logger.warning("  ✗ SLA assign failed: HTTP %d", resp2.status_code)
            result.record("sla_assignment", local_id, None, "failed")


def seed_findings(
    client: SeedClient,
    findings: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed findings via bulk endpoint or individual POST (SEED-003)."""
    logger.info("=== Seeding findings (%d) ===", len(findings))

    # Group by test_id for bulk-create endpoint
    by_test: dict[str, list[dict]] = {}
    for f in findings:
        tid = f["test_id"]
        by_test.setdefault(tid, []).append(f)

    for local_test_id, batch_findings in by_test.items():
        server_test_id = id_map.get(local_test_id)
        payloads = []
        local_ids = []
        for f in batch_findings:
            local_ids.append(f["_id"])
            p = {
                "title": f["title"],
                "description": f.get("description", ""),
                "mitigation": f.get("mitigation", ""),
                "severity": f["severity"],
                "cve": f.get("cve") or "",
                "cwe": f.get("cwe"),
                "cvss_v3_score": f.get("cvss_v3_score"),
                "component_name": f.get("component_name", ""),
                "component_version": f.get("component_version", ""),
                "date": f.get("date"),
                "active": f.get("active", True),
                "verified": f.get("verified", False),
                "false_positive": f.get("false_positive", False),
                "is_kev": f.get("is_kev", False),
                "tags": f.get("tags", []),
                "assigned_to": f.get("assigned_to", ""),
                "created_by": f.get("created_by", ""),
            }
            payloads.append(p)

        if dry_run:
            for lid in local_ids:
                result.record("finding", lid, None, "skipped")
                id_map.put(lid, f"dry-{lid[:8]}")
            continue

        # Try bulk-create endpoint first
        bulk_payload = {
            "test_id": server_test_id,
            "findings": payloads,
            "auto_close_duplicates": True,
            "auto_enrich_cve": False,
        }
        resp = client.post("/api/v2/findings/bulk-create", body=bulk_payload)
        if resp.status_code in (200, 201, 207):
            data = resp.json()
            created_results = data.get("results", [])
            for i, item in enumerate(created_results):
                if i < len(local_ids):
                    server_id = item.get("id", "")
                    status = item.get("status", "created")
                    id_map.put(local_ids[i], server_id)
                    result.record("finding", local_ids[i], server_id, status)
            logger.info(
                "  ✓ bulk-created %d findings for test %s",
                data.get("created_count", len(created_results)), server_test_id,
            )
        else:
            logger.warning(
                "bulk-create failed (HTTP %d) — falling back to individual POST", resp.status_code,
            )
            for i, payload in enumerate(payloads):
                payload["test_id"] = server_test_id
                resp2 = client.post("/api/v2/findings", body=payload)
                local_id = local_ids[i]
                if resp2.status_code in (200, 201):
                    data2 = resp2.json()
                    server_id = data2.get("id") or data2.get("data", {}).get("id", "")
                    id_map.put(local_id, server_id)
                    result.record("finding", local_id, server_id, "created")
                else:
                    logger.error(
                        "  ✗ finding '%s' failed: HTTP %d — %s",
                        payload.get("title", ""), resp2.status_code, resp2.text[:150],
                    )
                    result.record("finding", local_id, None, "failed")


def seed_finding_notes(
    client: SeedClient,
    notes: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed finding notes via POST /api/v1/findings/{id}/notes (SEED-003)."""
    logger.info("=== Seeding finding notes (%d) ===", len(notes))
    for note in notes:
        local_id = note["_id"]
        finding_server_id = id_map.get(note["finding_id"])
        if not finding_server_id or finding_server_id.startswith("dry-"):
            result.record("finding_note", local_id, None, "skipped")
            continue

        payload = {
            "content": note["content"],
            "is_private": note.get("is_private", False),
        }
        if dry_run:
            result.record("finding_note", local_id, None, "skipped")
            continue

        resp = client.post(f"/api/v1/findings/{finding_server_id}/notes", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            id_map.put(local_id, server_id)
            result.record("finding_note", local_id, server_id, "created")
        else:
            logger.warning("  ✗ finding_note for %s failed: HTTP %d", finding_server_id, resp.status_code)
            result.record("finding_note", local_id, None, "failed")


def seed_finding_groups(
    client: SeedClient,
    groups: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed finding groups via POST /api/v2/finding-groups (SEED-003)."""
    logger.info("=== Seeding finding groups (%d) ===", len(groups))
    for grp in groups:
        local_id = grp["_id"]
        product_server_id = id_map.get(grp["product_id"])
        finding_server_ids = [id_map.get(fid) for fid in grp.get("finding_ids", [])]
        payload = {
            "name": grp["name"],
            "product_id": product_server_id,
            "finding_ids": finding_server_ids,
        }
        if dry_run:
            result.record("finding_group", local_id, None, "skipped")
            continue

        resp = client.post("/api/v2/finding-groups", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            id_map.put(local_id, server_id)
            result.record("finding_group", local_id, server_id, "created")
            logger.info("  ✓ finding_group '%s' → %s", grp["name"], server_id)
        else:
            logger.warning("  ✗ finding_group '%s' failed: HTTP %d — %s",
                           grp["name"], resp.status_code, resp.text[:150])
            result.record("finding_group", local_id, None, "failed")


# ---------------------------------------------------------------------------
# CVE seeders (SEED-004)
# ---------------------------------------------------------------------------


def seed_custom_cves(
    client: SeedClient,
    cves: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed custom CVE records via POST /api/v2/cve/custom (SEED-004.1)."""
    logger.info("=== Seeding custom CVEs (%d) ===", len(cves))
    for cve in cves:
        local_id = cve["_id"]
        payload = {
            "id": cve["id"],
            "summary": cve["summary"],
            "description": cve.get("description", ""),
            "severity": cve.get("severity", "medium"),
            "cvss3": cve.get("cvss3"),
            "cvss3_vector": cve.get("cvss3_vector"),
            "published": cve.get("published"),
            "source": cve.get("source", "INTERNAL"),
            "vendors": cve.get("vendors", []),
            "products": cve.get("products", []),
            "references": cve.get("references", []),
            "is_kev": cve.get("is_kev", False),
            "is_exploit": cve.get("is_exploit", False),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v2/cve/custom %s", cve["id"])
            result.record("custom_cve", local_id, None, "skipped")
            id_map.put(local_id, cve["id"])  # map by CVE ID
            continue

        resp = client.post("/api/v2/cve/custom", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", cve["id"])
            id_map.put(local_id, server_id)
            result.record("custom_cve", local_id, server_id, "created")
            logger.info("  ✓ custom_cve '%s' → %s", cve["id"], server_id)
        elif resp.status_code == 409:
            logger.warning("  ↩ custom_cve '%s' already exists", cve["id"])
            id_map.put(local_id, cve["id"])
            result.record("custom_cve", local_id, cve["id"], "skipped")
        else:
            logger.error("  ✗ custom_cve '%s' failed: HTTP %d — %s",
                         cve["id"], resp.status_code, resp.text[:200])
            result.record("custom_cve", local_id, None, "failed")


def seed_cve_triages(
    client: SeedClient,
    triages: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed CVE triage decisions via PUT /api/v2/cve/{id}/triage (SEED-004.2)."""
    logger.info("=== Seeding CVE triages (%d) ===", len(triages))
    for triage in triages:
        local_id = triage["_id"]
        cve_id = triage["cve_id"]
        payload = {
            "remarks": triage["remarks"],
            "comments": triage.get("comments", ""),
            "justification": triage.get("justification", ""),
            "response": triage.get("response", []),
        }
        if dry_run:
            logger.info("[DRY-RUN] PUT /api/v2/cve/%s/triage", cve_id)
            result.record("cve_triage", local_id, None, "skipped")
            continue

        resp = client.put(f"/api/v2/cve/{cve_id}/triage", body=payload)
        if resp.status_code in (200, 201):
            result.record("cve_triage", local_id, cve_id, "created")
            logger.info("  ✓ cve_triage '%s' — %s", cve_id, triage["remarks"])
        else:
            logger.warning("  ✗ cve_triage '%s' failed: HTTP %d — %s",
                           cve_id, resp.status_code, resp.text[:150])
            result.record("cve_triage", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Ranking seeders (SEED-004.6)
# ---------------------------------------------------------------------------


def seed_ranking_entries(
    client: SeedClient,
    entries: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed CPE ranking entries via POST /api/v1/ranking/bulk (SEED-004.6)."""
    logger.info("=== Seeding ranking entries (%d) ===", len(entries))
    if dry_run:
        for e in entries:
            result.record("ranking_entry", e["_id"], None, "skipped")
        return

    bulk_payload = {
        "entries": [
            {"cpe": e["cpe"], "rank": e["rank"]}
            for e in entries
        ]
    }
    resp = client.post("/api/v1/ranking/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        bulk_results = data.get("results", [])
        for i, item in enumerate(bulk_results):
            if i < len(entries):
                local_id = entries[i]["_id"]
                server_id = item.get("id", "")
                id_map.put(local_id, server_id)
                result.record("ranking_entry", local_id, server_id, item.get("status", "created"))
        logger.info("  ✓ bulk ranking: created=%d", data.get("created_count", len(bulk_results)))
    else:
        logger.warning("bulk ranking failed (HTTP %d) — fallback to individual POST", resp.status_code)
        for entry in entries:
            local_id = entry["_id"]
            resp2 = client.post("/api/v1/ranking", body={"cpe": entry["cpe"], "rank": entry["rank"]})
            if resp2.status_code in (200, 201):
                data2 = resp2.json()
                server_id = data2.get("id", "")
                id_map.put(local_id, server_id)
                result.record("ranking_entry", local_id, server_id, "created")
            else:
                logger.warning("  ✗ ranking '%s' failed: HTTP %d", entry["cpe"], resp2.status_code)
                result.record("ranking_entry", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Asset seeders (SEED-005)
# ---------------------------------------------------------------------------


def seed_assets(
    client: SeedClient,
    assets: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed assets via POST /api/v1/assets/bulk (SEED-005.2), fallback to individual."""
    logger.info("=== Seeding assets (%d) ===", len(assets))

    bulk_payload = {
        "assets": [
            {
                "ip_address": a["ip_address"],
                "hostname": a.get("hostname", ""),
                "os": a.get("os", ""),
                "mac_address": a.get("mac_address", ""),
                "services": a.get("services", []),
                "tags": a.get("tags", []),
                "labels": a.get("labels", {}),
            }
            for a in assets
        ],
        "update_existing": True,
    }

    if dry_run:
        for a in assets:
            result.record("asset", a["_id"], None, "skipped")
        return

    resp = client.post("/api/v1/assets/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        bulk_results = data.get("results", [])
        for i, item in enumerate(bulk_results):
            if i < len(assets):
                server_id = item.get("id", "")
                id_map.put(assets[i]["_id"], server_id)
                result.record("asset", assets[i]["_id"], server_id, item.get("status", "created"))
        logger.info(
            "  ✓ bulk assets: created=%d updated=%d failed=%d",
            data.get("created_count", 0), data.get("updated_count", 0), data.get("failed_count", 0),
        )
    else:
        logger.warning("bulk assets failed (HTTP %d) — falling back to individual POST", resp.status_code)
        for asset in assets:
            local_id = asset["_id"]
            payload = {
                "ip_address": asset["ip_address"],
                "hostname": asset.get("hostname", ""),
                "os": asset.get("os", ""),
                "services": asset.get("services", []),
                "tags": asset.get("tags", []),
                "labels": asset.get("labels", {}),
            }
            resp2 = client.post("/api/v1/assets", body=payload)
            if resp2.status_code in (200, 201):
                data2 = resp2.json()
                server_id = data2.get("id", "")
                id_map.put(local_id, server_id)
                result.record("asset", local_id, server_id, "created")
            else:
                logger.error("  ✗ asset %s failed: HTTP %d — %s",
                             asset["ip_address"], resp2.status_code, resp2.text[:150])
                result.record("asset", local_id, None, "failed")


def seed_asset_vulnerabilities(
    client: SeedClient,
    asset_vulns: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Inject vulnerabilities into assets via POST /api/v1/assets/{id}/vulnerabilities (SEED-005.4)."""
    logger.info("=== Seeding asset vulnerabilities (%d assets) ===", len(asset_vulns))
    for av in asset_vulns:
        local_id = av["_id"]
        asset_server_id = id_map.get(av["asset_id"])
        if not asset_server_id:
            result.record("asset_vuln", local_id, None, "skipped")
            continue

        payload = {"vulnerabilities": av["vulnerabilities"]}
        if dry_run:
            result.record("asset_vuln", local_id, None, "skipped")
            continue

        resp = client.post(f"/api/v1/assets/{asset_server_id}/vulnerabilities", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            added = data.get("added_count", len(av["vulnerabilities"]))
            result.record("asset_vuln", local_id, asset_server_id, "created")
            logger.info("  ✓ asset %s — %d vulnerabilities injected", av["asset_ip"], added)
        else:
            logger.warning("  ✗ asset_vuln for %s failed: HTTP %d — %s",
                           av["asset_ip"], resp.status_code, resp.text[:150])
            result.record("asset_vuln", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Agent seeders (SEED-005.5 / 005.6)
# ---------------------------------------------------------------------------


def seed_agents(
    client: SeedClient,
    agents: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Register agents via POST /api/v1/agents (SEED-005.5)."""
    logger.info("=== Seeding agents (%d) ===", len(agents))
    for agent in agents:
        local_id = agent["_id"]
        payload = {
            "name": agent["name"],
            "hostname": agent["hostname"],
            "ip_address": agent["ip_address"],
            "os": agent.get("os", ""),
            "tags": agent.get("tags", []),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/agents %s", agent["name"])
            result.record("agent", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v1/agents", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            api_key = data.get("api_key", "")
            id_map.put(local_id, server_id)
            result.record("agent", local_id, server_id, "created")
            logger.info("  ✓ agent '%s' → %s (api_key: %s...)", agent["name"], server_id, api_key[:12])
        elif resp.status_code == 409:
            logger.warning("  ↩ agent '%s' already exists", agent["name"])
            result.record("agent", local_id, None, "skipped")
        else:
            logger.error("  ✗ agent '%s' failed: HTTP %d — %s",
                         agent["name"], resp.status_code, resp.text[:200])
            result.record("agent", local_id, None, "failed")


def seed_agent_reports(
    client: SeedClient,
    reports: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Submit agent reports via POST /api/v1/agents/{id}/reports (SEED-005.6)."""
    logger.info("=== Seeding agent reports (%d) ===", len(reports))
    for report in reports:
        local_id = report["_id"]
        agent_server_id = id_map.get(report["agent_id"])
        if not agent_server_id or agent_server_id.startswith("dry-"):
            result.record("agent_report", local_id, None, "skipped")
            continue

        payload = {
            "hostname": report["hostname"],
            "ip_address": report["ip_address"],
            "os_info": report.get("os_info", ""),
            "kernel_version": report.get("kernel_version", ""),
            "reported_at": report.get("reported_at"),
            "packages": report.get("packages", []),
        }
        if dry_run:
            result.record("agent_report", local_id, None, "skipped")
            continue

        resp = client.post(f"/api/v1/agents/{agent_server_id}/reports", body=payload)
        if resp.status_code in (200, 201, 202):
            data = resp.json()
            report_id = data.get("report_id", "")
            pkg_count = data.get("package_count", len(report.get("packages", [])))
            id_map.put(local_id, report_id)
            result.record("agent_report", local_id, report_id, "created")
            logger.info("  ✓ agent_report for '%s' — %d packages queued", report["hostname"], pkg_count)
        else:
            logger.warning("  ✗ agent_report for '%s' failed: HTTP %d — %s",
                           report["hostname"], resp.status_code, resp.text[:150])
            result.record("agent_report", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Scan seeders (SEED-005.7)
# ---------------------------------------------------------------------------


def seed_scheduled_scans(
    client: SeedClient,
    scans: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed scheduled scans via POST /api/v1/scans/scheduled (SEED-005.7)."""
    logger.info("=== Seeding scheduled scans (%d) ===", len(scans))
    for scan in scans:
        local_id = scan["_id"]
        payload = {
            "targets": scan.get("targets", []),
            "scan_type": scan.get("scan_type", "full_scan"),
            "cron_expr": scan["cron_expr"],
            "options": scan.get("options", {}),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/scans/scheduled %s", scan["cron_expr"])
            result.record("scheduled_scan", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v1/scans/scheduled", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            id_map.put(local_id, server_id)
            result.record("scheduled_scan", local_id, server_id, "created")
            logger.info("  ✓ scheduled_scan '%s' → %s", scan.get("description", scan["cron_expr"]), server_id)
        else:
            logger.warning("  ✗ scheduled_scan failed: HTTP %d — %s",
                           resp.status_code, resp.text[:150])
            result.record("scheduled_scan", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Notification seeders (SEED-006)
# ---------------------------------------------------------------------------


def seed_notification_rules(
    client: SeedClient,
    rules: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed notification rules via bulk endpoint, fallback to individual (SEED-006.3)."""
    logger.info("=== Seeding notification rules (%d) ===", len(rules))
    if dry_run:
        for rule in rules:
            result.record("notif_rule", rule["_id"], None, "skipped")
        return

    # Try bulk first
    bulk_payload = {
        "rules": [
            {
                "product_id": id_map.get(r["product_id"]) or r["product_id"],
                "finding_added": r.get("finding_added", ["inapp"]),
                "sla_breach": r.get("sla_breach", ["email", "inapp"]),
                "sla_expiring_soon": r.get("sla_expiring_soon", ["email"]),
                "finding_status_changed": r.get("finding_status_changed", ["inapp"]),
                "risk_acceptance_expiration": r.get("risk_acceptance_expiration", []),
            }
            for r in rules
            if id_map.get(r["product_id"])
        ]
    }
    resp = client.post("/api/v2/notification-rules/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        for i, item in enumerate(data.get("results", [])):
            if i < len(rules):
                id_map.put(rules[i]["_id"], item.get("id", ""))
                result.record("notif_rule", rules[i]["_id"], item.get("id"), item.get("status", "created"))
        logger.info("  ✓ bulk notification rules: created=%d", data.get("created_count", 0))
        return

    logger.warning("bulk notification-rules failed (HTTP %d) — fallback to individual", resp.status_code)
    for rule in rules:
        local_id = rule["_id"]
        product_server_id = id_map.get(rule["product_id"])
        payload = {
            "product_id": product_server_id,
            "finding_added": rule.get("finding_added", ["inapp"]),
            "sla_breach": rule.get("sla_breach", ["email", "inapp"]),
            "sla_expiring_soon": rule.get("sla_expiring_soon", ["email"]),
            "finding_status_changed": rule.get("finding_status_changed", ["inapp"]),
        }
        resp2 = client.post("/api/v2/notification-rules", body=payload)
        if resp2.status_code in (200, 201):
            data2 = resp2.json()
            server_id = data2.get("id", "")
            id_map.put(local_id, server_id)
            result.record("notif_rule", local_id, server_id, "created")
        else:
            logger.warning("  ✗ notif_rule for product %s failed: HTTP %d",
                           product_server_id, resp2.status_code)
            result.record("notif_rule", local_id, None, "failed")


def seed_subscriptions(
    client: SeedClient,
    subs: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed subscriptions via bulk endpoint, fallback to individual (SEED-006.4)."""
    logger.info("=== Seeding subscriptions (%d) ===", len(subs))
    if dry_run:
        for sub in subs:
            result.record("subscription", sub["_id"], None, "skipped")
        return

    bulk_payload = {
        "subscriptions": [
            {
                "type": s["type"],
                "value": s.get("value", ""),
                "min_severity": s.get("min_severity", "HIGH"),
                **({"min_epss": s["min_epss"]} if s.get("min_epss") else {}),
            }
            for s in subs
        ]
    }
    resp = client.post("/api/v2/subscriptions/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        for i, item in enumerate(data.get("results", [])):
            if i < len(subs):
                id_map.put(subs[i]["_id"], item.get("id", ""))
                result.record("subscription", subs[i]["_id"], item.get("id"), item.get("status", "created"))
        logger.info("  ✓ bulk subscriptions: created=%d", data.get("created_count", 0))
        return

    logger.warning("bulk subscriptions failed (HTTP %d) — fallback to individual", resp.status_code)
    for sub in subs:
        local_id = sub["_id"]
        payload = {
            "type": sub["type"],
            "value": sub.get("value", ""),
            "min_severity": sub.get("min_severity", "HIGH"),
        }
        if sub.get("min_epss"):
            payload["min_epss"] = sub["min_epss"]
        resp2 = client.post("/api/v2/subscriptions", body=payload)
        if resp2.status_code in (200, 201):
            data2 = resp2.json()
            server_id = data2.get("id", "")
            id_map.put(local_id, server_id)
            result.record("subscription", local_id, server_id, "created")
        else:
            logger.warning("  ✗ subscription type=%s failed: HTTP %d",
                           sub["type"], resp2.status_code)
            result.record("subscription", local_id, None, "failed")


def seed_webhooks(
    client: SeedClient,
    webhooks: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed webhooks via bulk endpoint, fallback to individual (SEED-006.6)."""
    logger.info("=== Seeding webhooks (%d) ===", len(webhooks))
    if dry_run:
        for wh in webhooks:
            result.record("webhook", wh["_id"], None, "skipped")
        return

    # Try /api/v2/webhooks/bulk (SEED-006.6)
    bulk_payload = {
        "webhooks": [
            {"url": wh["url"], "events": wh.get("events", []), "description": wh.get("description", "")}
            for wh in webhooks
        ]
    }
    resp = client.post("/api/v2/webhooks/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        for i, item in enumerate(data.get("results", [])):
            if i < len(webhooks):
                id_map.put(webhooks[i]["_id"], item.get("id", ""))
                result.record("webhook", webhooks[i]["_id"], item.get("id"), item.get("status", "created"))
        logger.info("  ✓ bulk webhooks: created=%d", data.get("created_count", 0))
        return

    logger.warning("bulk webhooks failed (HTTP %d) — fallback to individual", resp.status_code)
    for wh in webhooks:
        local_id = wh["_id"]
        payload = {"url": wh["url"], "events": wh.get("events", []), "description": wh.get("description", "")}
        resp2 = client.post("/api/v1/webhooks", body=payload)
        if resp2.status_code in (200, 201):
            data2 = resp2.json()
            server_id = data2.get("id", "")
            id_map.put(local_id, server_id)
            result.record("webhook", local_id, server_id, "created")
            logger.info("  ✓ webhook '%s' → %s", wh.get("description", wh["url"][:40]), server_id)
        else:
            logger.warning("  ✗ webhook %s failed: HTTP %d — %s",
                           wh["url"][:40], resp2.status_code, resp2.text[:150])
            result.record("webhook", local_id, None, "failed")


# ---------------------------------------------------------------------------
# Config seeders (SEED-006.5 / 006.7)
# ---------------------------------------------------------------------------


def seed_jira_configurations(
    client: SeedClient,
    jira_configs: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed JIRA configs via bulk endpoint, fallback to individual (SEED-006.5)."""
    logger.info("=== Seeding JIRA configurations (%d) ===", len(jira_configs))
    if dry_run:
        for jc in jira_configs:
            result.record("jira_config", jc["_id"], None, "skipped")
        return

    # Try bulk endpoint first
    bulk_payload = {
        "configurations": [
            {
                "product_id": id_map.get(jc["product_id"]) or jc["product_id"],
                "url": jc["url"],
                "username": jc["username"],
                "api_token": jc["api_token"],
                "project_key": jc["project_key"],
                "issue_type_id": jc.get("issue_type_id", "10001"),
                "push_notes": jc.get("push_notes", True),
                "push_all_issues": jc.get("push_all_issues", False),
                "enable_deduplication": jc.get("enable_deduplication", True),
                "priority_mapping": jc.get("priority_mapping", {}),
            }
            for jc in jira_configs
            if id_map.get(jc["product_id"])
        ]
    }
    resp = client.post("/api/v2/jira-configurations/bulk", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        for i, item in enumerate(data.get("results", [])):
            if i < len(jira_configs):
                id_map.put(jira_configs[i]["_id"], item.get("id", ""))
                result.record("jira_config", jira_configs[i]["_id"], item.get("id"), "created")
        logger.info("  ✓ bulk JIRA configs: created=%d", data.get("created_count", 0))
        return

    logger.warning("bulk JIRA configs failed (HTTP %d) — fallback to individual", resp.status_code)
    for jc in jira_configs:
        local_id = jc["_id"]
        product_server_id = id_map.get(jc["product_id"])
        if not product_server_id:
            result.record("jira_config", local_id, None, "skipped")
            continue
        payload = {
            "product_id": product_server_id,
            "url": jc["url"],
            "username": jc["username"],
            "api_token": jc["api_token"],
            "project_key": jc["project_key"],
            "issue_type_id": jc.get("issue_type_id", "10001"),
            "push_notes": jc.get("push_notes", True),
            "push_all_issues": jc.get("push_all_issues", False),
            "enable_deduplication": jc.get("enable_deduplication", True),
            "priority_mapping": jc.get("priority_mapping", {}),
        }
        resp2 = client.post("/api/v2/jira-configurations", body=payload)
        if resp2.status_code in (200, 201):
            data2 = resp2.json()
            server_id = data2.get("id", "")
            id_map.put(local_id, server_id)
            result.record("jira_config", local_id, server_id, "created")
            logger.info("  ✓ jira_config for product %s", product_server_id[:8])
        else:
            logger.warning("  ✗ jira_config failed: HTTP %d — %s",
                           resp2.status_code, resp2.text[:150])
            result.record("jira_config", local_id, None, "failed")


def seed_system_notification_rules(
    client: SeedClient,
    rules: dict,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed system-wide notification rules via PUT /api/v2/system-notification-rules (SEED-006.7)."""
    logger.info("=== Seeding system notification rules ===")
    if not rules:
        return
    if dry_run:
        logger.info("[DRY-RUN] PUT /api/v2/system-notification-rules")
        result.record("system_notif_rules", "system", None, "skipped")
        return

    resp = client.put("/api/v2/system-notification-rules", body=rules)
    if resp.status_code in (200, 201):
        result.record("system_notif_rules", "system", "system", "created")
        logger.info("  ✓ system notification rules configured")
    else:
        logger.warning("  ✗ system notification rules failed: HTTP %d — %s",
                       resp.status_code, resp.text[:150])
        result.record("system_notif_rules", "system", None, "failed")


# ---------------------------------------------------------------------------
# AI Triage seeders (CR-014)
# ---------------------------------------------------------------------------


def seed_ai_triage(
    client: SeedClient,
    triage_items: list[dict],
    id_map: IdMap,
    result: SeedResult,
    dry_run: bool,
) -> None:
    """Seed AI triage queue entries (CR-014).

    Note: The backend normally creates triage entries via ai-service processing pipeline.
    For seeding purposes, we try an internal/admin bulk endpoint if available,
    otherwise fall back to the review endpoint for pre-reviewed items.
    """
    logger.info("=== Seeding AI triage queue (%d items) ===", len(triage_items))

    if dry_run:
        for item in triage_items:
            result.record("ai_triage", item["_id"], None, "skipped")
        return

    # Resolve finding IDs
    for item in triage_items:
        finding_server_id = id_map.get(item["finding_id"])
        if finding_server_id:
            item["_resolved_finding_id"] = finding_server_id

    # Try admin bulk seed endpoint
    valid_items = [i for i in triage_items if i.get("_resolved_finding_id")]
    if not valid_items:
        logger.warning("  ⚠ No triage items with resolved finding IDs — skipping")
        return

    bulk_payload = {
        "items": [
            {
                "finding_id": i["_resolved_finding_id"],
                "finding_title": i["finding_title"],
                "cve_id": i.get("cve_id"),
                "severity": i["severity"],
                "ai_result": i["ai_result"],
            }
            for i in valid_items
        ]
    }
    resp = client.post("/api/v1/ai/triage/bulk-seed", body=bulk_payload)
    if resp.status_code in (200, 201, 207):
        data = resp.json()
        created_items = data.get("results", [])
        for i, created in enumerate(created_items):
            if i < len(valid_items):
                local_id = valid_items[i]["_id"]
                server_id = created.get("id", "")
                id_map.put(local_id, server_id)
                result.record("ai_triage", local_id, server_id, "created")
        logger.info("  ✓ bulk AI triage: created=%d", data.get("created_count", len(created_items)))
    else:
        # Fallback: for items that have human decisions, POST the review
        logger.warning("bulk AI triage seed failed (HTTP %d) — only reviewed items can be seeded",
                       resp.status_code)
        for item in valid_items:
            local_id = item["_id"]
            if item.get("human_decision"):
                finding_id = item["_resolved_finding_id"]
                review_payload = {
                    "decision": item["human_decision"],
                    "note": item.get("human_note", ""),
                }
                resp2 = client.post(f"/api/v1/ai/triage/{finding_id}/review", body=review_payload)
                if resp2.status_code in (200, 201):
                    result.record("ai_triage", local_id, finding_id, "created")
                    logger.info("  ✓ ai_triage review for finding %s", finding_id[:12])
                else:
                    result.record("ai_triage", local_id, None, "failed")
            else:
                result.record("ai_triage", local_id, None, "skipped")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# NEW: Platform Settings seeder (TASK-HC-009)
# ---------------------------------------------------------------------------


def seed_platform_settings(
    client: "SeedClient",
    settings: dict,
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Seed platform settings via PUT /api/v1/admin/settings (TASK-HC-009).

    The settings dict has format {key: {value, description}}.
    The API accepts a flat JSON body: {key: value, ...}.
    """
    logger.info("=== Seeding platform_settings (%d keys) ===", len(settings))
    updates = {k: v["value"] if isinstance(v, dict) else v for k, v in settings.items()}
    payload = {"updates": updates}

    if dry_run:
        logger.info("[DRY-RUN] PUT /api/v1/admin/settings")
        result.record("platform_settings", "all", None, "skipped")
        return

    resp = client.put("/api/v1/admin/settings", body=payload)
    if resp.status_code in (200, 201, 204):
        result.record("platform_settings", "all", "settings", "created")
        logger.info("  ✓ platform_settings seeded (%d keys)", len(payload))
    else:
        logger.warning(
            "  PUT /api/v1/admin/settings failed (HTTP %d) — trying PATCH",
            resp.status_code,
        )
        resp2 = client.patch("/api/v1/admin/settings", body=payload)
        if resp2.status_code in (200, 201, 204):
            result.record("platform_settings", "all", "settings", "created")
            logger.info("  ✓ platform_settings seeded via PATCH")
        else:
            logger.error(
                "  ✗ platform_settings failed: HTTP %d — %s",
                resp2.status_code, resp2.text[:200],
            )
            result.record("platform_settings", "all", None, "failed")


# ---------------------------------------------------------------------------
# NEW: RBAC Role Metadata seeder (TASK-HC-010)
# ---------------------------------------------------------------------------


def seed_rbac_roles(
    client: "SeedClient",
    roles: list[dict],
    id_map: "IdMap",
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Seed RBAC role metadata via GET /api/v1/admin/roles (verify exists).

    The identity-service seeds system roles on startup via db migration.
    This seeder verifies the roles are present and records their server IDs
    in id_map for referential integrity in later seeders.

    If the service exposes a write endpoint, POST /api/v1/admin/roles,
    this seeder will also attempt to create custom roles.
    """
    logger.info("=== Seeding/verifying RBAC roles (%d) ===", len(roles))

    if dry_run:
        for role in roles:
            id_map.put(f"role:{role['name']}", f"dry-{role['name']}")
            result.record("rbac_role", role["name"], None, "skipped")
        return

    # Read existing roles from DB
    resp = client.get("/api/v1/admin/roles")
    existing_roles: dict[str, str] = {}  # name -> server_id
    if resp.ok:
        try:
            data = resp.json()
            # Handle both list and nested formats
            roles_list = data if isinstance(data, list) else data.get("roles", [])
            for r in roles_list:
                existing_roles[r.get("name", "")] = r.get("id", "")
            logger.info("  Found %d existing roles on server", len(existing_roles))
        except Exception as e:
            logger.warning("  Could not parse roles response: %s", e)

    for role in roles:
        name = role["name"]
        local_key = f"role:{name}"

        if name in existing_roles:
            # Role already exists (seeded by migration)
            server_id = existing_roles[name]
            id_map.put(local_key, server_id)
            result.record("rbac_role", name, server_id, "skipped")
            logger.info("  ↩ rbac_role '%s' already exists → %s", name, server_id[:8] if server_id else "?")
        else:
            # Try to create (non-system roles only, or if service supports it)
            payload = {
                "name": name,
                "display_name": role.get("display_name", name),
                "description": role.get("description", ""),
                "color": role.get("color", "#6B7280"),
                "is_system": role.get("is_system", False),
                "permissions": role.get("permissions", []),
            }
            resp2 = client.post("/api/v1/admin/roles", body=payload)
            if resp2.status_code in (200, 201):
                data2 = resp2.json()
                server_id = data2.get("id", "")
                id_map.put(local_key, server_id)
                result.record("rbac_role", name, server_id, "created")
                logger.info("  ✓ rbac_role '%s' → %s", name, server_id)
            elif resp2.status_code == 404:
                # Endpoint not implemented — roles managed by migration only
                logger.info("  ↩ rbac_role POST not supported (404) — role '%s' managed by DB migration", name)
                result.record("rbac_role", name, None, "skipped")
            else:
                logger.warning("  ✗ rbac_role '%s' failed: HTTP %d", name, resp2.status_code)
                result.record("rbac_role", name, None, "failed")


# ---------------------------------------------------------------------------
# NEW: User Invitation seeder (TASK-HC-014)
# ---------------------------------------------------------------------------


def seed_user_invitations(
    client: "SeedClient",
    invitations: list[dict],
    id_map: "IdMap",
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Seed user invitations via POST /api/v1/admin/users/invite (TASK-HC-014).

    Only sends invitations for records without accepted_at set.
    The backend creates the invitation token and sends the email —
    the seed overrides the token in the JSON for test verification.
    """
    pending = [inv for inv in invitations if not inv.get("accepted_at")]
    logger.info("=== Seeding user invitations (%d pending) ===", len(pending))

    for inv in pending:
        local_id = inv["_id"]
        payload = {
            "email": inv["email"],
            "username": inv.get("username", ""),
            "role": inv.get("role", "user"),
        }
        if dry_run:
            logger.info("[DRY-RUN] POST /api/v1/admin/users/invite %s", inv["email"])
            result.record("invitation", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        resp = client.post("/api/v1/admin/users/invite", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            token = data.get("token", "")
            id_map.put(local_id, server_id)
            id_map.put(f"invite_token:{inv['email']}", token)
            result.record("invitation", local_id, server_id, "created")
            logger.info(
                "  ✓ invitation for %s → id=%s token=%s...",
                inv["email"], server_id, token[:16] if token else "",
            )
        elif resp.status_code == 409 or "already" in resp.text.lower():
            logger.warning("  ↩ invitation for %s already exists", inv["email"])
            result.record("invitation", local_id, None, "skipped")
        else:
            logger.error(
                "  ✗ invitation for %s failed: HTTP %d — %s",
                inv["email"], resp.status_code, resp.text[:200],
            )
            result.record("invitation", local_id, None, "failed")


# ---------------------------------------------------------------------------
# NEW: JIRA Issue Mapping seeder (TASK-HC-013)
# ---------------------------------------------------------------------------


def seed_jira_issue_mappings(
    client: "SeedClient",
    mappings: list[dict],
    id_map: "IdMap",
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Seed JIRA issue mappings via POST /api/v2/jira-issues (TASK-HC-013).

    Each mapping links one finding_id to a JIRA issue (jira_key, jira_id, jira_url).
    """
    logger.info("=== Seeding JIRA issue mappings (%d) ===", len(mappings))
    for mapping in mappings:
        local_id = mapping["_id"]
        finding_server_id = id_map.get(mapping["finding_id"])
        config_server_id = id_map.get(mapping.get("jira_configuration_id", ""))

        if dry_run:
            logger.info("[DRY-RUN] POST /api/v2/jira-issues %s", mapping.get("jira_key"))
            result.record("jira_mapping", local_id, None, "skipped")
            id_map.put(local_id, f"dry-{local_id[:8]}")
            continue

        if not finding_server_id:
            logger.warning("  ⚠ jira_mapping '%s' skipped: finding not seeded", mapping.get("jira_key"))
            result.record("jira_mapping", local_id, None, "skipped")
            continue

        payload = {
            "finding_id": finding_server_id,
            "jira_configuration_id": config_server_id or None,
            "jira_id": mapping["jira_id"],
            "jira_key": mapping["jira_key"],
            "jira_url": mapping["jira_url"],
            "jira_status": mapping.get("jira_status"),
            "jira_priority": mapping.get("jira_priority"),
            "synced": mapping.get("synced", True),
        }
        resp = client.post("/api/v2/jira-issues", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            id_map.put(local_id, server_id)
            result.record("jira_mapping", local_id, server_id, "created")
            logger.info("  ✓ jira_mapping '%s' → %s", mapping["jira_key"], server_id)
        elif resp.status_code == 409:
            logger.warning("  ↩ jira_mapping '%s' already exists (finding already mapped)", mapping["jira_key"])
            result.record("jira_mapping", local_id, None, "skipped")
        elif resp.status_code == 503:
            logger.warning("  ↩ jira_mapping skipped: issueRepo not configured (503)")
            result.record("jira_mapping", local_id, None, "skipped")
        else:
            logger.error(
                "  ✗ jira_mapping '%s' failed: HTTP %d — %s",
                mapping["jira_key"], resp.status_code, resp.text[:200],
            )
            result.record("jira_mapping", local_id, None, "failed")


# ---------------------------------------------------------------------------
# NEW: Search History seeder (TASK-HC-007)
# ---------------------------------------------------------------------------


def seed_search_history(
    client: "SeedClient",
    history: list[dict],
    id_map: "IdMap",
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Seed search history via POST /api/v1/search/history (TASK-HC-007).

    Records are written to the `search_history` PostgreSQL table.
    The GET /api/v1/search/recent endpoint reads from this table.
    If the write endpoint is not exposed, we gracefully skip.
    """
    logger.info("=== Seeding search history (%d) ===", len(history))
    for entry in history:
        local_id = entry["_id"]
        user_server_id = id_map.get(entry["user_id"])

        if dry_run:
            result.record("search_history", local_id, None, "skipped")
            continue

        if not user_server_id:
            result.record("search_history", local_id, None, "skipped")
            continue

        payload = {
            "user_id": user_server_id,
            "query": entry["query"],
            "result_count": entry.get("result_count", 0),
            "search_type": entry.get("search_type", "full_text"),
            "searched_at": entry.get("searched_at"),
        }
        resp = client.post("/api/v1/search/history", body=payload)
        if resp.status_code in (200, 201):
            data = resp.json()
            server_id = data.get("id", "")
            id_map.put(local_id, server_id)
            result.record("search_history", local_id, server_id, "created")
        elif resp.status_code == 404:
            # Endpoint not exposed publicly — search history is auto-populated on real queries
            logger.info(
                "  ↩ POST /api/v1/search/history not found (404) — history populated by live queries"
            )
            result.record("search_history", local_id, None, "skipped")
            break  # Don't repeat for every entry
        else:
            logger.warning(
                "  ✗ search_history failed: HTTP %d — %s",
                resp.status_code, resp.text[:150],
            )
            result.record("search_history", local_id, None, "failed")


# ---------------------------------------------------------------------------
# NEW: AI Batch Enrich seeder (TASK-HC-012)
# ---------------------------------------------------------------------------


def seed_batch_enrich(
    client: "SeedClient",
    targets: list[dict],
    result: "SeedResult",
    dry_run: bool,
) -> None:
    """Submit batch enrichment job via POST /api/v1/ai/enrichment/batch (TASK-HC-012).

    Sends all CVE IDs in one request. Expects 202 Accepted + job_id.
    """
    logger.info("=== Seeding AI batch enrich (%d CVEs) ===", len(targets))
    cve_ids = [t["cve_id"] for t in targets]

    if dry_run:
        logger.info("[DRY-RUN] POST /api/v1/ai/enrichment/batch (%d CVE IDs)", len(cve_ids))
        result.record("batch_enrich", "batch", None, "skipped")
        return

    payload = {
        "cve_ids": cve_ids,
        "priority": "normal",
    }
    resp = client.post("/api/v1/ai/enrichment/batch", body=payload)
    if resp.status_code in (200, 201, 202):
        data = resp.json()
        job_id = data.get("job_id", data.get("id", ""))
        result.record("batch_enrich", "batch", job_id, "created")
        logger.info(
            "  ✓ batch_enrich submitted — job_id=%s (%d CVEs)",
            job_id, len(cve_ids),
        )
    elif resp.status_code == 503:
        logger.warning("  ↩ batch_enrich: AI service unavailable (503) — skipped")
        result.record("batch_enrich", "batch", None, "skipped")
    else:
        logger.error(
            "  ✗ batch_enrich failed: HTTP %d — %s",
            resp.status_code, resp.text[:200],
        )
        result.record("batch_enrich", "batch", None, "failed")


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Push seed data to OSV backend")
    p.add_argument("--env", default=None, help="Path to .env file")
    p.add_argument("--data", default=None, help="Seed data directory (overrides SEED_DATA_DIR)")
    p.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be done without sending any requests",
    )
    p.add_argument(
        "--skip",
        nargs="*",
        default=[],
        choices=[
            "users", "api_keys", "sla", "product_types", "products",
            "engagements", "tests", "sla_assignments",
            "findings", "notes", "groups",
            "cves", "cve_triages", "ranking",
            "assets", "asset_vulns", "agents", "agent_reports", "scheduled_scans",
            "notifications", "subscriptions", "webhooks",
            "jira", "system_rules",
            "ai_triage",
            # NEW — TASK-HC-007/009/010/012/013/014
            "platform_settings", "rbac_roles", "invitations",
            "jira_mappings", "search_history", "batch_enrich",
        ],
        help="Skip specific domains",
    )
    return p.parse_args()


def main() -> None:
    args = parse_args()
    cfg = SeedConfig(env_file=args.env)
    cfg.validate()
    log = cfg.setup_logging()

    data_dir: Path = Path(args.data) if args.data else cfg.seed_data_dir
    output_dir: Path = cfg.seed_output_dir
    id_map_path = output_dir / "id_map.json"
    results_path = output_dir / "push_results.json"

    output_dir.mkdir(parents=True, exist_ok=True)

    client = SeedClient(cfg)
    id_map = IdMap(id_map_path)
    result = SeedResult()
    skip = set(args.skip or [])

    if args.dry_run:
        log.info("=== DRY-RUN mode — no requests will be sent ===")

    # Health check
    if not args.dry_run:
        if not client.health_check():
            log.error("Gateway at %s is unreachable. Aborting.", cfg.gateway_url)
            sys.exit(1)
        log.info("Gateway health check OK")
        client.authenticate()

    # ---- 1. Identity ----------------------------------------------------
    if "users" not in skip:
        users = load_json(data_dir / "identity" / "users.json")
        seed_users(client, users, id_map, result, args.dry_run)
        id_map.save()

    if "api_keys" not in skip:
        api_keys = load_json(data_dir / "identity" / "api_keys.json")
        seed_api_keys(client, api_keys, id_map, result, args.dry_run)
        id_map.save()

    # ---- 2. SLA ---------------------------------------------------------
    if "sla" not in skip:
        sla_configs = load_json(data_dir / "sla" / "sla_configurations.json")
        seed_sla_configurations(client, sla_configs, id_map, result, args.dry_run)
        id_map.save()

    # ---- 3. Product hierarchy -------------------------------------------
    if "product_types" not in skip:
        product_types = load_json(data_dir / "products" / "product_types.json")
        seed_product_types(client, product_types, id_map, result, args.dry_run)
        id_map.save()

    if "products" not in skip:
        products = load_json(data_dir / "products" / "products.json")
        seed_products(client, products, id_map, result, args.dry_run)
        id_map.save()

    if "engagements" not in skip:
        engagements = load_json(data_dir / "products" / "engagements.json")
        seed_engagements(client, engagements, id_map, result, args.dry_run)
        id_map.save()

    if "tests" not in skip:
        tests = load_json(data_dir / "products" / "tests.json")
        seed_tests(client, tests, id_map, result, args.dry_run)
        id_map.save()

    # ---- 4. SLA Assignments ---------------------------------------------
    if "sla_assignments" not in skip:
        sla_assignments = load_json(data_dir / "config" / "sla_assignments.json")
        seed_sla_assignments(client, sla_assignments, id_map, result, args.dry_run)
        id_map.save()

    # ---- 5. Findings ----------------------------------------------------
    if "findings" not in skip:
        findings = load_json(data_dir / "findings" / "findings.json")
        seed_findings(client, findings, id_map, result, args.dry_run)
        id_map.save()

    if "notes" not in skip:
        notes = load_json(data_dir / "findings" / "finding_notes.json")
        seed_finding_notes(client, notes, id_map, result, args.dry_run)
        id_map.save()

    if "groups" not in skip:
        groups = load_json(data_dir / "findings" / "finding_groups.json")
        seed_finding_groups(client, groups, id_map, result, args.dry_run)
        id_map.save()

    # ---- 6. CVEs --------------------------------------------------------
    if "cves" not in skip:
        custom_cves = load_json(data_dir / "cves" / "custom_cves.json")
        seed_custom_cves(client, custom_cves, id_map, result, args.dry_run)
        id_map.save()

    if "cve_triages" not in skip:
        cve_triages = load_json(data_dir / "cves" / "cve_triages.json")
        seed_cve_triages(client, cve_triages, id_map, result, args.dry_run)
        id_map.save()

    # ---- 7. Ranking -----------------------------------------------------
    if "ranking" not in skip:
        ranking_entries = load_json(data_dir / "ranking" / "ranking_entries.json")
        seed_ranking_entries(client, ranking_entries, id_map, result, args.dry_run)
        id_map.save()

    # ---- 8. Assets ------------------------------------------------------
    if "assets" not in skip:
        assets = load_json(data_dir / "assets" / "assets.json")
        seed_assets(client, assets, id_map, result, args.dry_run)
        id_map.save()

    if "asset_vulns" not in skip:
        asset_vulns = load_json(data_dir / "assets" / "asset_vulnerabilities.json")
        seed_asset_vulnerabilities(client, asset_vulns, id_map, result, args.dry_run)
        id_map.save()

    # ---- 9. Agents ------------------------------------------------------
    if "agents" not in skip:
        agents = load_json(data_dir / "agents" / "agents.json")
        seed_agents(client, agents, id_map, result, args.dry_run)
        id_map.save()

    if "agent_reports" not in skip:
        agent_reports = load_json(data_dir / "agents" / "agent_reports.json")
        seed_agent_reports(client, agent_reports, id_map, result, args.dry_run)
        id_map.save()

    # ---- 10. Scans ------------------------------------------------------
    if "scheduled_scans" not in skip:
        scheduled_scans = load_json(data_dir / "scans" / "scheduled_scans.json")
        seed_scheduled_scans(client, scheduled_scans, id_map, result, args.dry_run)
        id_map.save()

    # ---- 11. Notifications -----------------------------------------------
    if "notifications" not in skip:
        notif_rules = load_json(data_dir / "notifications" / "notification_rules.json")
        seed_notification_rules(client, notif_rules, id_map, result, args.dry_run)
        id_map.save()

    if "subscriptions" not in skip:
        subs = load_json(data_dir / "notifications" / "subscriptions.json")
        seed_subscriptions(client, subs, id_map, result, args.dry_run)
        id_map.save()

    if "webhooks" not in skip:
        webhooks = load_json(data_dir / "notifications" / "webhooks.json")
        seed_webhooks(client, webhooks, id_map, result, args.dry_run)
        id_map.save()

    # ---- 12. Config -----------------------------------------------------
    if "jira" not in skip:
        jira_configs = load_json(data_dir / "config" / "jira_configurations.json")
        seed_jira_configurations(client, jira_configs, id_map, result, args.dry_run)
        id_map.save()

    if "system_rules" not in skip:
        system_rules = load_json_obj(data_dir / "config" / "system_notification_rules.json")
        seed_system_notification_rules(client, system_rules, result, args.dry_run)

    # ---- 13. AI Triage --------------------------------------------------
    if "ai_triage" not in skip:
        triage_items = load_json(data_dir / "ai" / "triage_queue.json")
        seed_ai_triage(client, triage_items, id_map, result, args.dry_run)
        id_map.save()

    # ---- 14. NEW: Platform Settings (TASK-HC-009) -----------------------
    if "platform_settings" not in skip:
        platform_settings = load_json_obj(data_dir / "identity" / "platform_settings.json")
        if platform_settings:
            seed_platform_settings(client, platform_settings, result, args.dry_run)

    # ---- 15. NEW: RBAC Roles (TASK-HC-010) ------------------------------
    if "rbac_roles" not in skip:
        rbac_roles = load_json(data_dir / "identity" / "rbac_roles.json")
        seed_rbac_roles(client, rbac_roles, id_map, result, args.dry_run)
        id_map.save()

    # ---- 16. NEW: User Invitations (TASK-HC-014) ------------------------
    if "invitations" not in skip:
        invitations = load_json(data_dir / "identity" / "user_invitations.json")
        seed_user_invitations(client, invitations, id_map, result, args.dry_run)
        id_map.save()

    # ---- 17. NEW: JIRA Issue Mappings (TASK-HC-013) ---------------------
    # Must run AFTER jira configs + findings are seeded
    if "jira_mappings" not in skip:
        jira_mappings = load_json(data_dir / "config" / "jira_issue_mappings.json")
        seed_jira_issue_mappings(client, jira_mappings, id_map, result, args.dry_run)
        id_map.save()

    # ---- 18. NEW: Search History (TASK-HC-007) --------------------------
    if "search_history" not in skip:
        search_hist = load_json(data_dir / "search" / "search_history.json")
        seed_search_history(client, search_hist, id_map, result, args.dry_run)
        id_map.save()

    # ---- 19. NEW: AI Batch Enrich (TASK-HC-012) -------------------------
    if "batch_enrich" not in skip:
        batch_targets = load_json(data_dir / "ai" / "batch_enrich_targets.json")
        seed_batch_enrich(client, batch_targets, result, args.dry_run)

    # ---- Final report ---------------------------------------------------
    all_results = {
        "summary": result.summary(),
        "success": result.success,
        "failed": result.failed,
        "skipped": result.skipped,
    }
    results_path.write_text(
        json.dumps(all_results, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    log.info("=== Push complete: %s ===", result.summary())
    log.info("  ID map  → %s", id_map_path)
    log.info("  Results → %s", results_path)

    if result.failed:
        log.warning("%d failures — review %s for details", len(result.failed), results_path)
        sys.exit(2)


if __name__ == "__main__":
    main()
