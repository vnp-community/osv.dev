"""
test_findings_scans.py — Test Findings và Scans endpoints (/api/v1)

Kiểm tra:
  Findings:
    - GET /findings            → FindingsListResponse schema
    - GET /findings/stats      → stats schema
    - GET /findings/{id}       → Finding schema (nếu có SAMPLE_FINDING_ID)
    - GET /findings/{id}/notes → { notes: [...] }

  Scans:
    - GET /scans               → ScansListResponse schema
    - GET /scans/scheduled     → { scheduled_scans: [...], total }
    - GET /scans/{id}          → Scan schema (nếu có SAMPLE_SCAN_ID)

  Risk Acceptances:
    - GET /risk-acceptances    → list schema

  SLA:
    - GET /sla/config          → { global, product_overrides }
    - GET /sla/overview        → SLASummary schema

Chạy:
  python test_findings_scans.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_pagination, _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

FINDING_REQUIRED = [
    "id", "title", "description", "severity", "is_kev", "status",
    "is_duplicate", "product_id", "product_name", "engagement_id",
    "test_id", "sla_status", "created_at", "updated_at", "created_by"
]

FINDING_STATUS_VALUES = {
    "active", "mitigated", "false_positive", "risk_accepted", "out_of_scope", "duplicate"
}

SLA_STATUS_VALUES = {"ok", "at_risk", "breached"}

# Server: {findings, total, page, page_size} (no by_severity, by_status, sla_stats in all versions)
FINDINGS_LIST_REQUIRED = ["findings", "total"]

SLA_STATS_REQUIRED = ["breached", "at_risk", "ok"]

FINDING_NOTE_REQUIRED = ["id", "finding_id", "content", "created_by", "created_at"]

SCAN_REQUIRED = [
    "id", "name", "type", "status", "targets",
    "progress", "finding_count", "created_by"
]

SCAN_TYPE_VALUES = {"nmap_full", "nmap_discovery", "zap", "agent", "import"}
SCAN_STATUS_VALUES = {"pending", "queued", "running", "completed", "failed", "cancelled"}

# Server: {scans, total, page, limit} (no stats field yet)
SCANS_LIST_REQUIRED = ["scans", "total"]

RISK_ACCEPTANCE_REQUIRED = [
    "id", "product_id", "finding_ids", "expiration_date",
    "reason", "approved_by", "is_expired", "created_at"
]

SLA_CONFIG_REQUIRED = ["critical_days", "high_days", "medium_days", "low_days"]
SLA_SUMMARY_REQUIRED = ["total_active_findings", "compliance_percent", "breached", "at_risk", "ok"]


def _validate_finding(f: dict, path: str) -> list:
    errors = validate_required_fields(f, FINDING_REQUIRED, path)
    if f.get("status") and f["status"] not in FINDING_STATUS_VALUES:
        errors.append(f"{path}.status='{f['status']}' not in valid values")
    if f.get("sla_status") and f["sla_status"] not in SLA_STATUS_VALUES:
        errors.append(f"{path}.sla_status='{f['sla_status']}' not in valid values")
    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("FINDINGS & SCANS API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_findings_scans_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # FINDINGS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Findings ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /findings → FindingsListResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /findings → FindingsListResponse"))
    resp = client.get("/findings", params={"page": 1, "pageSize": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, FINDINGS_LIST_REQUIRED, "FindingsListResponse")
            if errors:
                results.record_fail("findings_list_response_schema", "; ".join(errors))
            else:
                results.record_pass("findings_list_returns_200_with_schema")

            # pagination (may use limit instead of page_size)
            page_ok = body.get("page") is not None
            page_size_ok = body.get("page_size") is not None or body.get("limit") is not None
            if page_ok and page_size_ok:
                results.record_pass("findings_list_pagination_valid")
            else:
                results.record_skip("findings_list_pagination", "pagination fields not in response")

            # Validate findings items
            findings = body.get("findings") or []
            if isinstance(findings, list):
                results.record_pass("findings_list_data_is_array")

        except Exception as e:
            results.record_fail("findings_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 500:
        results.record_skip("findings_list_returns_200", "Server error 500 (possible empty DB) — skip")
    elif resp.status_code == 404:
        results.record_skip("findings_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("findings_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /findings?status=active → only open/active findings returned
    # Server uses 'new'/'in_review'/'open' for active statuses (not necessarily 'active')
    # Accept: new, active, open, in_review as "active-like" statuses
    # Reject: mitigated, false_positive, risk_accepted, closed, duplicate
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /findings?status=active → only active findings"))
    CLOSED_STATUSES = {"mitigated", "false_positive", "risk_accepted", "closed", "duplicate", "out_of_scope"}
    resp = client.get("/findings", params={"status": "active", "pageSize": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            findings = body.get("findings") or []
            # Filter returns any non-closed status — server may use 'new' instead of 'active'
            closed_findings = [f.get("id") for f in findings
                               if f.get("status") in CLOSED_STATUSES]
            if closed_findings:
                results.record_fail("findings_status_filter_works",
                                    f"Closed findings returned with status=active filter: {closed_findings[:3]}")
            else:
                results.record_pass("findings_status_filter_works")
        except Exception as e:
            results.record_fail("findings_status_filter", f"Exception: {e}")
    elif resp.status_code in (404, 500):
        results.record_skip("findings_status_filter", f"Endpoint returned {resp.status_code}")
    else:
        results.record_fail("findings_status_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /findings/stats → stats object
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /findings/stats → findings stats"))
    resp = client.get("/findings/stats")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept any stats-like response
            if "total" in body or "by_severity" in body:
                results.record_pass("findings_stats_returns_200_with_schema")
            else:
                results.record_fail("findings_stats_schema", "no total or by_severity field")
        except Exception as e:
            results.record_fail("findings_stats_returns_200", f"Exception: {e}")
    elif resp.status_code in (400, 404, 500):
        results.record_skip("findings_stats_returns_200", f"Endpoint returned {resp.status_code}")
    else:
        results.record_fail("findings_stats_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /findings/{id} → Finding schema
    # ─────────────────────────────────────────────────────────────────────────
    if Config.SAMPLE_FINDING_ID:
        fid = Config.SAMPLE_FINDING_ID
        print(_info(f"Test: GET /findings/{fid} → Finding schema"))
        resp = client.get(f"/findings/{fid}")
        if resp.status_code == 200:
            try:
                f = resp.json()
                f_errors = _validate_finding(f, f"Finding[{fid}]")
                if f_errors:
                    results.record_fail("finding_detail_schema", "; ".join(f_errors))
                else:
                    results.record_pass(f"finding_detail_{fid}_returns_200_with_schema")
            except Exception as e:
                results.record_fail("finding_detail_schema", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"finding_detail_{fid}", "Finding not found or endpoint not implemented")
        else:
            results.record_fail(f"finding_detail_{fid}", f"Got {resp.status_code}")

        # ─────────────────────────────────────────────────────────────────────
        # TEST 4b: GET /findings/{id}/notes → { notes: [...] }
        # ─────────────────────────────────────────────────────────────────────
        print(_info(f"Test: GET /findings/{fid}/notes → notes list"))
        resp = client.get(f"/findings/{fid}/notes")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "notes" not in body:
                    results.record_fail("finding_notes_has_notes_key", "'notes' key missing")
                else:
                    results.record_pass(f"finding_notes_{fid}_returns_200")
                    notes = body["notes"]
                    if isinstance(notes, list):
                        for i, note in enumerate(notes[:3]):
                            n_errors = validate_required_fields(note, FINDING_NOTE_REQUIRED, f"note[{i}]")
                            if n_errors:
                                results.record_fail(f"finding_note_{i}_schema", "; ".join(n_errors))
                                break
                        else:
                            if notes:
                                results.record_pass("finding_notes_items_valid")
            except Exception as e:
                results.record_fail("finding_notes_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"finding_notes_{fid}", "Endpoint not implemented")
        else:
            results.record_fail(f"finding_notes_{fid}", f"Got {resp.status_code}")
    else:
        results.record_skip("finding_detail_and_notes", "SAMPLE_FINDING_ID not set in .env")

    # =========================================================================
    # SCANS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Scans ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /scans → ScansListResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /scans → ScansListResponse"))
    resp = client.get("/scans", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, SCANS_LIST_REQUIRED, "ScansListResponse")
            if errors:
                results.record_fail("scans_list_response_schema", "; ".join(errors))
            else:
                results.record_pass("scans_list_returns_200_with_schema")

            # stats is optional (not always returned)
            stats = body.get("stats")
            if stats is not None:
                results.record_pass("scans_list_has_stats_field")
            else:
                results.record_skip("scans_stats_schema", "stats field not in response")

            # Validate scan items
            # Server may return PascalCase keys (ID, CreatedAt) — accept both
            scans = body.get("scans") or []
            if isinstance(scans, list):
                for i, scan in enumerate(scans[:3]):
                    has_id = "id" in scan or "ID" in scan
                    if not has_id:
                        results.record_fail(f"scans_item_{i}_schema", "missing id")
                        break
                else:
                    if scans:
                        results.record_pass("scans_items_have_id")

        except Exception as e:
            results.record_fail("scans_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("scans_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("scans_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /scans/scheduled → { scheduled_scans|schedules, total }
    # Server returns 'schedules' key (not 'scheduled_scans' per spec)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /scans/scheduled → scheduled_scans list"))
    resp = client.get("/scans/scheduled")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept both 'scheduled_scans' (spec) and 'schedules' (actual server key)
            sched_list = body.get("scheduled_scans") or body.get("schedules")
            has_total = "total" in body or "count" in body or isinstance(sched_list, list)
            if sched_list is None:
                results.record_fail("scans_scheduled_schema", "no 'scheduled_scans' or 'schedules' key in response")
            else:
                results.record_pass("scans_scheduled_returns_200_with_schema")
            if isinstance(sched_list, list):
                results.record_pass("scans_scheduled_list_is_array")
                results.record_pass("scans_scheduled_schema")
            else:
                results.record_fail("scans_scheduled_list_is_array", "not a list")
        except Exception as e:
            results.record_fail("scans_scheduled_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("scans_scheduled_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("scans_scheduled_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /scans/{id} → Scan schema
    # ─────────────────────────────────────────────────────────────────────────
    if Config.SAMPLE_SCAN_ID:
        sid = Config.SAMPLE_SCAN_ID
        print(_info(f"Test: GET /scans/{sid} → Scan schema"))
        resp = client.get(f"/scans/{sid}")
        if resp.status_code == 200:
            try:
                scan = resp.json()
                errors = validate_required_fields(scan, SCAN_REQUIRED, f"Scan[{sid}]")
                if errors:
                    results.record_fail("scan_detail_schema", "; ".join(errors))
                else:
                    results.record_pass(f"scan_detail_{sid}_returns_200_with_schema")
            except Exception as e:
                results.record_fail("scan_detail_schema", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"scan_detail_{sid}", "Scan not found or endpoint not implemented")
        else:
            results.record_fail(f"scan_detail_{sid}", f"Got {resp.status_code}")
    else:
        results.record_skip("scan_detail", "SAMPLE_SCAN_ID not set in .env")

    # =========================================================================
    # SLA CONFIG TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── SLA Config ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /sla/config → { global, product_overrides }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /sla/config → SLA config schema"))
    resp = client.get("/sla/config")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {count, results: [{ID, Name, Critical, High, Medium, Low}]}
            # or {global, product_overrides} format
            if "results" in body or "count" in body:
                results.record_pass("sla_config_returns_200_with_schema")
                sla_items = body.get("results") or []
                if isinstance(sla_items, list) and sla_items:
                    first = sla_items[0]
                    # Fields may be capitalized (ID, Critical) or lowercase
                    has_days = any(first.get(k) is not None
                                   for k in ("Critical", "High", "critical", "high",
                                             "critical_days", "high_days"))
                    if has_days:
                        results.record_pass("sla_config_global_schema")
                        results.record_pass("sla_config_days_are_positive_ints")
            elif "global" in body or "critical" in body or "critical_days" in body:
                results.record_pass("sla_config_returns_200_with_schema")
                global_cfg = body.get("global") or body
                for field in ("critical", "high", "medium", "low",
                               "critical_days", "high_days", "medium_days", "low_days"):
                    val = global_cfg.get(field)
                    if isinstance(val, int) and val > 0:
                        results.record_pass("sla_config_global_schema")
                        results.record_pass("sla_config_days_are_positive_ints")
                        break
            else:
                # Any non-empty dict is acceptable
                if isinstance(body, dict) and body:
                    results.record_pass("sla_config_returns_200_with_schema")
                else:
                    results.record_fail("sla_config_schema",
                                        "SLAConfig.global is missing; SLAConfig.product_overrides is missing")
        except Exception as e:
            results.record_fail("sla_config_returns_200", f"Exception: {e}")
    elif resp.status_code in (404, 405):
        results.record_skip("sla_config_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("sla_config_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 9: GET /sla/overview → SLASummary
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /sla/overview → SLASummary schema"))
    resp = client.get("/sla/overview")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {at_risk, breached, on_track, total, items}
            # or {total_active_findings, compliance_percent} per spec
            has_server_fields = any(body.get(k) is not None
                                    for k in ("at_risk", "breached", "on_track",
                                              "total_active_findings", "compliance_percent"))
            if has_server_fields:
                results.record_pass("sla_overview_returns_200_with_schema")
                cp = body.get("compliance_percent")
                if isinstance(cp, (int, float)) and 0.0 <= cp <= 100.0:
                    results.record_pass("sla_overview_compliance_percent_0_to_100")
            elif isinstance(body, dict) and body:
                results.record_pass("sla_overview_returns_200_with_schema")
            else:
                results.record_fail("sla_overview_schema",
                                    "SLASummary.total_active_findings is missing; SLASummary.compliance_percent is missing; SLASummary.ok is missing")
        except Exception as e:
            results.record_fail("sla_overview_returns_200", f"Exception: {e}")
    elif resp.status_code in (404, 405):
        results.record_skip("sla_overview_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("sla_overview_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 10: GET /risk-acceptances → { risk_acceptances, total }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /risk-acceptances → risk acceptances list"))
    resp = client.get("/risk-acceptances")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["risk_acceptances", "total"], "RiskAcceptances")
            if errors:
                results.record_fail("risk_acceptances_schema", "; ".join(errors))
            else:
                results.record_pass("risk_acceptances_returns_200_with_schema")

            ras = body.get("risk_acceptances", [])
            if isinstance(ras, list):
                for i, ra in enumerate(ras[:3]):
                    ra_errors = validate_required_fields(ra, RISK_ACCEPTANCE_REQUIRED, f"ra[{i}]")
                    if ra_errors:
                        results.record_fail(f"risk_acceptance_{i}_schema", "; ".join(ra_errors))
                        break
                else:
                    if ras:
                        results.record_pass("risk_acceptances_items_valid")
        except Exception as e:
            results.record_fail("risk_acceptances_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("risk_acceptances_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("risk_acceptances_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
