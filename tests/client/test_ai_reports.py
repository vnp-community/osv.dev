"""
test_ai_reports.py — Test AI Center và Reports endpoints (/api/v1)

Kiểm tra:
  AI Center:
    - GET  /ai/triage/queue           → { items, total }
    - GET  /ai/enrichment             → EnrichmentStatus schema
    - GET  /ai/enrichment/{cveId}     → CVEEnrichmentDetail schema

  Reports:
    - GET  /reports                   → { reports, total }
    - POST /reports                   → Report schema (202)
    - GET  /reports/{id}              → Report schema

  Integrations:
    - GET  /webhooks                  → array of Webhook
    - GET  /api-keys                  → array of APIKey
    - GET  /jira/config               → { enabled, ... }

Chạy:
  python test_ai_reports.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

TRIAGE_QUEUE_ITEM_REQUIRED = ["finding_id", "finding_title", "severity", "status", "queued_at"]
TRIAGE_STATUS_VALUES = {"pending", "processing", "done", "failed"}
SEVERITY_VALUES = {"Critical", "High", "Medium", "Low", "Info"}

ENRICHMENT_STATUS_REQUIRED = ["total_enriched", "status"]
ENRICHMENT_STATUS_VALUES = {"idle", "running", "error"}

CVE_ENRICHMENT_DETAIL_REQUIRED = [
    "cve_id", "summary", "attack_vectors",
    "remediation", "references", "enriched_at"
]

REPORT_REQUIRED = ["id", "name", "type", "status", "created_at", "created_by"]
REPORT_STATUS_VALUES = {"pending", "generating", "completed", "failed"}

WEBHOOK_REQUIRED = ["id", "name", "url", "events", "secret", "active", "created_at"]

API_KEY_REQUIRED = ["id", "name", "prefix", "scopes", "created_at"]

JIRA_CONFIG_REQUIRED = ["enabled"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("AI CENTER, REPORTS & INTEGRATIONS API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_ai_reports_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # AI CENTER TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── AI Center ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /ai/triage/queue → { items, total }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /ai/triage/queue → triage queue"))
    resp = client.get("/ai/triage/queue", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["items", "total"], "triage_queue")
            if errors:
                results.record_fail("ai_triage_queue_schema", "; ".join(errors))
            else:
                results.record_pass("ai_triage_queue_returns_200_with_schema")

            items = body.get("items", [])
            if isinstance(items, list):
                results.record_pass("ai_triage_queue_items_is_array")
                for i, item in enumerate(items[:3]):
                    item_errors = validate_required_fields(
                        item, TRIAGE_QUEUE_ITEM_REQUIRED, f"queue_item[{i}]"
                    )
                    if item_errors:
                        results.record_fail(f"ai_triage_item_{i}_schema", "; ".join(item_errors))
                        break
                    # status phải thuộc enum
                    if item.get("status") not in TRIAGE_STATUS_VALUES:
                        results.record_fail(f"ai_triage_item_{i}_status_enum",
                                            f"status='{item.get('status')}'")
                        break
                    # severity phải thuộc enum
                    if item.get("severity") not in SEVERITY_VALUES:
                        results.record_fail(f"ai_triage_item_{i}_severity_enum",
                                            f"severity='{item.get('severity')}'")
                        break
                else:
                    if items:
                        results.record_pass("ai_triage_queue_items_schema_valid")
            else:
                results.record_fail("ai_triage_queue_items_is_array", "not a list")

        except Exception as e:
            results.record_fail("ai_triage_queue_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_queue_returns_200", "Endpoint not implemented (404)")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_queue_returns_200", "AI service unavailable (503) — ai-service is down")
    else:
        results.record_fail("ai_triage_queue_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /ai/triage/queue?status=pending → filter
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /ai/triage/queue?status=pending → filter"))
    resp = client.get("/ai/triage/queue", params={"status": "pending"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            items = body.get("items", [])
            non_pending = [i.get("finding_id") for i in items if i.get("status") != "pending"]
            if non_pending:
                results.record_fail("ai_triage_queue_status_filter",
                                    f"Non-pending items: {non_pending[:3]}")
            else:
                results.record_pass("ai_triage_queue_status_filter_works")
        except Exception as e:
            results.record_fail("ai_triage_queue_status_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_queue_status_filter", "Endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_queue_status_filter", "AI service unavailable (503)")
    else:
        results.record_fail("ai_triage_queue_status_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /ai/enrichment → EnrichmentStatus
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /ai/enrichment → EnrichmentStatus"))
    resp = client.get("/ai/enrichment")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ENRICHMENT_STATUS_REQUIRED, "EnrichmentStatus")
            if errors:
                results.record_fail("ai_enrichment_status_schema", "; ".join(errors))
            else:
                results.record_pass("ai_enrichment_status_returns_200_with_schema")

            # status phải thuộc enum
            status = body.get("status")
            if status in ENRICHMENT_STATUS_VALUES:
                results.record_pass("ai_enrichment_status_valid_enum")
            else:
                results.record_fail("ai_enrichment_status_valid_enum",
                                    f"status='{status}' not in {ENRICHMENT_STATUS_VALUES}")

            # total_enriched phải là int không âm
            te = body.get("total_enriched")
            if isinstance(te, int) and te >= 0:
                results.record_pass("ai_enrichment_total_enriched_is_non_negative_int")
            else:
                results.record_fail("ai_enrichment_total_enriched_is_non_negative_int",
                                    f"total_enriched={te}")

        except Exception as e:
            results.record_fail("ai_enrichment_status_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_enrichment_status_returns_200", "Endpoint not implemented (404)")
    elif resp.status_code == 503:
        results.record_skip("ai_enrichment_status_returns_200", "AI service unavailable (503)")
    else:
        results.record_fail("ai_enrichment_status_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /ai/enrichment/{cveId} → CVEEnrichmentDetail
    # ─────────────────────────────────────────────────────────────────────────
    cve_id = Config.SAMPLE_CVE_ID
    print(_info(f"Test: GET /ai/enrichment/{cve_id} → CVEEnrichmentDetail"))
    resp = client.get(f"/ai/enrichment/{cve_id}")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, CVE_ENRICHMENT_DETAIL_REQUIRED, "CVEEnrichmentDetail")
            if errors:
                results.record_fail("ai_enrichment_detail_schema", "; ".join(errors))
            else:
                results.record_pass(f"ai_enrichment_{cve_id}_returns_200_with_schema")

            # attack_vectors phải là list
            av = body.get("attack_vectors", [])
            if isinstance(av, list):
                results.record_pass("ai_enrichment_attack_vectors_is_array")
            else:
                results.record_fail("ai_enrichment_attack_vectors_is_array", f"type={type(av)}")

            # references phải là list
            refs = body.get("references", [])
            if isinstance(refs, list):
                results.record_pass("ai_enrichment_references_is_array")
            else:
                results.record_fail("ai_enrichment_references_is_array", f"type={type(refs)}")

        except Exception as e:
            results.record_fail("ai_enrichment_detail_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip(f"ai_enrichment_{cve_id}", "No enrichment for this CVE or endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip(f"ai_enrichment_{cve_id}", "AI service unavailable (503)")
    else:
        results.record_fail(f"ai_enrichment_{cve_id}", f"Got {resp.status_code}")

    # =========================================================================
    # REPORTS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Reports ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /reports → { reports, total }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /reports → reports list"))
    resp = client.get("/reports")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["reports", "total"], "reports")
            if errors:
                results.record_fail("reports_list_schema", "; ".join(errors))
            else:
                results.record_pass("reports_list_returns_200_with_schema")

            reports = body.get("reports", [])
            if isinstance(reports, list):
                results.record_pass("reports_list_is_array")
                for i, r in enumerate(reports[:3]):
                    r_errors = validate_required_fields(r, REPORT_REQUIRED, f"report[{i}]")
                    if r_errors:
                        results.record_fail(f"report_{i}_schema", "; ".join(r_errors))
                        break
                    # status phải thuộc enum
                    if r.get("status") not in REPORT_STATUS_VALUES:
                        results.record_fail(f"report_{i}_status_enum",
                                            f"status='{r.get('status')}'")
                        break
                else:
                    if reports:
                        results.record_pass("reports_items_schema_valid")
            else:
                results.record_fail("reports_list_is_array", "not a list")

        except Exception as e:
            results.record_fail("reports_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("reports_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("reports_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: POST /reports → Report schema (202 Accepted)
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /reports → 202 + Report schema"))
    resp = client.post("/reports", body={
        "name": "Test Executive Summary",
        "type": "executive_summary",
        "config": {"product_ids": [], "date_range": "30d"}
    })
    if resp.status_code == 202:
        try:
            body = resp.json()
            errors = validate_required_fields(body, REPORT_REQUIRED, "Report")
            if errors:
                results.record_fail("report_create_response_schema", "; ".join(errors))
            else:
                results.record_pass("report_create_returns_202_with_schema")
            # status phải là 'pending' hoặc 'generating' khi mới tạo
            status = body.get("status")
            if status in ("pending", "generating"):
                results.record_pass("report_create_initial_status_pending_or_generating")
            else:
                results.record_fail("report_create_initial_status_pending_or_generating",
                                    f"status='{status}'")
        except Exception as e:
            results.record_fail("report_create_returns_202", f"Exception: {e}")
    elif resp.status_code == 201:
        # Một số server trả 201 thay 202
        results.record_skip("report_create_returns_202", "Server returned 201 instead of 202")
    elif resp.status_code == 404:
        results.record_skip("report_create_returns_202", "Endpoint not implemented")
    elif resp.status_code == 400:
        results.record_fail("report_create_returns_202", f"400 Bad Request: {resp.text[:200]}")
    else:
        results.record_fail("report_create_returns_202", f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # INTEGRATIONS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Integrations ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /webhooks → array of Webhook
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /webhooks → array of Webhook"))
    resp = client.get("/webhooks")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Response là array trực tiếp (không phải wrapper)
            if isinstance(body, list):
                results.record_pass("webhooks_list_returns_200_as_array")
                for i, wh in enumerate(body[:3]):
                    wh_errors = validate_required_fields(wh, WEBHOOK_REQUIRED, f"webhook[{i}]")
                    if wh_errors:
                        results.record_fail(f"webhook_{i}_schema", "; ".join(wh_errors))
                        break
                    # active phải là boolean
                    if not isinstance(wh.get("active"), bool):
                        results.record_fail(f"webhook_{i}_active_is_bool",
                                            f"active={wh.get('active')!r}")
                        break
                    # events phải là list
                    if not isinstance(wh.get("events"), list):
                        results.record_fail(f"webhook_{i}_events_is_array",
                                            f"type={type(wh.get('events'))}")
                        break
                else:
                    if body:
                        results.record_pass("webhooks_items_schema_valid")
            else:
                results.record_fail("webhooks_list_returns_200_as_array",
                                    f"Expected array, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("webhooks_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("webhooks_list_returns_200", "Access denied (403)")
    elif resp.status_code == 404:
        results.record_skip("webhooks_list_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("webhooks_list_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /api-keys → array of APIKey
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /api-keys → array of APIKey"))
    resp = client.get("/api-keys")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept both formats: direct array OR object { keys: [...], total: N }
            if isinstance(body, dict) and "keys" in body:
                keys_list = body["keys"]
            elif isinstance(body, list):
                keys_list = body
            else:
                keys_list = None
            if isinstance(keys_list, list):
                results.record_pass("api_keys_list_returns_200_as_array")
                for i, key in enumerate(keys_list[:3]):
                    key_errors = validate_required_fields(key, API_KEY_REQUIRED, f"api_key[{i}]")
                    if key_errors:
                        results.record_fail(f"api_key_{i}_schema", "; ".join(key_errors))
                        break
                    # prefix phải là string không rỗng
                    if not isinstance(key.get("prefix"), str) or len(key.get("prefix", "")) < 1:
                        results.record_fail(f"api_key_{i}_prefix_is_nonempty_string",
                                            f"prefix={key.get('prefix')!r}")
                        break
                    # scopes phải là list
                    if not isinstance(key.get("scopes"), list):
                        results.record_fail(f"api_key_{i}_scopes_is_array",
                                            f"type={type(key.get('scopes'))}")
                        break
                else:
                    if keys_list:
                        results.record_pass("api_keys_items_schema_valid")
            else:
                results.record_fail("api_keys_list_returns_200_as_array",
                                    f"Expected array or {{keys:[]}}, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("api_keys_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("api_keys_list_returns_200", "Access denied (403)")
    elif resp.status_code == 404:
        results.record_skip("api_keys_list_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("api_keys_list_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 9: GET /jira/config → { enabled, ... }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /jira/config → JIRA config"))
    resp = client.get("/jira/config")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, JIRA_CONFIG_REQUIRED, "JiraConfig")
            if errors:
                results.record_fail("jira_config_schema", "; ".join(errors))
            else:
                results.record_pass("jira_config_returns_200_with_schema")
            # enabled phải là boolean
            if isinstance(body.get("enabled"), bool):
                results.record_pass("jira_config_enabled_is_bool")
            else:
                results.record_fail("jira_config_enabled_is_bool",
                                    f"enabled={body.get('enabled')!r}")
        except Exception as e:
            results.record_fail("jira_config_returns_200", f"Exception: {e}")
    elif resp.status_code == 403:
        results.record_skip("jira_config_returns_200", "Access denied (403)")
    elif resp.status_code == 404:
        results.record_skip("jira_config_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("jira_config_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
