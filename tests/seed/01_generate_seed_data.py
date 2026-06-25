#!/usr/bin/env python3
"""
01_generate_seed_data.py — Generate seed data files based on domain models.

Generates realistic JSON seed-data files for all services:
  - identity/  : users.json, api_keys.json, platform_settings.json,
                 rbac_roles.json, user_invitations.json
  - products/  : product_types.json, products.json, engagements.json, tests.json
  - findings/  : findings.json, finding_notes.json, finding_groups.json
  - cves/      : custom_cves.json, cve_triages.json
  - sla/       : sla_configurations.json
  - config/    : sla_assignments.json, jira_configurations.json,
                 jira_issue_mappings.json,
                 system_notification_rules.json
  - notifications/ : notification_rules.json, subscriptions.json, webhooks.json
  - assets/    : assets.json, asset_vulnerabilities.json
  - agents/    : agents.json, agent_reports.json
  - scans/     : scheduled_scans.json
  - ranking/   : ranking_entries.json
  - ai/        : triage_queue.json, batch_enrich_targets.json
  - search/    : search_history.json

Usage:
    python 01_generate_seed_data.py [--env .env] [--out ./data] [--count N]
"""

from __future__ import annotations

import argparse
import json
import logging
import random
import sys
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

# ---------------------------------------------------------------------------
# Allow running from any CWD by adding parent to sys.path
# ---------------------------------------------------------------------------
sys.path.insert(0, str(Path(__file__).parent))
from seed_config import SeedConfig  # noqa: E402

logger = logging.getLogger("seed.generate")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def uid() -> str:
    """Return a new UUIDv4 string."""
    return str(uuid.uuid4())


def now_iso(offset_days: int = 0) -> str:
    dt = datetime.now(timezone.utc) + timedelta(days=offset_days)
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")


def pick(*choices: str) -> str:
    return random.choice(choices)


def save(path: Path, data: list | dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")
    logger.info("Saved %s (%d items)", path, len(data) if isinstance(data, list) else 1)


# ---------------------------------------------------------------------------
# Generator functions
# ---------------------------------------------------------------------------


def gen_users(n: int = 10) -> list[dict]:
    """Generate n user records (SEED-001 model, CR-011 fields)."""
    roles = ["admin", "user", "user", "user", "readonly"]
    users = [
        {
            "_id": uid(),
            "email": "admin@company.com",
            "username": "admin",
            "password": "AdminPass123!",
            "role": "admin",
            "is_active": True,
            "is_verified": True,
            "login_attempts": 0,
            "is_locked": False,
        }
    ]
    first_names = [
        "Alice", "Bob", "Carol", "Dave", "Eve",
        "Frank", "Grace", "Hank", "Iris", "Jack",
        "Karen", "Leo", "Mia", "Nick", "Olivia",
    ]
    for i in range(1, n):
        name = first_names[i % len(first_names)]
        users.append(
            {
                "_id": uid(),
                "email": f"{name.lower()}{i}@company.com",
                "username": f"{name.lower()}{i}",
                "password": f"Pass{i}Secure!",
                "role": pick(*roles),
                "is_active": True,
                "is_verified": True,
                "login_attempts": 0,
                "is_locked": False,
            }
        )
    return users


def gen_api_keys(users: list[dict]) -> list[dict]:
    """Generate API keys for admin and service-account users (SEED-001)."""
    keys = []
    # Admin gets a full-access key
    keys.append(
        {
            "_id": uid(),
            "user_id": users[0]["_id"],
            "name": "Admin Master Key",
            "scopes": ["admin:*"],
            "expires_at": None,
        }
    )
    # CI/CD service account key
    keys.append(
        {
            "_id": uid(),
            "user_id": users[0]["_id"],
            "name": "CI/CD Pipeline",
            "scopes": ["cve:read", "finding:write", "scan:execute"],
            "expires_at": now_iso(365),
        }
    )
    # Read-only analytics key
    keys.append(
        {
            "_id": uid(),
            "user_id": users[0]["_id"],
            "name": "Analytics Dashboard",
            "scopes": ["cve:read", "finding:read", "asset:read"],
            "expires_at": now_iso(180),
        }
    )
    return keys


def gen_product_types(n: int = 5) -> list[dict]:
    """Generate ProductType records (SEED-002)."""
    names = [
        ("Web Application", "Web-based applications exposed to the internet"),
        ("Mobile App", "iOS and Android mobile applications"),
        ("API Service", "Backend REST/gRPC microservices"),
        ("Infrastructure", "Network devices, VMs, Kubernetes clusters"),
        ("Desktop App", "Native desktop applications"),
        ("IoT Device", "Embedded and IoT devices"),
        ("CI/CD Pipeline", "Automation and pipeline tooling"),
    ]
    result = []
    for i in range(min(n, len(names))):
        name, desc = names[i]
        result.append(
            {
                "_id": uid(),
                "name": name,
                "description": desc,
                "critical_product": i < 2,
                "key_product": i < 4,
            }
        )
    return result


def gen_products(product_types: list[dict], n: int = 8) -> list[dict]:
    """Generate Product records (SEED-002)."""
    criticalities = ["very high", "high", "medium", "low"]
    platforms = ["web", "api", "mobile", "desktop"]
    lifecycles = ["production", "production", "construction", "retirement"]
    origins = ["internal", "contractor", "outsourced", "open source", "purchased"]
    samples = [
        ("Customer Portal", "B2C customer-facing web portal", True, True),
        ("Internal HR System", "Internal HR management system", False, False),
        ("Payment Gateway API", "Core payment processing microservice", True, True),
        ("Mobile Banking App", "iOS/Android banking application", True, True),
        ("Admin Dashboard", "Internal admin tooling", False, False),
        ("Inventory Service", "Warehouse inventory management API", False, False),
        ("Analytics Platform", "Data analytics and reporting pipeline", False, True),
        ("VPN Gateway", "Corporate VPN infrastructure", False, True),
        ("Legacy ERP", "On-premise ERP system (SAP)", False, False),
        ("DevOps Toolchain", "Jenkins, GitLab CI, ArgoCD", False, False),
    ]
    result = []
    for i in range(min(n, len(samples))):
        name, desc, ext, internet = samples[i]
        pt = product_types[i % len(product_types)]
        result.append(
            {
                "_id": uid(),
                "product_type_id": pt["_id"],
                "product_type_name": pt["name"],  # used by import endpoint
                "name": name,
                "description": desc,
                "business_criticality": pick(*criticalities),
                "platform": pick(*platforms),
                "lifecycle": pick(*lifecycles),
                "origin": pick(*origins),
                "external_audience": ext,
                "internet_accessible": internet,
                "enable_full_risk_acceptance": False,
                "enable_simple_risk_acceptance": True,
                "tags": random.sample(
                    ["production", "critical", "web-tier", "api", "external", "internal", "legacy"],
                    k=random.randint(1, 3),
                ),
            }
        )
    return result


def gen_engagements(products: list[dict], users: list[dict], n_per_product: int = 2) -> list[dict]:
    """Generate Engagement records (SEED-002)."""
    types = ["Interactive", "CI/CD"]
    statuses = ["Not Started", "In Progress", "Completed"]
    result = []
    for product in products:
        for j in range(n_per_product):
            lead = random.choice(users)
            start = now_iso(-random.randint(30, 180))
            result.append(
                {
                    "_id": uid(),
                    "product_id": product["_id"],
                    "name": f"Security Assessment Q{(j % 4) + 1} 2026",
                    "description": f"Quarterly security assessment for {product['name']}",
                    "lead_id": lead["_id"],
                    "engagement_type": pick(*types),
                    "status": pick(*statuses),
                    "start_date": start,
                    "end_date": now_iso(random.randint(-7, 30)),
                    "version": f"v{random.randint(1, 5)}.{random.randint(0, 9)}.0",
                    "tags": ["quarterly", "manual"] if j == 0 else ["ci-cd", "automated"],
                    "deduplication_on_engagement": True,
                }
            )
    return result


def gen_tests(engagements: list[dict], n_per_engagement: int = 2) -> list[dict]:
    """Generate Test records (SEED-002)."""
    scan_types = ["Trivy Scan", "Bandit Scan", "SARIF", "Manual Pentest", "DAST Scan"]
    result = []
    for eng in engagements:
        for j in range(n_per_engagement):
            scan_type = scan_types[j % len(scan_types)]
            result.append(
                {
                    "_id": uid(),
                    "engagement_id": eng["_id"],
                    "scan_type": scan_type,
                    "title": f"{scan_type} — {now_iso()[:10]}",
                    "description": f"Automated {scan_type} run",
                    "target_start": eng["start_date"],
                    "target_end": now_iso(14),
                    "percent_complete": random.randint(60, 100),
                    "tags": ["automated"],
                }
            )
    return result


def gen_findings(
    tests: list[dict],
    engagements: list[dict],
    products: list[dict],
    users: list[dict],
    n_per_test: int = 5,
) -> list[dict]:
    """Generate Finding records (SEED-003, finding-service model)."""
    severities = ["Critical", "High", "High", "Medium", "Medium", "Medium", "Low", "Info"]
    cve_samples = [
        ("CVE-2021-44228", "Critical", 10.0, 79),   # Log4Shell
        ("CVE-2022-22965", "Critical", 9.8, 94),    # Spring4Shell
        ("CVE-2023-44487", "High", 7.5, 400),       # HTTP/2 Rapid Reset
        ("CVE-2022-42889", "Critical", 9.8, 78),    # Text4Shell
        ("CVE-2023-34362", "Critical", 9.8, 89),    # MOVEit
        ("CVE-2021-26084", "Critical", 9.8, 74),    # Confluence OGNL
        ("CVE-2022-1388", "Critical", 9.8, 290),    # F5 BIG-IP
        ("CVE-2023-20198", "Critical", 10.0, 78),   # Cisco IOS XE
        ("", "High", 7.5, 79),
        ("", "Medium", 5.0, 601),
        ("", "Low", 2.5, 200),
    ]
    components = [
        ("log4j-core", "2.14.1"),
        ("spring-webmvc", "5.3.18"),
        ("nginx", "1.20.0"),
        ("openssl", "1.1.1k"),
        ("apache-struts", "2.5.28"),
        ("commons-text", "1.9"),
        ("jquery", "3.5.1"),
        ("jackson-databind", "2.13.0"),
    ]

    # Build lookup maps
    eng_map = {e["_id"]: e for e in engagements}
    test_eng_map = {t["_id"]: eng_map[t["engagement_id"]] for t in tests}
    prod_map = {p["_id"]: p for p in products}

    result = []
    for test in tests:
        eng = test_eng_map[test["_id"]]
        prod = prod_map[eng["product_id"]]
        creator = random.choice(users)
        for _ in range(n_per_test):
            cve_info = random.choice(cve_samples)
            cve_id, sev, cvss, cwe = cve_info
            comp, comp_ver = random.choice(components)
            assignee = random.choice(users)
            fid = uid()
            result.append(
                {
                    "_id": fid,
                    "title": (
                        f"{comp} — {sev} vulnerability"
                        if not cve_id
                        else f"{cve_id} in {comp}"
                    ),
                    "description": (
                        f"A {sev.lower()} severity vulnerability was detected in {comp} "
                        f"version {comp_ver}. Immediate remediation is recommended."
                    ),
                    "mitigation": "Update to the latest patched version. Apply vendor advisory.",
                    "severity": sev,
                    "cve": cve_id or None,
                    "cwe": cwe,
                    "cvss_v3_score": cvss,
                    "component_name": comp,
                    "component_version": comp_ver,
                    "date": now_iso(-random.randint(1, 90)),
                    "active": random.random() > 0.2,
                    "verified": random.random() > 0.5,
                    "false_positive": False,
                    "duplicate": False,
                    "out_of_scope": False,
                    "is_mitigated": False,
                    "risk_accepted": False,
                    "is_kev": cve_id in (
                        "CVE-2021-44228", "CVE-2022-22965", "CVE-2023-20198",
                        "CVE-2023-34362",
                    ),
                    "assigned_to": assignee["email"],
                    "created_by": creator["email"],
                    "test_id": test["_id"],
                    "engagement_id": eng["_id"],
                    "product_id": prod["_id"],
                    "tags": random.sample(
                        ["webapp", "injection", "xss", "rce", "authn", "crypto"],
                        k=random.randint(1, 2),
                    ),
                }
            )
    return result


def gen_finding_notes(findings: list[dict], users: list[dict]) -> list[dict]:
    """Generate FindingNote records for a random subset of findings."""
    notes = []
    for finding in random.sample(findings, k=min(len(findings), 20)):
        author = random.choice(users)
        notes.append(
            {
                "_id": uid(),
                "finding_id": finding["_id"],
                "author_id": author["_id"],
                "content": f"Triaged during Q2 review. Component {finding['component_name']} confirmed.",
                "is_private": False,
            }
        )
    return notes


def gen_finding_groups(findings: list[dict], products: list[dict]) -> list[dict]:
    """Generate FindingGroup records grouping findings by CVE."""
    groups = []
    log4shell_ids = [
        f["_id"] for f in findings if "CVE-2021-44228" in (f.get("cve") or "")
    ]
    if log4shell_ids and products:
        groups.append(
            {
                "_id": uid(),
                "name": "Log4Shell variants",
                "product_id": products[0]["_id"],
                "finding_ids": log4shell_ids[:5],
            }
        )
    spring4shell_ids = [
        f["_id"] for f in findings if "CVE-2022-22965" in (f.get("cve") or "")
    ]
    if spring4shell_ids and len(products) > 1:
        groups.append(
            {
                "_id": uid(),
                "name": "Spring4Shell cluster",
                "product_id": products[1]["_id"],
                "finding_ids": spring4shell_ids[:3],
            }
        )
    return groups


# ---------------------------------------------------------------------------
# CVE generators (SEED-004)
# ---------------------------------------------------------------------------


def gen_custom_cves(n: int = 5) -> list[dict]:
    """Generate custom/internal CVE records (SEED-004.1)."""
    internal_cves = [
        {
            "_id": uid(),
            "id": "CVE-INTERNAL-2026-001",
            "summary": "Internal authentication bypass in legacy API",
            "description": (
                "Legacy API endpoint /api/legacy/auth bypasses token validation "
                "when X-Internal header is set. Affects versions prior to 2.1.0."
            ),
            "severity": "critical",
            "cvss3": 9.1,
            "cvss3_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:N",
            "published": now_iso(-30),
            "source": "INTERNAL",
            "vendors": ["company-internal"],
            "products": ["legacy-api"],
            "references": ["https://internal.company.com/security/2026-001"],
            "is_kev": False,
            "is_exploit": False,
        },
        {
            "_id": uid(),
            "id": "CVE-INTERNAL-2026-002",
            "summary": "SQL injection in report generation endpoint",
            "description": (
                "Report generation endpoint /api/reports/custom accepts unsanitized "
                "user input leading to SQL injection. Affected all versions before 3.5.2."
            ),
            "severity": "high",
            "cvss3": 8.8,
            "cvss3_vector": "CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H",
            "published": now_iso(-14),
            "source": "INTERNAL",
            "vendors": ["company-internal"],
            "products": ["report-service"],
            "references": ["https://internal.company.com/security/2026-002"],
            "is_kev": False,
            "is_exploit": True,
        },
        {
            "_id": uid(),
            "id": "CVE-INTERNAL-2026-003",
            "summary": "SSRF vulnerability in webhook delivery",
            "description": (
                "Webhook delivery service accepts arbitrary URLs including internal network addresses, "
                "enabling SSRF attacks against internal services."
            ),
            "severity": "high",
            "cvss3": 7.7,
            "cvss3_vector": "CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:C/C:H/I:N/A:N",
            "published": now_iso(-7),
            "source": "INTERNAL",
            "vendors": ["company-internal"],
            "products": ["notification-service"],
            "references": ["https://internal.company.com/security/2026-003"],
            "is_kev": False,
            "is_exploit": False,
        },
    ]
    return internal_cves[:min(n, len(internal_cves))]


def gen_cve_triages(custom_cves: list[dict], users: list[dict]) -> list[dict]:
    """Generate triage decisions for well-known public CVEs (SEED-004.2)."""
    public_triages = [
        {
            "_id": uid(),
            "cve_id": "CVE-2021-44228",
            "remarks": "Confirmed",
            "comments": (
                "Confirmed affected — log4j-core 2.14.1 in use across 3 services. "
                "Emergency patch deployed on 2021-12-13."
            ),
            "justification": "vulnerable_code_in_execute_path",
            "response": ["workaround_available", "update"],
            "triaged_by": users[0]["email"] if users else "admin@company.com",
        },
        {
            "_id": uid(),
            "cve_id": "CVE-2022-22965",
            "remarks": "Mitigated",
            "comments": (
                "Spring Boot upgraded to 2.7.4. JVM patched to > 9. "
                "No additional compensating controls required."
            ),
            "justification": "vulnerable_code_not_in_execute_path",
            "response": ["update"],
            "triaged_by": users[0]["email"] if users else "admin@company.com",
        },
        {
            "_id": uid(),
            "cve_id": "CVE-2023-44487",
            "remarks": "NotAffected",
            "comments": (
                "HTTP/2 rapid reset — our nginx version 1.25.3 includes the patch. "
                "Infrastructure team confirmed rate limiting is also in place."
            ),
            "justification": "inline_mitigations_already_exist",
            "response": ["will_not_fix"],
            "triaged_by": users[0]["email"] if users else "admin@company.com",
        },
        {
            "_id": uid(),
            "cve_id": "CVE-2022-42889",
            "remarks": "Confirmed",
            "comments": "commons-text 1.9 confirmed in report-service classpath. Upgrade to 1.10.0 scheduled.",
            "justification": "vulnerable_code_in_execute_path",
            "response": ["update"],
            "triaged_by": users[0]["email"] if users else "admin@company.com",
        },
    ]
    return public_triages


# ---------------------------------------------------------------------------
# Asset & Agent generators (SEED-005)
# ---------------------------------------------------------------------------


def gen_assets(n: int = 10) -> list[dict]:
    """Generate Asset records (SEED-005).

    Supports generating 1000+ diverse assets across multiple subnets,
    tiers, environments, OSes, and service profiles.
    """
    oses = [
        "Ubuntu 22.04 LTS",
        "Ubuntu 20.04 LTS",
        "CentOS 8",
        "CentOS 7",
        "Rocky Linux 9",
        "AlmaLinux 8",
        "Windows Server 2022",
        "Windows Server 2019",
        "Debian 11",
        "Debian 12",
        "Alpine Linux 3.18",
        "Red Hat Enterprise Linux 9",
        "Oracle Linux 8",
        "Amazon Linux 2",
        "SUSE Linux Enterprise 15",
    ]

    # Extended tier definitions covering multiple environments
    tiers = [
        ("web",      "web-tier",     "dmz",      "production"),
        ("db",       "database",     "critical",  "production"),
        ("api",      "api-tier",     "internal",  "production"),
        ("ci",       "ci-cd",        "internal",  "devops"),
        ("gw",       "gateway",      "core",      "production"),
        ("cache",    "cache-tier",   "internal",  "production"),
        ("mq",       "messaging",    "internal",  "production"),
        ("mon",      "monitoring",   "ops",       "ops"),
        ("k8s",      "kubernetes",   "orchestration", "production"),
        ("mail",     "mail-server",  "dmz",       "production"),
        ("vpn",      "vpn",          "core",      "production"),
        ("nfs",      "storage",      "critical",  "production"),
        ("ldap",     "directory",    "critical",  "production"),
        ("proxy",    "reverse-proxy","dmz",       "production"),
        ("backup",   "backup",       "ops",       "ops"),
        ("dev",      "dev",          "internal",  "development"),
        ("staging",  "staging",      "internal",  "staging"),
        ("bastion",  "bastion",      "dmz",       "production"),
        ("waf",      "waf",          "dmz",       "production"),
        ("siem",     "siem",         "ops",       "security"),
    ]

    # Service profiles per tier
    _services_map: dict[str, list[dict]] = {
        "web": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 80, "protocol": "tcp", "name": "http", "product": "nginx", "version": "1.24.0"},
            {"port": 443, "protocol": "tcp", "name": "https", "product": "nginx", "version": "1.24.0"},
        ],
        "db": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 5432, "protocol": "tcp", "name": "postgresql", "product": "postgresql", "version": "15.3"},
        ],
        "api": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 8080, "protocol": "tcp", "name": "http-alt", "product": "go-httpd", "version": "1.21"},
            {"port": 8443, "protocol": "tcp", "name": "https-alt", "product": "go-httpd", "version": "1.21"},
        ],
        "ci": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 8080, "protocol": "tcp", "name": "jenkins", "product": "jenkins", "version": "2.401"},
        ],
        "gw": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 80, "protocol": "tcp", "name": "http", "product": "envoy", "version": "1.27"},
            {"port": 443, "protocol": "tcp", "name": "https", "product": "envoy", "version": "1.27"},
            {"port": 9901, "protocol": "tcp", "name": "admin", "product": "envoy", "version": "1.27"},
        ],
        "cache": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 6379, "protocol": "tcp", "name": "redis", "product": "redis", "version": "7.2"},
        ],
        "mq": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 5672, "protocol": "tcp", "name": "amqp", "product": "rabbitmq", "version": "3.12"},
            {"port": 15672, "protocol": "tcp", "name": "rabbitmq-mgmt", "product": "rabbitmq", "version": "3.12"},
        ],
        "mon": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 9090, "protocol": "tcp", "name": "prometheus", "product": "prometheus", "version": "2.46"},
            {"port": 3000, "protocol": "tcp", "name": "grafana", "product": "grafana", "version": "10.1"},
        ],
        "k8s": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 6443, "protocol": "tcp", "name": "kubernetes-api", "product": "kubernetes", "version": "1.28"},
            {"port": 10250, "protocol": "tcp", "name": "kubelet", "product": "kubernetes", "version": "1.28"},
        ],
        "mail": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 25, "protocol": "tcp", "name": "smtp", "product": "postfix", "version": "3.7"},
            {"port": 993, "protocol": "tcp", "name": "imaps", "product": "dovecot", "version": "2.3"},
        ],
        "vpn": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 1194, "protocol": "udp", "name": "openvpn", "product": "openvpn", "version": "2.6"},
        ],
        "nfs": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 2049, "protocol": "tcp", "name": "nfs", "product": "nfs-kernel-server", "version": "2.6"},
        ],
        "ldap": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 389, "protocol": "tcp", "name": "ldap", "product": "openldap", "version": "2.6"},
            {"port": 636, "protocol": "tcp", "name": "ldaps", "product": "openldap", "version": "2.6"},
        ],
        "proxy": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 80, "protocol": "tcp", "name": "http", "product": "haproxy", "version": "2.8"},
            {"port": 443, "protocol": "tcp", "name": "https", "product": "haproxy", "version": "2.8"},
        ],
        "backup": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 9102, "protocol": "tcp", "name": "bacula-fd", "product": "bacula", "version": "13.0"},
        ],
        "dev": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 3000, "protocol": "tcp", "name": "node-dev", "product": "node", "version": "20.0"},
        ],
        "staging": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 80, "protocol": "tcp", "name": "http", "product": "nginx", "version": "1.24.0"},
            {"port": 443, "protocol": "tcp", "name": "https", "product": "nginx", "version": "1.24.0"},
        ],
        "bastion": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
        ],
        "waf": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 80, "protocol": "tcp", "name": "http", "product": "modsecurity", "version": "3.0"},
            {"port": 443, "protocol": "tcp", "name": "https", "product": "modsecurity", "version": "3.0"},
        ],
        "siem": [
            {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"},
            {"port": 5601, "protocol": "tcp", "name": "kibana", "product": "kibana", "version": "8.10"},
            {"port": 9200, "protocol": "tcp", "name": "elasticsearch", "product": "elasticsearch", "version": "8.10"},
        ],
    }
    _default_services = [
        {"port": 22, "protocol": "tcp", "name": "ssh", "product": "openssh", "version": "8.9"}
    ]

    assets = []
    for i in range(n):
        tier_name, tag1, tag2, env = tiers[i % len(tiers)]
        # Distribute across multiple /16 subnets to support > 65k unique IPs
        subnet_b = (i // 65025) + 10   # 10.x, 11.x, 12.x …
        subnet_c = (i // 255) % 255
        subnet_d = (i % 255) + 1
        ip = f"{subnet_b}.{subnet_c}.{subnet_d}.{random.randint(1, 254)}"
        # Ensure hostname uniqueness using index
        seq = i + 1
        services = _services_map.get(tier_name, _default_services)
        assets.append(
            {
                "_id": uid(),
                "ip_address": f"{subnet_b}.{subnet_c}.{subnet_d}.{seq % 254 + 1}",
                "hostname": f"{tier_name}-{seq:04d}.{env}.internal",
                "os": random.choice(oses),
                "mac_address": ":".join(
                    [f"{random.randint(0, 255):02X}" for _ in range(6)]
                ),
                "services": services,
                "tags": [tag1, tag2, env, "production" if env == "production" else env],
                "labels": {
                    "environment": env,
                    "tier": tier_name,
                    "criticality": "high" if tag2 in ("critical", "core", "dmz") else "medium",
                    "index": str(seq),
                },
            }
        )
    return assets


def gen_asset_vulnerabilities(assets: list[dict]) -> list[dict]:
    """Generate vulnerability injection payloads per asset (SEED-005.4)."""
    known_vulns = [
        ("CVE-2021-44228", "critical", 10.0),
        ("CVE-2022-22965", "critical", 9.8),
        ("CVE-2023-44487", "high", 7.5),
        ("CVE-2022-42889", "critical", 9.8),
        ("CVE-2021-41773", "critical", 9.8),
        ("CVE-2022-1388", "critical", 9.8),
        ("CVE-2021-34527", "critical", 8.8),
        ("CVE-2023-20198", "critical", 10.0),
    ]
    result = []
    # Inject vulnerabilities into a subset of assets
    for asset in random.sample(assets, k=min(len(assets), 6)):
        n_vulns = random.randint(1, 4)
        vulns = random.sample(known_vulns, k=n_vulns)
        result.append(
            {
                "_id": uid(),
                "asset_id": asset["_id"],
                "asset_ip": asset["ip_address"],
                "vulnerabilities": [
                    {
                        "cve_id": cve_id,
                        "severity": severity,
                        "cvss": cvss,
                        "detected_at": now_iso(-random.randint(1, 30)),
                    }
                    for cve_id, severity, cvss in vulns
                ],
            }
        )
    return result


def gen_agents(assets: list[dict], n: int = 5) -> list[dict]:
    """Generate Agent records (SEED-005.5)."""
    result = []
    for i, asset in enumerate(assets[:min(n, len(assets))]):
        result.append(
            {
                "_id": uid(),
                "name": f"Agent-Prod-{asset['hostname'].split('.')[0].upper()}",
                "hostname": asset["hostname"],
                "ip_address": asset["ip_address"],
                "os": asset.get("os", "Ubuntu 22.04 LTS"),
                "tags": asset.get("tags", ["production"]),
                "asset_id": asset["_id"],  # link to asset
            }
        )
    return result


def gen_agent_reports(agents: list[dict]) -> list[dict]:
    """Generate agent report payloads with package data (SEED-005.6)."""
    ecosystems = ["debian", "ubuntu", "rpm", "maven", "npm", "pypi", "go"]
    pkg_pool = [
        ("openssl", "3.0.2-0ubuntu1", "debian"),
        ("log4j", "2.14.1", "maven"),
        ("spring-core", "5.3.18", "maven"),
        ("apache-commons-text", "1.9", "maven"),
        ("nginx", "1.24.0", "debian"),
        ("nodejs", "18.12.0", "rpm"),
        ("python3", "3.10.6", "ubuntu"),
        ("curl", "7.68.0-1ubuntu2.18", "debian"),
        ("libssl1.1", "1.1.1f-1ubuntu2", "debian"),
        ("jackson-databind", "2.13.0", "maven"),
        ("requests", "2.27.1", "pypi"),
        ("lodash", "4.17.20", "npm"),
        ("express", "4.17.1", "npm"),
    ]
    result = []
    for agent in agents:
        n_packages = random.randint(8, 20)
        packages = []
        for pkg_name, pkg_ver, ecosystem in random.sample(pkg_pool, k=min(n_packages, len(pkg_pool))):
            packages.append(
                {
                    "name": pkg_name,
                    "version": pkg_ver,
                    "ecosystem": ecosystem,
                    "architecture": pick("amd64", "x86_64", "noarch"),
                }
            )
        result.append(
            {
                "_id": uid(),
                "agent_id": agent["_id"],
                "hostname": agent["hostname"],
                "ip_address": agent["ip_address"],
                "os_info": agent.get("os", "Ubuntu 22.04.3 LTS"),
                "kernel_version": pick(
                    "5.15.0-91-generic",
                    "5.14.0-362.18.1.el9_3.x86_64",
                    "10.0.20348.2340",
                ),
                "reported_at": now_iso(-random.randint(0, 3)),
                "packages": packages,
            }
        )
    return result


def gen_scheduled_scans(n: int = 4) -> list[dict]:
    """Generate ScheduledScan records (SEED-005.7)."""
    cron_exprs = [
        ("0 2 * * *", "Daily at 2am"),
        ("0 0 * * 0", "Weekly on Sunday midnight"),
        ("0 */6 * * *", "Every 6 hours"),
        ("0 3 1 * *", "Monthly on 1st at 3am"),
    ]
    scan_types = ["full_scan", "incremental_scan", "targeted_scan"]
    result = []
    for i in range(min(n, len(cron_exprs))):
        cron, cron_desc = cron_exprs[i]
        result.append(
            {
                "_id": uid(),
                "targets": [f"10.0.{i}.0/24"],
                "scan_type": pick(*scan_types),
                "cron_expr": cron,
                "description": cron_desc,
                "options": {
                    "ports": "1-1024",
                    "timeout": 3600,
                    "intensity": random.randint(1, 5),
                },
                "is_active": True,
            }
        )
    return result


# ---------------------------------------------------------------------------
# SLA generators (SEED-006)
# ---------------------------------------------------------------------------


def gen_sla_configurations() -> list[dict]:
    """Generate SLAConfiguration records (SEED-006)."""
    return [
        {
            "_id": uid(),
            "name": "Standard SLA",
            "description": "Default SLA policy for most products",
            "critical_days": 7,
            "high_days": 30,
            "medium_days": 90,
            "low_days": 365,
            "is_default": True,
        },
        {
            "_id": uid(),
            "name": "Critical Assets SLA",
            "description": "Stricter SLA for business-critical / internet-facing systems",
            "critical_days": 3,
            "high_days": 14,
            "medium_days": 60,
            "low_days": 180,
            "is_default": False,
        },
        {
            "_id": uid(),
            "name": "Relaxed SLA",
            "description": "Relaxed SLA for internal / low-risk systems",
            "critical_days": 14,
            "high_days": 60,
            "medium_days": 180,
            "low_days": 365,
            "is_default": False,
        },
    ]


def gen_sla_assignments(products: list[dict], sla_configs: list[dict]) -> list[dict]:
    """Generate SLA-to-product assignment records (SEED-006.2)."""
    # Default SLA for most, stricter for internet-accessible products
    default_sla = sla_configs[0]["_id"]
    critical_sla = sla_configs[1]["_id"] if len(sla_configs) > 1 else default_sla
    assignments = []
    for prod in products:
        sla_id = critical_sla if prod.get("internet_accessible") else default_sla
        assignments.append(
            {
                "_id": uid(),
                "product_id": prod["_id"],
                "sla_configuration_id": sla_id,
            }
        )
    return assignments


# ---------------------------------------------------------------------------
# Notification generators (SEED-006)
# ---------------------------------------------------------------------------


def gen_notification_rules(products: list[dict]) -> list[dict]:
    """Generate NotificationRule records per product (SEED-006)."""
    rules = []
    for i, product in enumerate(products):
        rules.append(
            {
                "_id": uid(),
                "product_id": product["_id"],
                "finding_added": ["email", "inapp"] if i < 3 else ["inapp"],
                "sla_breach": ["email", "inapp"],
                "sla_expiring_soon": ["email"],
                "finding_status_changed": ["inapp"],
                "risk_acceptance_expiration": ["email"] if i < 2 else [],
            }
        )
    return rules


def gen_subscriptions(users: list[dict]) -> list[dict]:
    """Generate alert subscription records (SEED-006)."""
    return [
        {"_id": uid(), "type": "vendor", "value": "apache", "min_severity": "HIGH"},
        {"_id": uid(), "type": "vendor", "value": "microsoft", "min_severity": "CRITICAL"},
        {"_id": uid(), "type": "vendor", "value": "oracle", "min_severity": "CRITICAL"},
        {"_id": uid(), "type": "vendor", "value": "cisco", "min_severity": "HIGH"},
        {"_id": uid(), "type": "product", "value": "log4j", "min_severity": "HIGH", "min_epss": 0.5},
        {"_id": uid(), "type": "product", "value": "spring", "min_severity": "HIGH"},
        {"_id": uid(), "type": "product", "value": "openssl", "min_severity": "HIGH"},
        {"_id": uid(), "type": "kev", "value": "", "min_severity": "MEDIUM"},
    ]


def gen_webhooks() -> list[dict]:
    """Generate Webhook records (SEED-006)."""
    return [
        {
            "_id": uid(),
            "url": "https://hooks.slack.com/services/T00/B00/placeholder",
            "events": ["kev.new", "cve.new.critical"],
            "description": "Slack alerts for new KEV and critical CVEs",
        },
        {
            "_id": uid(),
            "url": "https://ci.company.internal/hooks/security",
            "events": ["cve.epss.high", "cve.vendor"],
            "description": "CI pipeline security gate trigger",
        },
        {
            "_id": uid(),
            "url": "https://teams.microsoft.com/webhooks/placeholder",
            "events": ["sla.breach", "finding.critical.new"],
            "description": "Microsoft Teams channel for SLA breaches",
        },
    ]


def gen_system_notification_rules() -> dict:
    """Generate system-wide notification rule config (SEED-006.7)."""
    return {
        "scan_added": ["email", "inapp"],
        "finding_added": ["inapp"],
        "sla_breach": ["email", "inapp"],
        "sla_expiring_soon": ["email", "inapp"],
        "risk_acceptance_expiration": ["email"],
        "product_added": ["inapp"],
    }


def gen_jira_configurations(products: list[dict]) -> list[dict]:
    """Generate JIRA configuration records for critical products (SEED-006.5)."""
    jira_configs = []
    # Only configure JIRA for internet-accessible or critical products
    jira_products = [p for p in products if p.get("internet_accessible") or p.get("external_audience")][:3]
    for i, prod in enumerate(jira_products):
        jira_configs.append(
            {
                "_id": uid(),
                "product_id": prod["_id"],
                "url": "https://company.atlassian.net",
                "username": "jira-security-bot@company.com",
                "api_token": f"JIRA_API_TOKEN_PLACEHOLDER_{i + 1}",  # will be encrypted by backend
                "project_key": pick("SEC", "VULN", "SECSCAN"),
                "issue_type_id": pick("10001", "10002", "10003"),
                "push_notes": True,
                "push_all_issues": False,
                "enable_deduplication": True,
                "priority_mapping": {
                    "Critical": "Highest",
                    "High": "High",
                    "Medium": "Medium",
                    "Low": "Low",
                    "Info": "Lowest",
                },
            }
        )
    return jira_configs


# ---------------------------------------------------------------------------
# Ranking generators (SEED-004.6)
# ---------------------------------------------------------------------------


def gen_ranking_entries() -> list[dict]:
    """Generate CPE ranking entries for organization-specific priority (SEED-004.6)."""
    return [
        {
            "_id": uid(),
            "cpe": "apache:log4j",
            "rank": [
                {"group": "it", "rank": 10},
                {"group": "engineering", "rank": 9},
                {"group": "finance", "rank": 3},
            ],
        },
        {
            "_id": uid(),
            "cpe": "oracle:database",
            "rank": [
                {"group": "it", "rank": 8},
                {"group": "finance", "rank": 10},
                {"group": "engineering", "rank": 5},
            ],
        },
        {
            "_id": uid(),
            "cpe": "microsoft:windows_server",
            "rank": [
                {"group": "it", "rank": 9},
                {"group": "engineering", "rank": 6},
                {"group": "operations", "rank": 10},
            ],
        },
        {
            "_id": uid(),
            "cpe": "nginx:nginx",
            "rank": [
                {"group": "it", "rank": 7},
                {"group": "engineering", "rank": 8},
                {"group": "operations", "rank": 7},
            ],
        },
        {
            "_id": uid(),
            "cpe": "openssl:openssl",
            "rank": [
                {"group": "it", "rank": 9},
                {"group": "engineering", "rank": 9},
                {"group": "security", "rank": 10},
            ],
        },
        {
            "_id": uid(),
            "cpe": "cisco:ios",
            "rank": [
                {"group": "it", "rank": 10},
                {"group": "network-ops", "rank": 10},
                {"group": "security", "rank": 9},
            ],
        },
        {
            "_id": uid(),
            "cpe": "vmware:vcenter_server",
            "rank": [
                {"group": "it", "rank": 9},
                {"group": "operations", "rank": 9},
            ],
        },
    ]


# ---------------------------------------------------------------------------
# AI Triage generators (CR-014)
# ---------------------------------------------------------------------------


def gen_ai_triage_queue(findings: list[dict], users: list[dict]) -> list[dict]:
    """Generate AI triage queue entries (CR-014) for a subset of findings."""
    remarks_pool = [
        ("Confirmed", 0.85, 0.99),
        ("FalsePositive", 0.70, 0.92),
        ("NotAffected", 0.75, 0.95),
        ("Unexplored", 0.50, 0.75),
    ]
    human_decisions = [None, None, None, "accepted", "overridden", "rejected"]

    # Pick high/critical findings for triage
    critical_findings = [
        f for f in findings if f.get("severity") in ("Critical", "High")
    ]
    sample_size = min(len(critical_findings), 15)
    sampled = random.sample(critical_findings, k=sample_size) if critical_findings else []

    result = []
    reviewer = users[0]["email"] if users else "admin@company.com"

    for i, finding in enumerate(sampled):
        remarks, conf_min, conf_max = random.choice(remarks_pool)
        confidence = round(random.uniform(conf_min, conf_max), 4)
        human_decision = random.choice(human_decisions)
        reviewed_by = reviewer if human_decision else None
        reviewed_at = now_iso(-random.randint(0, 2)) if human_decision else None

        result.append(
            {
                "_id": uid(),
                "finding_id": finding["_id"],
                "finding_title": finding.get("title", ""),
                "cve_id": finding.get("cve"),
                "severity": finding.get("severity", "Medium"),
                "ai_result": {
                    "remarks": remarks,
                    "confidence": confidence,
                    "justification": _gen_triage_justification(finding, remarks),
                    "actions": _gen_triage_actions(finding, remarks),
                    "generated_at": now_iso(-random.randint(0, 5)),
                },
                "human_decision": human_decision,
                "human_note": (
                    f"Reviewed during security sprint — {remarks.lower()} confirmed."
                    if human_decision else None
                ),
                "reviewed_by": reviewed_by,
                "reviewed_at": reviewed_at,
            }
        )
    return result


def _gen_triage_justification(finding: dict, remarks: str) -> str:
    comp = finding.get("component_name", "component")
    ver = finding.get("component_version", "unknown")
    cve = finding.get("cve") or "this vulnerability"
    severity = finding.get("severity", "Medium")
    if remarks == "Confirmed":
        return (
            f"CVSS {finding.get('cvss_v3_score', '?')} ({severity}). "
            f"{comp} {ver} is in the affected version range for {cve}. "
            "No compensating controls detected in the current configuration."
        )
    elif remarks == "FalsePositive":
        return (
            f"{comp} {ver} appears in the scan, but static analysis confirms "
            "the vulnerable code path is not reachable in this deployment context."
        )
    elif remarks == "NotAffected":
        return (
            f"Infrastructure analysis confirms {comp} is not deployed in a configuration "
            f"affected by {cve}. Compensating controls are in place."
        )
    else:
        return (
            f"{cve} requires deeper investigation. Automated analysis inconclusive — "
            "manual review recommended."
        )


def _gen_triage_actions(finding: dict, remarks: str) -> list[str]:
    comp = finding.get("component_name", "component")
    if remarks == "Confirmed":
        return [
            f"Upgrade {comp} to the latest patched version immediately.",
            "Apply vendor security advisory mitigations.",
            "Monitor for exploitation attempts in SIEM.",
        ]
    elif remarks == "FalsePositive":
        return [
            "Mark as false positive in finding tracker.",
            "Document justification for audit trail.",
        ]
    elif remarks == "NotAffected":
        return [
            "Document not-affected status with justification.",
            "Review compensating controls quarterly.",
        ]
    else:
        return [
            "Assign to security team for manual review.",
            "Gather additional context from asset owner.",
        ]


# ---------------------------------------------------------------------------
# Platform Settings generators (TASK-HC-009)
# ---------------------------------------------------------------------------


def gen_platform_settings() -> dict:
    """Generate platform_settings seed (TASK-HC-009 — replaces hardcode).

    Returns a dict of key -> {value, description} as stored in the
    `platform_settings` PostgreSQL table.
    """
    return {
        "max_scan_concurrent": {
            "value": 5,
            "description": "Maximum number of concurrent scan jobs",
        },
        "default_scan_timeout": {
            "value": 3600,
            "description": "Default scan timeout in seconds",
        },
        "ai_enrichment_enabled": {
            "value": True,
            "description": "Enable automatic AI enrichment for new CVEs",
        },
        "ai_enrichment_batch_size": {
            "value": 20,
            "description": "Number of CVEs per AI batch-enrich job",
        },
        "ai_enrichment_concurrency": {
            "value": 5,
            "description": "Max concurrent goroutines in batch_enrich",
        },
        "smtp_from_address": {
            "value": "security-noreply@company.com",
            "description": "From address for system emails (invitations, alerts)",
        },
        "smtp_from_name": {
            "value": "OSV Security Platform",
            "description": "Display name for system emails",
        },
        "invitation_expiry_hours": {
            "value": 48,
            "description": "User invitation token expiry in hours",
        },
        "session_max_devices": {
            "value": 5,
            "description": "Maximum concurrent sessions per user",
        },
        "finding_report_date_mode": {
            "value": "dynamic",
            "description": "How report dates are computed: dynamic (now) | fixed | creation_date",
        },
        "sla_warning_days": {
            "value": 3,
            "description": "Days before SLA breach to send warning notification",
        },
        "rate_limit_login_per_minute": {
            "value": 10,
            "description": "Max login attempts per IP per minute",
        },
        "cve_fetch_sources": {
            "value": ["NVD", "OSV", "EPSS"],
            "description": "Default CVE fetch sources for data-service PopulateDB",
        },
        "jira_sync_interval_minutes": {
            "value": 30,
            "description": "Interval for bidirectional Jira sync",
        },
        "scan_scheduled_enabled": {
            "value": True,
            "description": "Enable/disable scheduled scan execution",
        },
    }


# ---------------------------------------------------------------------------
# RBAC Role Metadata generators (TASK-HC-010)
# ---------------------------------------------------------------------------


def gen_rbac_roles() -> list[dict]:
    """Generate rbac_roles seed data (TASK-HC-010).

    4 system roles that replace the static maps in GetRBACMatrix handler.
    """
    return [
        {
            "name": "admin",
            "display_name": "Administrator",
            "description": "Full system access — manage users, settings, all products",
            "color": "#8B5CF6",
            "is_system": True,
            "permissions": [
                "product:view", "product:add", "product:edit", "product:delete",
                "finding:view", "finding:add", "finding:edit", "finding:close", "finding:delete",
                "engagement:view", "engagement:add", "engagement:edit",
                "risk_acceptance:view", "risk_acceptance:manage",
                "user:view", "user:add", "user:edit",
                "system:configure",
                "report:download",
                "scan:execute", "scan:manage",
                "ai:enrich", "ai:triage",
                "import:scan_result",
            ],
        },
        {
            "name": "user",
            "display_name": "Security Analyst",
            "description": "Standard analyst — view/add/edit findings and engagements, import scans",
            "color": "#3B82F6",
            "is_system": True,
            "permissions": [
                "product:view",
                "finding:view", "finding:add", "finding:edit", "finding:close",
                "engagement:view", "engagement:add", "engagement:edit",
                "report:download",
                "scan:execute",
                "ai:enrich",
                "import:scan_result",
            ],
        },
        {
            "name": "readonly",
            "display_name": "Read-Only Viewer",
            "description": "Read-only access to products, findings and reports — no write operations",
            "color": "#6B7280",
            "is_system": True,
            "permissions": [
                "product:view",
                "finding:view",
                "engagement:view",
                "report:download",
            ],
        },
        {
            "name": "agent",
            "display_name": "Scan Agent",
            "description": "Machine identity for scan agents — submit reports, import scan results",
            "color": "#F59E0B",
            "is_system": True,
            "permissions": [
                "scan:execute",
                "import:scan_result",
                "finding:add",
            ],
        },
    ]


def gen_rbac_permission_categories() -> list[dict]:
    """Generate rbac_permission_categories seed data (TASK-HC-010)."""
    return [
        {
            "category": "Dashboard",
            "sort_order": 1,
            "permissions": ["product:view"],
        },
        {
            "category": "Scanning",
            "sort_order": 2,
            "permissions": ["scan:execute", "scan:manage", "import:scan_result"],
        },
        {
            "category": "Findings",
            "sort_order": 3,
            "permissions": [
                "finding:view", "finding:add", "finding:edit",
                "finding:close", "finding:delete",
                "risk_acceptance:view", "risk_acceptance:manage",
            ],
        },
        {
            "category": "Reports",
            "sort_order": 4,
            "permissions": ["report:download"],
        },
        {
            "category": "AI Center",
            "sort_order": 5,
            "permissions": ["ai:enrich", "ai:triage"],
        },
        {
            "category": "Administration",
            "sort_order": 6,
            "permissions": [
                "user:view", "user:add", "user:edit",
                "product:add", "product:edit", "product:delete",
                "system:configure",
            ],
        },
        {
            "category": "Agent",
            "sort_order": 7,
            "permissions": ["scan:execute", "import:scan_result", "finding:add"],
        },
    ]


# ---------------------------------------------------------------------------
# User Invitation generators (TASK-HC-014)
# ---------------------------------------------------------------------------


def gen_user_invitations(users: list[dict]) -> list[dict]:
    """Generate UserInvitation seed records (TASK-HC-014).

    Simulates pending and accepted invitations.
    NOTE: In real usage, invitations are created via
    POST /api/v1/admin/users/invite — this seed data is used
    to pre-populate the DB or verify the accept-invite flow.
    """
    import secrets
    invitations = []

    # Pending invitations (not yet accepted)
    pending_emails = [
        ("new.analyst@company.com", "new_analyst", "user"),
        ("contractor.sec@partner.org", "contractor_sec", "user"),
        ("readonly.viewer@company.com", "readonly_viewer", "readonly"),
    ]
    admin_id = users[0]["_id"] if users else uid()

    for email, username, role in pending_emails:
        token = secrets.token_hex(32)
        invitations.append(
            {
                "_id": uid(),
                "email": email,
                "username": username,
                "role": role,
                "token": token,
                "invited_by_id": admin_id,
                "expires_at": now_iso(48 // 24 + 2),  # 2 days from now
                "accepted_at": None,
                "_comment": "pending — not yet accepted",
            }
        )

    # Accepted invitation (historical, for verify step)
    accepted_token = secrets.token_hex(32)
    invitations.append(
        {
            "_id": uid(),
            "email": "accepted.user@company.com",
            "username": "accepted_user",
            "role": "user",
            "token": accepted_token,
            "invited_by_id": admin_id,
            "expires_at": now_iso(2),
            "accepted_at": now_iso(-1),  # accepted yesterday
            "_comment": "already accepted — user is active",
        }
    )

    return invitations


# ---------------------------------------------------------------------------
# JIRA Issue Mapping generators (TASK-HC-013)
# ---------------------------------------------------------------------------


def gen_jira_issue_mappings(
    findings: list[dict],
    jira_configs: list[dict],
    n: int = 6,
) -> list[dict]:
    """Generate JiraIssueMapping seed records (TASK-HC-013).

    Links a subset of high/critical findings to mock JIRA issues.
    In production, these are created via POST /api/v2/jira-issues.
    """
    # Target high/critical findings for JIRA
    critical_findings = [
        f for f in findings
        if f.get("severity") in ("Critical", "High") and f.get("cve")
    ]
    sample_size = min(n, len(critical_findings))
    sampled = random.sample(critical_findings, k=sample_size) if critical_findings else []

    jira_statuses = ["To Do", "In Progress", "Done", "In Review"]
    jira_priorities = ["Highest", "High", "Medium"]
    project_keys = [c.get("project_key", "SEC") for c in jira_configs] or ["SEC"]

    mappings = []
    for i, finding in enumerate(sampled):
        project_key = project_keys[i % len(project_keys)]
        jira_issue_num = 100 + i + 1
        jira_key = f"{project_key}-{jira_issue_num}"
        jira_id = str(10000 + i)
        config = jira_configs[i % len(jira_configs)] if jira_configs else {}
        config_id = config.get("_id") if config else None

        mappings.append(
            {
                "_id": uid(),
                "finding_id": finding["_id"],
                "jira_configuration_id": config_id,
                "jira_id": jira_id,
                "jira_key": jira_key,
                "jira_url": f"https://company.atlassian.net/browse/{jira_key}",
                "jira_status": random.choice(jira_statuses),
                "jira_priority": random.choice(jira_priorities),
                "synced": True,
                "last_sync_at": now_iso(-random.randint(0, 7)),
                "sync_error": None,
            }
        )
    return mappings


# ---------------------------------------------------------------------------
# Search History generators (TASK-HC-007)
# ---------------------------------------------------------------------------


def gen_search_history(users: list[dict], n: int = 20) -> list[dict]:
    """Generate search_history seed records (TASK-HC-007).

    Simulates realistic CVE search queries saved in PostgreSQL
    `search_history` table. These are returned by
    GET /api/v1/search/recent and GET /api/v1/search/suggested.
    """
    query_templates = [
        # Vendor/product queries
        "log4j CVE critical",
        "spring rce 2022",
        "nginx 1.20 vulnerability",
        "openssl 3.0 critical",
        "apache struts rce",
        "jackson-databind deserialization",
        "cisco ios xss",
        "microsoft windows zero day",
        "oracle database privilege escalation",
        "vmware vcenter authentication bypass",
        # Specific CVEs
        "CVE-2021-44228",
        "CVE-2022-22965",
        "CVE-2023-44487",
        "CVE-2023-20198",
        "CVE-2022-42889",
        # Search terms
        "CVSS >= 9.0 remote code execution",
        "KEV 2024 high",
        "EPSS > 0.9 unpatched",
        "buffer overflow kernel",
        "sql injection authentication",
        "memory corruption heap spray",
        "supply chain npm package",
        "container escape kubernetes",
        "ransomware exploit kit 2025",
        "zero day browser chrome",
    ]

    history = []
    for i in range(min(n, len(query_templates))):
        user = random.choice(users)
        query = query_templates[i]
        history.append(
            {
                "_id": uid(),
                "user_id": user["_id"],
                "query": query,
                "result_count": random.randint(0, 250),
                "search_type": random.choice(["full_text", "semantic", "full_text"]),
                "searched_at": now_iso(-random.randint(0, 30)),
            }
        )
    return history


# ---------------------------------------------------------------------------
# AI Batch Enrich targets (TASK-HC-012)
# ---------------------------------------------------------------------------


def gen_batch_enrich_targets() -> list[dict]:
    """Generate batch enrichment target list (TASK-HC-012).

    CVE IDs to submit to POST /api/v1/ai/enrichment/batch.
    These are high-priority CVEs that should be enriched on seed.
    """
    return [
        {
            "cve_id": "CVE-2021-44228",
            "priority": "critical",
            "reason": "Log4Shell — most widely exploited CVE",
        },
        {
            "cve_id": "CVE-2022-22965",
            "priority": "critical",
            "reason": "Spring4Shell — RCE in Spring Framework",
        },
        {
            "cve_id": "CVE-2023-44487",
            "priority": "high",
            "reason": "HTTP/2 Rapid Reset — DDoS at protocol level",
        },
        {
            "cve_id": "CVE-2022-42889",
            "priority": "critical",
            "reason": "Text4Shell — RCE in Apache Commons Text",
        },
        {
            "cve_id": "CVE-2023-34362",
            "priority": "critical",
            "reason": "MOVEit Transfer — actively exploited",
        },
        {
            "cve_id": "CVE-2023-20198",
            "priority": "critical",
            "reason": "Cisco IOS XE — privilege escalation",
        },
        {
            "cve_id": "CVE-2024-21762",
            "priority": "critical",
            "reason": "Fortinet SSL VPN out-of-bounds write",
        },
        {
            "cve_id": "CVE-2024-3400",
            "priority": "critical",
            "reason": "Palo Alto PAN-OS command injection",
        },
    ]



def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Generate OSV seed data files")
    p.add_argument("--env", default=None, help="Path to .env file")
    p.add_argument("--out", default=None, help="Output directory (overrides SEED_DATA_DIR)")
    p.add_argument(
        "--count",
        type=int,
        default=None,
        help="Base count for generated items (scales all generators)",
    )
    p.add_argument(
        "--assets-count",
        type=int,
        default=None,
        help="Number of assets to generate (default: 1200, overrides --count for assets)",
    )
    return p.parse_args()


def main() -> None:
    args = parse_args()
    cfg = SeedConfig(env_file=args.env)
    log = cfg.setup_logging()

    out_dir: Path = Path(args.out) if args.out else cfg.seed_data_dir
    out_dir.mkdir(parents=True, exist_ok=True)

    count = args.count or 10
    # Assets use a separate, higher default to satisfy the 1000+ requirement
    assets_count = args.assets_count if args.assets_count is not None else max(count, 1200)
    log.info(
        "=== Generating seed data → %s (base count: %d, assets: %d) ===",
        out_dir, count, assets_count,
    )

    # ---- Identity -------------------------------------------------------
    users = gen_users(n=count)
    save(out_dir / "identity" / "users.json", users)

    api_keys = gen_api_keys(users)
    save(out_dir / "identity" / "api_keys.json", api_keys)

    # ---- SLA ------------------------------------------------------------
    sla_configs = gen_sla_configurations()
    save(out_dir / "sla" / "sla_configurations.json", sla_configs)

    # ---- Products -------------------------------------------------------
    product_types = gen_product_types(n=min(count // 2, 7))
    save(out_dir / "products" / "product_types.json", product_types)

    products = gen_products(product_types, n=min(count, 10))
    save(out_dir / "products" / "products.json", products)

    engagements = gen_engagements(products, users, n_per_product=2)
    save(out_dir / "products" / "engagements.json", engagements)

    tests = gen_tests(engagements, n_per_engagement=2)
    save(out_dir / "products" / "tests.json", tests)

    # ---- SLA Assignments ------------------------------------------------
    sla_assignments = gen_sla_assignments(products, sla_configs)
    save(out_dir / "config" / "sla_assignments.json", sla_assignments)

    # ---- Findings -------------------------------------------------------
    findings = gen_findings(tests, engagements, products, users, n_per_test=max(count // 4, 3))
    save(out_dir / "findings" / "findings.json", findings)

    notes = gen_finding_notes(findings, users)
    save(out_dir / "findings" / "finding_notes.json", notes)

    groups = gen_finding_groups(findings, products)
    save(out_dir / "findings" / "finding_groups.json", groups)

    # ---- CVEs -----------------------------------------------------------
    custom_cves = gen_custom_cves(n=min(count // 3, 5))
    save(out_dir / "cves" / "custom_cves.json", custom_cves)

    cve_triages = gen_cve_triages(custom_cves, users)
    save(out_dir / "cves" / "cve_triages.json", cve_triages)

    # ---- Ranking --------------------------------------------------------
    ranking_entries = gen_ranking_entries()
    save(out_dir / "ranking" / "ranking_entries.json", ranking_entries)

    # ---- Assets ---------------------------------------------------------
    assets = gen_assets(n=assets_count)
    save(out_dir / "assets" / "assets.json", assets)

    asset_vulns = gen_asset_vulnerabilities(assets)
    save(out_dir / "assets" / "asset_vulnerabilities.json", asset_vulns)

    # ---- Agents ---------------------------------------------------------
    agents = gen_agents(assets, n=min(count // 2, 5))
    save(out_dir / "agents" / "agents.json", agents)

    agent_reports = gen_agent_reports(agents)
    save(out_dir / "agents" / "agent_reports.json", agent_reports)

    # ---- Scheduled Scans ------------------------------------------------
    scheduled_scans = gen_scheduled_scans(n=min(count // 3, 4))
    save(out_dir / "scans" / "scheduled_scans.json", scheduled_scans)

    # ---- Notifications --------------------------------------------------
    notif_rules = gen_notification_rules(products)
    save(out_dir / "notifications" / "notification_rules.json", notif_rules)

    subs = gen_subscriptions(users)
    save(out_dir / "notifications" / "subscriptions.json", subs)

    webhooks = gen_webhooks()
    save(out_dir / "notifications" / "webhooks.json", webhooks)

    # ---- Config (JIRA, System Rules) ------------------------------------
    jira_configs = gen_jira_configurations(products)
    save(out_dir / "config" / "jira_configurations.json", jira_configs)

    system_rules = gen_system_notification_rules()
    save(out_dir / "config" / "system_notification_rules.json", system_rules)

    # ---- AI Triage Queue ------------------------------------------------
    ai_triage = gen_ai_triage_queue(findings, users)
    save(out_dir / "ai" / "triage_queue.json", ai_triage)

    # ---- NEW: Platform Settings (TASK-HC-009) ---------------------------
    platform_settings = gen_platform_settings()
    save(out_dir / "identity" / "platform_settings.json", platform_settings)

    # ---- NEW: RBAC Roles + Permission Categories (TASK-HC-010) ----------
    rbac_roles = gen_rbac_roles()
    save(out_dir / "identity" / "rbac_roles.json", rbac_roles)

    rbac_categories = gen_rbac_permission_categories()
    save(out_dir / "identity" / "rbac_permission_categories.json", rbac_categories)

    # ---- NEW: User Invitations (TASK-HC-014) ----------------------------
    invitations = gen_user_invitations(users)
    save(out_dir / "identity" / "user_invitations.json", invitations)

    # ---- NEW: JIRA Issue Mappings (TASK-HC-013) -------------------------
    jira_mappings = gen_jira_issue_mappings(findings, jira_configs, n=min(count // 2, 8))
    save(out_dir / "config" / "jira_issue_mappings.json", jira_mappings)

    # ---- NEW: Search History (TASK-HC-007) ------------------------------
    search_hist = gen_search_history(users, n=min(count * 2, 25))
    save(out_dir / "search" / "search_history.json", search_hist)

    # ---- NEW: AI Batch Enrich Targets (TASK-HC-012) ---------------------
    batch_targets = gen_batch_enrich_targets()
    save(out_dir / "ai" / "batch_enrich_targets.json", batch_targets)

    # ---- Summary --------------------------------------------------------
    log.info("=== Generation complete ===")
    log.info("  users                : %d", len(users))
    log.info("  api_keys             : %d", len(api_keys))
    log.info("  platform_settings    : %d keys", len(platform_settings))
    log.info("  rbac_roles           : %d", len(rbac_roles))
    log.info("  rbac_categories      : %d", len(rbac_categories))
    log.info("  user_invitations     : %d", len(invitations))
    log.info("  product_types        : %d", len(product_types))
    log.info("  products             : %d", len(products))
    log.info("  engagements          : %d", len(engagements))
    log.info("  tests                : %d", len(tests))
    log.info("  findings             : %d", len(findings))
    log.info("  finding_notes        : %d", len(notes))
    log.info("  finding_groups       : %d", len(groups))
    log.info("  sla_configs          : %d", len(sla_configs))
    log.info("  sla_assignments      : %d", len(sla_assignments))
    log.info("  custom_cves          : %d", len(custom_cves))
    log.info("  cve_triages          : %d", len(cve_triages))
    log.info("  ranking_entries      : %d", len(ranking_entries))
    log.info("  assets               : %d (target: %d)", len(assets), assets_count)
    log.info("  asset_vulnerabilities: %d", len(asset_vulns))
    log.info("  agents               : %d", len(agents))
    log.info("  agent_reports        : %d", len(agent_reports))
    log.info("  scheduled_scans      : %d", len(scheduled_scans))
    log.info("  notif_rules          : %d", len(notif_rules))
    log.info("  subscriptions        : %d", len(subs))
    log.info("  webhooks             : %d", len(webhooks))
    log.info("  jira_configs         : %d", len(jira_configs))
    log.info("  jira_issue_mappings  : %d", len(jira_mappings))
    log.info("  search_history       : %d", len(search_hist))
    log.info("  ai_triage_queue      : %d", len(ai_triage))
    log.info("  batch_enrich_targets : %d", len(batch_targets))


if __name__ == "__main__":
    main()
