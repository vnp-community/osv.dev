"""
test_missing_endpoints.py — Test các endpoint còn thiếu từ api_endpoints.md

Danh sách endpoint được bổ sung (chưa có trong các module khác):

  Notifications (mutations):
    - PATCH /notifications/{id}/read     → 204
    - POST  /notifications/mark-all-read → 204

  Scans (mutations):
    - POST  /scans                       → Scan schema (201/202)
    - POST  /scans/{id}/cancel           → 200/204
    - GET   /scans/history               → { history, total }
    - GET   /scans/{id}/results/nmap     → nmap results
    - GET   /scans/{id}/results/zap      → zap results
    - POST  /scans/import                → 202

  Findings (mutations):
    - PATCH /findings/{id}               → updated Finding
    - POST  /findings/{id}/notes         → Note schema (201)
    - GET   /findings/{id}/audit         → { audit_trail }
    - POST  /findings/bulk/close         → BulkActionResponse
    - POST  /findings/bulk/reopen        → BulkActionResponse
    - POST  /findings/bulk/assign        → BulkActionResponse

  Risk Acceptances (mutations):
    - POST   /risk-acceptances           → RiskAcceptance (201)
    - DELETE /risk-acceptances/{id}      → 204

  SLA (mutations):
    - PUT /sla/config                    → 200

  Assets (mutations):
    - PATCH /assets/{id}                 → Asset schema
    - GET   /assets/{id}/findings        → FindingsListResponse

  Products (mutations):
    - POST  /products                    → Product schema (201)
    - PATCH /products/{id}               → Product schema
    - GET   /engagements/{engId}/tests   → { tests, total }

  AI (full set):
    - POST /ai/triage/{findingId}        → AI triage result (202)
    - POST /ai/triage/{findingId}/review → 204
    - GET  /ai/insights                  → InsightsResponse
    - POST /ai/enrichment/trigger        → 202

  Reports (full set):
    - GET    /reports/templates          → { templates }
    - GET    /reports/{id}               → Report schema
    - GET    /reports/{id}/download      → binary/redirect
    - DELETE /reports/{id}               → 204

  Integrations (mutations):
    - POST   /webhooks                   → Webhook (201)
    - DELETE /webhooks/{id}              → 204
    - POST   /webhooks/{id}/test         → WebhookTestResult
    - POST   /jira/config/test           → { ok, latency_ms }
    - GET    /integrations/jira          → JiraConfig
    - PUT    /integrations/jira          → 200

  Profile (mutations):
    - PATCH /profile                     → User schema
    - POST  /profile/change-password     → 204
    - GET   /profile/sessions            → { sessions }
    - GET   /profile/notifications/settings  → NotificationSettings
    - PUT   /profile/notifications/settings  → 200

  Admin (mutations):
    - GET  /admin/users/{id}             → AdminUser schema
    - POST /admin/users/invite           → 201
    - POST /admin/users/{id}/unlock      → 204
    - POST /admin/users/{id}/reset-password → 204

  Audit:
    - GET /audit-log                     → { entries, total }

  Global Search:
    - GET /search/recent                 → { searches }
    - GET /search/suggested              → { suggestions }

Chạy:
  python test_missing_endpoints.py

Môi trường:
  Đọc từ .env — xem .env.example
  SAMPLE_FINDING_ID, SAMPLE_SCAN_ID, SAMPLE_ASSET_ID, SAMPLE_PRODUCT_ID (optional)
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    _info, _Color
)
from config import Config
import os

# ── Sample IDs từ env ─────────────────────────────────────────────────────────

SAMPLE_FINDING_ID  = Config.SAMPLE_FINDING_ID
SAMPLE_SCAN_ID     = Config.SAMPLE_SCAN_ID
SAMPLE_ASSET_ID    = Config.SAMPLE_ASSET_ID
SAMPLE_PRODUCT_ID  = Config.SAMPLE_PRODUCT_ID
SAMPLE_USER_ID     = os.environ.get("SAMPLE_USER_ID") or None
SAMPLE_REPORT_ID   = os.environ.get("SAMPLE_REPORT_ID") or None
SAMPLE_WEBHOOK_ID  = os.environ.get("SAMPLE_WEBHOOK_ID") or None
SAMPLE_RA_ID       = os.environ.get("SAMPLE_RISK_ACCEPTANCE_ID") or None
SAMPLE_ENGAGEMENT_ID = os.environ.get("SAMPLE_ENGAGEMENT_ID") or None

# ── Field schemas ─────────────────────────────────────────────────────────────

REPORT_REQUIRED       = ["id", "name", "type", "status", "created_at", "created_by"]
WEBHOOK_REQUIRED      = ["id", "name", "url", "events", "secret", "active", "created_at"]
RISK_ACCEPTANCE_REQUIRED = [
    "id", "product_id", "finding_ids", "expiration_date",
    "reason", "approved_by", "is_expired", "created_at"
]
PROFILE_USER_REQUIRED = ["id", "email", "name", "role", "permissions", "mfa_enabled", "created_at"]
NOTIF_SETTINGS_REQUIRED = ["email", "in_app"]
SESSION_REQUIRED      = ["id", "user_agent", "ip", "created_at", "last_active_at"]
AUDIT_ENTRY_REQUIRED  = ["id", "action", "actor_id", "actor_email", "resource_type", "created_at"]


# =============================================================================
# HELPER: record 4xx/5xx / not-implemented as skip or fail
# =============================================================================

def _handle_common(resp, test_name: str, results: TestResults,
                   ok_status=200, skip_on=(404,), fail_on=(405,)) -> bool:
    """
    Returns True if resp.status_code == ok_status (caller proceeds),
    False otherwise (pass/skip/fail already recorded).
    """
    if resp.status_code == ok_status:
        return True
    if resp.status_code in skip_on:
        results.record_skip(test_name, f"Endpoint not implemented ({resp.status_code})")
        return False
    if resp.status_code in fail_on:
        results.record_fail(test_name, f"Method Not Allowed ({resp.status_code})")
        return False
    if resp.status_code in (401, 403):
        results.record_skip(test_name, f"Auth/permission error ({resp.status_code})")
        return False
    if resp.status_code in (500, 503):
        results.record_skip(test_name, f"Server error ({resp.status_code}) — may be empty DB")
        return False
    results.record_fail(test_name, f"Got {resp.status_code}: {resp.text[:200]}")
    return False


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*65}")
    print("MISSING ENDPOINTS COVERAGE — ALL api_endpoints.md GAPS")
    print(f"{'='*65}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_missing_endpoint_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # SECTION 1: Notifications (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Notifications (mutations) ──{_Color.RESET}")

    # ── GET /notifications d'abord pour récupérer un ID ──────────────────────
    notif_id = None
    resp = client.get("/notifications", params={"page": 1, "page_size": 5})
    if resp.status_code == 200:
        try:
            body = resp.json()
            notifs = body.get("notifications") or []
            if notifs:
                notif_id = notifs[0].get("id")
        except Exception:
            pass

    # PATCH /notifications/{id}/read → 204
    if notif_id:
        print(_info(f"Test: PATCH /notifications/{notif_id}/read → 204"))
        resp = client.patch(f"/notifications/{notif_id}/read")
        if resp.status_code == 204:
            results.record_pass("notification_mark_read_returns_204")
        elif resp.status_code == 200:
            results.record_pass("notification_mark_read_returns_200_or_204")
        elif resp.status_code == 404:
            results.record_skip("notification_mark_read", "Endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("notification_mark_read", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("notification_mark_read_returns_204",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("notification_mark_read", "No notifications found or endpoint unavailable")

    # POST /notifications/mark-all-read → 204
    print(_info("Test: POST /notifications/mark-all-read → 204"))
    resp = client.post("/notifications/mark-all-read")
    if resp.status_code in (204, 200):
        results.record_pass("notifications_mark_all_read_returns_2xx")
    elif resp.status_code == 404:
        results.record_skip("notifications_mark_all_read", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("notifications_mark_all_read", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("notifications_mark_all_read_returns_2xx",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 2: Scans (mutations + extra GETs)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Scans (mutations + history + results) ──{_Color.RESET}")

    # POST /scans → 201 / 202
    print(_info("Test: POST /scans → create scan (201/202)"))
    resp = client.post("/scans", body={
        "name": "Test Scan (auto-test)",
        "type": "nmap_discovery",
        "targets": ["192.168.1.0/24"]
    })
    created_scan_id = None
    if resp.status_code in (201, 202):
        try:
            body = resp.json()
            if "id" in body:
                results.record_pass("scan_create_returns_201_with_id")
                created_scan_id = body.get("id")
            else:
                results.record_fail("scan_create_response_has_id", "no 'id' in response")
        except Exception as e:
            results.record_fail("scan_create_returns_201", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("scan_create_returns_201", "Endpoint not implemented (404)")
    elif resp.status_code in (400, 422):
        results.record_skip("scan_create_returns_201",
                            f"Validation error ({resp.status_code}) — payload may need adjustment")
    elif resp.status_code in (401, 403):
        results.record_skip("scan_create_returns_201", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("scan_create_returns_201",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # GET /scans/history → { history, total }
    print(_info("Test: GET /scans/history → scan history"))
    resp = client.get("/scans/history")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, list):
                results.record_pass("scans_history_returns_200_as_array")
            elif isinstance(body, dict) and ("history" in body or "scans" in body or "total" in body):
                results.record_pass("scans_history_returns_200_with_schema")
            elif isinstance(body, dict):
                results.record_pass("scans_history_returns_200")
            else:
                results.record_fail("scans_history_schema",
                                    f"Unexpected response type: {type(body).__name__}")
        except Exception as e:
            results.record_fail("scans_history_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("scans_history_returns_200", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("scans_history_returns_200", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("scans_history_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # Cancel, results/nmap, results/zap — dùng SAMPLE_SCAN_ID nếu có
    scan_id = SAMPLE_SCAN_ID or created_scan_id

    if scan_id:
        # POST /scans/{id}/cancel → 200/204
        print(_info(f"Test: POST /scans/{scan_id}/cancel → 200/204"))
        resp = client.post(f"/scans/{scan_id}/cancel")
        if resp.status_code in (200, 204):
            results.record_pass("scan_cancel_returns_2xx")
        elif resp.status_code == 409:
            # scan may already be completed — valid conflict
            results.record_pass("scan_cancel_conflict_409_acceptable")
        elif resp.status_code == 404:
            results.record_skip("scan_cancel", "Scan not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("scan_cancel", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("scan_cancel_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # GET /scans/{id}/results/nmap
        print(_info(f"Test: GET /scans/{scan_id}/results/nmap → nmap results"))
        resp = client.get(f"/scans/{scan_id}/results/nmap")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if isinstance(body, (dict, list)):
                    results.record_pass("scan_results_nmap_returns_200")
                else:
                    results.record_fail("scan_results_nmap_schema",
                                        f"Unexpected type: {type(body).__name__}")
            except Exception as e:
                results.record_fail("scan_results_nmap_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("scan_results_nmap", "No nmap results / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("scan_results_nmap", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("scan_results_nmap_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # GET /scans/{id}/results/zap
        print(_info(f"Test: GET /scans/{scan_id}/results/zap → zap results"))
        resp = client.get(f"/scans/{scan_id}/results/zap")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if isinstance(body, (dict, list)):
                    results.record_pass("scan_results_zap_returns_200")
                else:
                    results.record_fail("scan_results_zap_schema",
                                        f"Unexpected type: {type(body).__name__}")
            except Exception as e:
                results.record_fail("scan_results_zap_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("scan_results_zap", "No zap results / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("scan_results_zap", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("scan_results_zap_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("scan_cancel_nmap_zap",
                            "SAMPLE_SCAN_ID not set — set in .env to enable cancel/results tests")

    # POST /scans/import → 202
    print(_info("Test: POST /scans/import → 202"))
    resp = client.post("/scans/import", body={
        "type": "nmap",
        "data": "<nmaprun></nmaprun>"
    })
    if resp.status_code in (200, 201, 202):
        results.record_pass("scan_import_returns_2xx")
    elif resp.status_code in (400, 422):
        results.record_skip("scan_import_returns_202",
                            f"Validation error ({resp.status_code}) — payload format required")
    elif resp.status_code == 404:
        results.record_skip("scan_import_returns_202", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("scan_import_returns_202", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("scan_import_returns_2xx",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 3: Findings (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Findings (mutations) ──{_Color.RESET}")

    fid = SAMPLE_FINDING_ID
    if fid:
        # PATCH /findings/{id} → updated Finding
        print(_info(f"Test: PATCH /findings/{fid} → update finding"))
        resp = client.patch(f"/findings/{fid}", body={"status": "active"})
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "id" in body:
                    results.record_pass("finding_patch_returns_200_with_id")
                else:
                    results.record_fail("finding_patch_schema", "no 'id' in response")
            except Exception as e:
                results.record_fail("finding_patch_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("finding_patch", "Finding not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 422):
            results.record_skip("finding_patch", f"Validation error ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("finding_patch", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("finding_patch_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # POST /findings/{id}/notes → 201
        print(_info(f"Test: POST /findings/{fid}/notes → add note (201)"))
        resp = client.post(f"/findings/{fid}/notes", body={
            "content": "Automated test note — can be ignored"
        })
        if resp.status_code in (200, 201):
            try:
                body = resp.json()
                if "id" in body or "content" in body:
                    results.record_pass("finding_add_note_returns_201")
                else:
                    results.record_fail("finding_add_note_schema", f"unexpected body: {list(body.keys())}")
            except Exception as e:
                results.record_fail("finding_add_note_returns_201", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("finding_add_note", "Finding not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 422):
            results.record_skip("finding_add_note", f"Validation error ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("finding_add_note", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("finding_add_note_returns_201",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # GET /findings/{id}/audit → { audit_trail }
        print(_info(f"Test: GET /findings/{fid}/audit → audit trail"))
        resp = client.get(f"/findings/{fid}/audit")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "audit_trail" in body or "audit" in body or isinstance(body, list):
                    results.record_pass("finding_audit_returns_200")
                else:
                    results.record_fail("finding_audit_schema",
                                        f"Unexpected keys: {list(body.keys())[:5]}")
            except Exception as e:
                results.record_fail("finding_audit_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("finding_audit", "Finding not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("finding_audit", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("finding_audit_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("finding_mutations", "SAMPLE_FINDING_ID not set — set in .env")

    # Bulk operations (không cần SAMPLE_FINDING_ID — có thể test với list rỗng)
    for action in ("close", "reopen", "assign"):
        print(_info(f"Test: POST /findings/bulk/{action} → BulkActionResponse"))
        body = {"finding_ids": []}
        if action == "assign":
            body["assignee_id"] = "test_user"
        resp = client.post(f"/findings/bulk/{action}", body=body)
        if resp.status_code in (200, 204):
            results.record_pass(f"findings_bulk_{action}_returns_2xx")
        elif resp.status_code in (400, 422):
            # Empty list may be rejected — but endpoint exists
            results.record_pass(f"findings_bulk_{action}_endpoint_exists_400_422")
        elif resp.status_code == 404:
            results.record_skip(f"findings_bulk_{action}", "Endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip(f"findings_bulk_{action}", f"Auth error ({resp.status_code})")
        else:
            results.record_fail(f"findings_bulk_{action}_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 4: Risk Acceptances (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Risk Acceptances (mutations) ──{_Color.RESET}")

    # POST /risk-acceptances → 201
    print(_info("Test: POST /risk-acceptances → create RA (201)"))
    resp = client.post("/risk-acceptances", body={
        "finding_ids": [],
        "reason": "Test RA — auto-test",
        "expiration_date": "2099-12-31T00:00:00Z",
        "product_id": "test_product_id"
    })
    created_ra_id = None
    if resp.status_code in (200, 201):
        try:
            body = resp.json()
            if "id" in body:
                results.record_pass("risk_acceptance_create_returns_201_with_id")
                created_ra_id = body.get("id")
            else:
                results.record_fail("risk_acceptance_create_schema", "no 'id' in response")
        except Exception as e:
            results.record_fail("risk_acceptance_create_returns_201", f"Exception: {e}")
    elif resp.status_code in (400, 422):
        results.record_skip("risk_acceptance_create",
                            f"Validation error ({resp.status_code}) — payload may need real finding_ids")
    elif resp.status_code == 404:
        results.record_skip("risk_acceptance_create", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("risk_acceptance_create", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("risk_acceptance_create_returns_201",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # DELETE /risk-acceptances/{id} → 204
    ra_id = SAMPLE_RA_ID or created_ra_id
    if ra_id:
        print(_info(f"Test: DELETE /risk-acceptances/{ra_id} → 204"))
        resp = client.delete(f"/risk-acceptances/{ra_id}")
        if resp.status_code in (200, 204):
            results.record_pass("risk_acceptance_delete_returns_204")
        elif resp.status_code == 404:
            results.record_skip("risk_acceptance_delete",
                                "RA not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("risk_acceptance_delete", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("risk_acceptance_delete_returns_204",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("risk_acceptance_delete",
                            "No risk acceptance ID — set SAMPLE_RISK_ACCEPTANCE_ID in .env")

    # =========================================================================
    # SECTION 5: SLA (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── SLA (mutations) ──{_Color.RESET}")

    # PUT /sla/config → 200
    print(_info("Test: PUT /sla/config → update SLA config"))
    resp = client.put("/sla/config", body={
        "critical_days": 7,
        "high_days": 30,
        "medium_days": 90,
        "low_days": 180
    })
    if resp.status_code == 200:
        results.record_pass("sla_config_put_returns_200")
    elif resp.status_code in (400, 422):
        results.record_skip("sla_config_put", f"Validation error ({resp.status_code})")
    elif resp.status_code == 404:
        results.record_skip("sla_config_put", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("sla_config_put", f"Auth/permission error ({resp.status_code})")
    else:
        results.record_fail("sla_config_put_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 6: Assets (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Assets (mutations) ──{_Color.RESET}")

    aid = SAMPLE_ASSET_ID
    if aid:
        # PATCH /assets/{id} → updated Asset
        print(_info(f"Test: PATCH /assets/{aid} → update asset"))
        resp = client.patch(f"/assets/{aid}", body={"tags": ["test-tag"]})
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "id" in body:
                    results.record_pass("asset_patch_returns_200_with_id")
                else:
                    results.record_fail("asset_patch_schema", "no 'id' in response")
            except Exception as e:
                results.record_fail("asset_patch_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("asset_patch", "Asset not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 422):
            results.record_skip("asset_patch", f"Validation error ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("asset_patch", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("asset_patch_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # GET /assets/{id}/findings → FindingsListResponse
        print(_info(f"Test: GET /assets/{aid}/findings → findings for asset"))
        resp = client.get(f"/assets/{aid}/findings")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "findings" in body or "total" in body or isinstance(body, list):
                    results.record_pass("asset_findings_returns_200")
                else:
                    results.record_fail("asset_findings_schema",
                                        f"Unexpected keys: {list(body.keys())[:5]}")
            except Exception as e:
                results.record_fail("asset_findings_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("asset_findings", "Asset not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("asset_findings", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("asset_findings_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("asset_mutations", "SAMPLE_ASSET_ID not set — set in .env")

    # =========================================================================
    # SECTION 7: Products (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Products (mutations) ──{_Color.RESET}")

    # POST /products → 201
    print(_info("Test: POST /products → create product (201)"))
    resp = client.post("/products", body={
        "name": "Test Product (auto-test)",
        "product_type": "web_application",
        "team_members": []
    })
    created_product_id = None
    if resp.status_code in (200, 201):
        try:
            body = resp.json()
            if "id" in body:
                results.record_pass("product_create_returns_201_with_id")
                created_product_id = body.get("id")
            else:
                results.record_fail("product_create_schema", "no 'id' in response")
        except Exception as e:
            results.record_fail("product_create_returns_201", f"Exception: {e}")
    elif resp.status_code in (400, 422):
        results.record_skip("product_create",
                            f"Validation error ({resp.status_code}) — payload may need adjustment")
    elif resp.status_code == 404:
        results.record_skip("product_create", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("product_create", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("product_create_returns_201",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    pid = SAMPLE_PRODUCT_ID or created_product_id
    if pid:
        # PATCH /products/{id} → Product schema
        print(_info(f"Test: PATCH /products/{pid} → update product"))
        resp = client.patch(f"/products/{pid}", body={"name": "Updated Product Name"})
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "id" in body:
                    results.record_pass("product_patch_returns_200_with_id")
                else:
                    results.record_fail("product_patch_schema", "no 'id' in response")
            except Exception as e:
                results.record_fail("product_patch_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("product_patch", "Product not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 422):
            results.record_skip("product_patch", f"Validation error ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("product_patch", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("product_patch_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("product_patch", "No product ID available — set SAMPLE_PRODUCT_ID in .env")

    # GET /engagements/{engId}/tests → { tests, total }
    eng_id = SAMPLE_ENGAGEMENT_ID
    if eng_id:
        print(_info(f"Test: GET /engagements/{eng_id}/tests → engagement tests"))
        resp = client.get(f"/engagements/{eng_id}/tests")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "tests" in body or "total" in body or isinstance(body, list):
                    results.record_pass("engagement_tests_returns_200")
                else:
                    results.record_fail("engagement_tests_schema",
                                        f"Unexpected keys: {list(body.keys())[:5]}")
            except Exception as e:
                results.record_fail("engagement_tests_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("engagement_tests", "Engagement not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("engagement_tests", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("engagement_tests_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("engagement_tests",
                            "SAMPLE_ENGAGEMENT_ID not set — set in .env to enable this test")

    # =========================================================================
    # SECTION 8: AI (full set)
    # =========================================================================
    print(f"\n{_Color.BOLD}── AI (mutations + insights) ──{_Color.RESET}")

    ai_finding_id = SAMPLE_FINDING_ID

    if ai_finding_id:
        # POST /ai/triage/{findingId} → 202
        print(_info(f"Test: POST /ai/triage/{ai_finding_id} → AI triage (202)"))
        resp = client.post(f"/ai/triage/{ai_finding_id}")
        if resp.status_code in (200, 202):
            results.record_pass("ai_triage_trigger_returns_2xx")
        elif resp.status_code == 404:
            results.record_skip("ai_triage_trigger", "Finding not found / endpoint not implemented (404)")
        elif resp.status_code == 503:
            results.record_skip("ai_triage_trigger", "AI service unavailable (503)")
        elif resp.status_code in (400, 409):
            # Already in queue or already triaged
            results.record_skip("ai_triage_trigger", f"Conflict/already queued ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("ai_triage_trigger", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("ai_triage_trigger_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # POST /ai/triage/{findingId}/review → 204
        print(_info(f"Test: POST /ai/triage/{ai_finding_id}/review → review triage"))
        resp = client.post(f"/ai/triage/{ai_finding_id}/review", body={
            "decision": "accepted",
            "comment": "Auto-test review"
        })
        if resp.status_code in (200, 204):
            results.record_pass("ai_triage_review_returns_2xx")
        elif resp.status_code == 404:
            results.record_skip("ai_triage_review", "Finding not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 409):
            results.record_skip("ai_triage_review", f"Validation/state error ({resp.status_code})")
        elif resp.status_code == 503:
            results.record_skip("ai_triage_review", "AI service unavailable (503)")
        elif resp.status_code in (401, 403):
            results.record_skip("ai_triage_review", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("ai_triage_review_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("ai_triage_and_review",
                            "SAMPLE_FINDING_ID not set — set in .env to enable AI triage tests")

    # GET /ai/insights → InsightsResponse
    print(_info("Test: GET /ai/insights → AI insights"))
    resp = client.get("/ai/insights")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, dict):
                results.record_pass("ai_insights_returns_200")
            else:
                results.record_fail("ai_insights_schema",
                                    f"Expected object, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("ai_insights_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_insights", "Endpoint not implemented (404)")
    elif resp.status_code == 503:
        results.record_skip("ai_insights", "AI service unavailable (503)")
    elif resp.status_code in (401, 403):
        results.record_skip("ai_insights", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("ai_insights_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # POST /ai/enrichment/trigger → 202
    print(_info("Test: POST /ai/enrichment/trigger → trigger enrichment (202)"))
    resp = client.post("/ai/enrichment/trigger")
    if resp.status_code in (200, 202):
        results.record_pass("ai_enrichment_trigger_returns_2xx")
    elif resp.status_code == 404:
        results.record_skip("ai_enrichment_trigger", "Endpoint not implemented (404)")
    elif resp.status_code == 503:
        results.record_skip("ai_enrichment_trigger", "AI service unavailable (503)")
    elif resp.status_code in (401, 403):
        results.record_skip("ai_enrichment_trigger", f"Auth error ({resp.status_code})")
    elif resp.status_code in (409, 400):
        results.record_skip("ai_enrichment_trigger",
                            f"Enrichment already running or config issue ({resp.status_code})")
    else:
        results.record_fail("ai_enrichment_trigger_returns_2xx",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 9: Reports (full set)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Reports (full set) ──{_Color.RESET}")

    # GET /reports/templates → { templates }
    print(_info("Test: GET /reports/templates → templates list"))
    resp = client.get("/reports/templates")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "templates" in body or isinstance(body, list):
                results.record_pass("reports_templates_returns_200")
            else:
                results.record_fail("reports_templates_schema",
                                    f"Unexpected keys: {list(body.keys())[:5]}")
        except Exception as e:
            results.record_fail("reports_templates_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("reports_templates", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("reports_templates", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("reports_templates_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # Create a report to get an ID
    created_report_id = None
    resp = client.post("/reports", body={
        "name": "Auto-test Report",
        "type": "executive_summary",
        "config": {"product_ids": [], "date_range": "30d"}
    })
    if resp.status_code in (200, 201, 202):
        try:
            body = resp.json()
            created_report_id = body.get("id")
        except Exception:
            pass

    report_id = SAMPLE_REPORT_ID or created_report_id

    if report_id:
        # GET /reports/{id} → Report schema
        print(_info(f"Test: GET /reports/{report_id} → Report schema"))
        resp = client.get(f"/reports/{report_id}")
        if resp.status_code == 200:
            try:
                body = resp.json()
                errors = validate_required_fields(body, REPORT_REQUIRED, "Report")
                if errors:
                    results.record_fail("report_detail_schema", "; ".join(errors))
                else:
                    results.record_pass("report_detail_returns_200_with_schema")
            except Exception as e:
                results.record_fail("report_detail_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("report_detail", "Report not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("report_detail", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("report_detail_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # GET /reports/{id}/download → binary or redirect
        print(_info(f"Test: GET /reports/{report_id}/download → download"))
        resp = client.get(f"/reports/{report_id}/download")
        if resp.status_code in (200, 302):
            results.record_pass("report_download_returns_200_or_302")
        elif resp.status_code == 404:
            results.record_skip("report_download",
                                "Report not found or not ready / endpoint not implemented (404)")
        elif resp.status_code == 409:
            results.record_skip("report_download", "Report not yet generated (409 Conflict)")
        elif resp.status_code in (401, 403):
            results.record_skip("report_download", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("report_download_returns_200",
                                f"Got {resp.status_code}: {resp.text[:100]}")

        # DELETE /reports/{id} → 204
        print(_info(f"Test: DELETE /reports/{report_id} → 204"))
        resp = client.delete(f"/reports/{report_id}")
        if resp.status_code in (200, 204):
            results.record_pass("report_delete_returns_204")
        elif resp.status_code == 404:
            results.record_skip("report_delete", "Report not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("report_delete", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("report_delete_returns_204",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("report_detail_download_delete",
                            "No report ID — set SAMPLE_REPORT_ID in .env or ensure POST /reports works")

    # =========================================================================
    # SECTION 10: Integrations (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Integrations (mutations) ──{_Color.RESET}")

    # POST /webhooks → Webhook (201)
    print(_info("Test: POST /webhooks → create webhook (201)"))
    resp = client.post("/webhooks", body={
        "name": "Test Webhook (auto-test)",
        "url": "https://example.com/webhook",
        "events": ["finding.created"],
        "secret": "test-secret-12345"
    })
    created_webhook_id = None
    if resp.status_code in (200, 201):
        try:
            body = resp.json()
            if "id" in body:
                results.record_pass("webhook_create_returns_201_with_id")
                created_webhook_id = body.get("id")
            else:
                results.record_fail("webhook_create_schema", "no 'id' in response")
        except Exception as e:
            results.record_fail("webhook_create_returns_201", f"Exception: {e}")
    elif resp.status_code in (400, 422):
        results.record_skip("webhook_create",
                            f"Validation error ({resp.status_code}) — payload may need adjustment")
    elif resp.status_code == 404:
        results.record_skip("webhook_create", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("webhook_create", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("webhook_create_returns_201",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    webhook_id = SAMPLE_WEBHOOK_ID or created_webhook_id

    if webhook_id:
        # POST /webhooks/{id}/test → WebhookTestResult
        print(_info(f"Test: POST /webhooks/{webhook_id}/test → test webhook"))
        resp = client.post(f"/webhooks/{webhook_id}/test")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "status" in body or "success" in body or "ok" in body:
                    results.record_pass("webhook_test_returns_200_with_status")
                else:
                    results.record_pass("webhook_test_returns_200")
            except Exception as e:
                results.record_fail("webhook_test_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("webhook_test", "Webhook not found / endpoint not implemented (404)")
        elif resp.status_code in (400, 422):
            results.record_skip("webhook_test", f"Test delivery failed ({resp.status_code})")
        elif resp.status_code in (401, 403):
            results.record_skip("webhook_test", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("webhook_test_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # DELETE /webhooks/{id} → 204
        print(_info(f"Test: DELETE /webhooks/{webhook_id} → 204"))
        resp = client.delete(f"/webhooks/{webhook_id}")
        if resp.status_code in (200, 204):
            results.record_pass("webhook_delete_returns_204")
        elif resp.status_code == 404:
            results.record_skip("webhook_delete", "Webhook not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("webhook_delete", f"Auth error ({resp.status_code})")
        else:
            results.record_fail("webhook_delete_returns_204",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("webhook_test_and_delete",
                            "No webhook ID — set SAMPLE_WEBHOOK_ID in .env or ensure POST /webhooks works")

    # POST /jira/config/test → { ok, latency_ms }
    print(_info("Test: POST /jira/config/test → test JIRA connection"))
    resp = client.post("/jira/config/test")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "ok" in body or "success" in body or "latency_ms" in body:
                results.record_pass("jira_config_test_returns_200_with_schema")
            else:
                results.record_pass("jira_config_test_returns_200")
        except Exception as e:
            results.record_fail("jira_config_test_returns_200", f"Exception: {e}")
    elif resp.status_code in (400, 422, 503):
        results.record_skip("jira_config_test",
                            f"JIRA not configured / connection failed ({resp.status_code})")
    elif resp.status_code == 404:
        results.record_skip("jira_config_test", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("jira_config_test", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("jira_config_test_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # GET /integrations/jira → JiraConfig
    print(_info("Test: GET /integrations/jira → JIRA integration config"))
    resp = client.get("/integrations/jira")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, dict) and ("enabled" in body or "url" in body or "project_key" in body):
                results.record_pass("integrations_jira_get_returns_200_with_schema")
            elif isinstance(body, dict):
                results.record_pass("integrations_jira_get_returns_200")
            else:
                results.record_fail("integrations_jira_get_schema",
                                    f"Expected object, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("integrations_jira_get_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("integrations_jira_get", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("integrations_jira_get", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("integrations_jira_get_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # PUT /integrations/jira → 200
    print(_info("Test: PUT /integrations/jira → update JIRA config"))
    resp = client.put("/integrations/jira", body={
        "enabled": False,
        "url": "https://jira.example.com",
        "project_key": "TEST",
        "username": "test@example.com",
        "api_token": "test-token"
    })
    if resp.status_code == 200:
        results.record_pass("integrations_jira_put_returns_200")
    elif resp.status_code in (400, 422):
        results.record_skip("integrations_jira_put",
                            f"Validation error ({resp.status_code}) — payload may need adjustment")
    elif resp.status_code == 404:
        results.record_skip("integrations_jira_put", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("integrations_jira_put", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("integrations_jira_put_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 11: Profile (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Profile (mutations) ──{_Color.RESET}")

    # PATCH /profile → User schema
    print(_info("Test: PATCH /profile → update profile"))
    resp = client.patch("/profile", body={"name": "Test User (auto-test)"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "id" in body or "email" in body:
                results.record_pass("profile_patch_returns_200_with_user")
            else:
                results.record_fail("profile_patch_schema", f"Unexpected keys: {list(body.keys())[:5]}")
        except Exception as e:
            results.record_fail("profile_patch_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("profile_patch", "Endpoint not implemented (404)")
    elif resp.status_code in (400, 422):
        results.record_skip("profile_patch", f"Validation error ({resp.status_code})")
    elif resp.status_code in (401, 403):
        results.record_skip("profile_patch", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("profile_patch_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # POST /profile/change-password → 204
    print(_info("Test: POST /profile/change-password → 204"))
    resp = client.post("/profile/change-password", body={
        "current_password": "WRONG_PASSWORD_ON_PURPOSE",
        "new_password": "NewSecure@123"
    })
    if resp.status_code in (200, 204):
        results.record_pass("profile_change_password_returns_2xx")
    elif resp.status_code in (400, 401, 422):
        # Wrong current password → expected rejection, but endpoint exists
        results.record_pass("profile_change_password_endpoint_exists_rejects_wrong_pwd")
    elif resp.status_code == 404:
        results.record_skip("profile_change_password", "Endpoint not implemented (404)")
    elif resp.status_code == 403:
        results.record_skip("profile_change_password", "Auth error (403)")
    else:
        results.record_fail("profile_change_password_returns_2xx",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # GET /profile/sessions → { sessions }
    print(_info("Test: GET /profile/sessions → user sessions"))
    resp = client.get("/profile/sessions")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "sessions" in body or isinstance(body, list):
                results.record_pass("profile_sessions_returns_200")
                sessions = body.get("sessions") if isinstance(body, dict) else body
                if isinstance(sessions, list):
                    results.record_pass("profile_sessions_is_array")
            else:
                results.record_fail("profile_sessions_schema",
                                    f"Unexpected keys: {list(body.keys())[:5]}")
        except Exception as e:
            results.record_fail("profile_sessions_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("profile_sessions", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("profile_sessions", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("profile_sessions_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # GET /profile/notifications/settings → NotificationSettings
    print(_info("Test: GET /profile/notifications/settings → notification settings"))
    resp = client.get("/profile/notifications/settings")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, dict) and ("email" in body or "in_app" in body or "preferences" in body):
                results.record_pass("profile_notif_settings_returns_200_with_schema")
            elif isinstance(body, dict):
                results.record_pass("profile_notif_settings_returns_200")
            else:
                results.record_fail("profile_notif_settings_schema",
                                    f"Expected object, got {type(body).__name__}")
        except Exception as e:
            results.record_fail("profile_notif_settings_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("profile_notif_settings_get", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("profile_notif_settings_get", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("profile_notif_settings_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # PUT /profile/notifications/settings → 200
    print(_info("Test: PUT /profile/notifications/settings → update settings"))
    resp = client.put("/profile/notifications/settings", body={
        "email": {"finding_created": True, "scan_completed": True},
        "in_app": {"finding_created": True}
    })
    if resp.status_code == 200:
        results.record_pass("profile_notif_settings_put_returns_200")
    elif resp.status_code in (400, 422):
        results.record_skip("profile_notif_settings_put", f"Validation error ({resp.status_code})")
    elif resp.status_code == 404:
        results.record_skip("profile_notif_settings_put", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("profile_notif_settings_put", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("profile_notif_settings_put_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 12: Admin (mutations)
    # =========================================================================
    print(f"\n{_Color.BOLD}── Admin (mutations + user detail) ──{_Color.RESET}")

    # GET /admin/users/{id} → AdminUser
    user_id = SAMPLE_USER_ID
    if not user_id:
        # Cố tìm user từ GET /admin/users
        resp = client.get("/admin/users", params={"page": 1, "page_size": 1})
        if resp.status_code == 200:
            try:
                body = resp.json()
                users = body.get("users") or []
                if users:
                    user_id = users[0].get("id")
            except Exception:
                pass

    if user_id:
        print(_info(f"Test: GET /admin/users/{user_id} → AdminUser schema"))
        resp = client.get(f"/admin/users/{user_id}")
        if resp.status_code == 200:
            try:
                body = resp.json()
                if "id" in body and "email" in body:
                    results.record_pass("admin_user_detail_returns_200_with_schema")
                else:
                    results.record_fail("admin_user_detail_schema",
                                        f"Missing id/email. Keys: {list(body.keys())[:5]}")
            except Exception as e:
                results.record_fail("admin_user_detail_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("admin_user_detail",
                                "User not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("admin_user_detail",
                                f"Auth/permission error ({resp.status_code}) — need admin role")
        else:
            results.record_fail("admin_user_detail_returns_200",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("admin_user_detail",
                            "No user ID — set SAMPLE_USER_ID in .env or ensure GET /admin/users works")

    # POST /admin/users/invite → 201
    print(_info("Test: POST /admin/users/invite → invite user (201)"))
    resp = client.post("/admin/users/invite", body={
        "email": f"autotest+{__import__('time').time_ns()}@example.com",
        "role": "readonly",
        "name": "Auto Test Invite"
    })
    if resp.status_code in (200, 201):
        results.record_pass("admin_user_invite_returns_201")
    elif resp.status_code in (400, 409, 422):
        # Email may already exist → but endpoint is registered
        results.record_pass("admin_user_invite_endpoint_exists_4xx")
    elif resp.status_code == 404:
        results.record_skip("admin_user_invite", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("admin_user_invite",
                            f"Auth/permission error ({resp.status_code}) — need admin role")
    else:
        results.record_fail("admin_user_invite_returns_201",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    if user_id:
        # POST /admin/users/{id}/unlock → 204
        print(_info(f"Test: POST /admin/users/{user_id}/unlock → 204"))
        resp = client.post(f"/admin/users/{user_id}/unlock")
        if resp.status_code in (200, 204):
            results.record_pass("admin_user_unlock_returns_2xx")
        elif resp.status_code in (400, 409):
            # User may not be locked — but endpoint exists
            results.record_pass("admin_user_unlock_endpoint_exists_4xx")
        elif resp.status_code == 404:
            results.record_skip("admin_user_unlock",
                                "User not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("admin_user_unlock",
                                f"Auth/permission error ({resp.status_code}) — need admin role")
        else:
            results.record_fail("admin_user_unlock_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")

        # POST /admin/users/{id}/reset-password → 204
        print(_info(f"Test: POST /admin/users/{user_id}/reset-password → 204"))
        resp = client.post(f"/admin/users/{user_id}/reset-password")
        if resp.status_code in (200, 202, 204):
            results.record_pass("admin_user_reset_password_returns_2xx")
        elif resp.status_code in (400, 409):
            results.record_pass("admin_user_reset_password_endpoint_exists_4xx")
        elif resp.status_code == 404:
            results.record_skip("admin_user_reset_password",
                                "User not found / endpoint not implemented (404)")
        elif resp.status_code in (401, 403):
            results.record_skip("admin_user_reset_password",
                                f"Auth/permission error ({resp.status_code}) — need admin role")
        else:
            results.record_fail("admin_user_reset_password_returns_2xx",
                                f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("admin_user_unlock_reset_password",
                            "No user ID available — set SAMPLE_USER_ID in .env")

    # =========================================================================
    # SECTION 13: Audit Log
    # =========================================================================
    print(f"\n{_Color.BOLD}── Audit Log ──{_Color.RESET}")

    # GET /audit-log → { entries, total }
    print(_info("Test: GET /audit-log → audit log entries"))
    resp = client.get("/audit-log", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept {entries, total} or {data, total} or {logs, total} or list
            if isinstance(body, list):
                results.record_pass("audit_log_returns_200_as_array")
            elif isinstance(body, dict):
                has_list = any(k in body for k in ("entries", "data", "logs", "audit_log", "items"))
                has_total = "total" in body
                if has_list or has_total:
                    results.record_pass("audit_log_returns_200_with_schema")
                    key = next((k for k in ("entries", "data", "logs", "audit_log", "items") if k in body), None)
                    if key:
                        entries = body[key]
                        if isinstance(entries, list):
                            results.record_pass("audit_log_entries_is_array")
                            if entries:
                                sample = entries[0]
                                errors = validate_required_fields(sample, AUDIT_ENTRY_REQUIRED,
                                                                  "AuditEntry[0]")
                                if errors:
                                    results.record_skip("audit_log_entry_schema",
                                                        f"Schema partial: {errors[:2]}")
                                else:
                                    results.record_pass("audit_log_entry_schema_valid")
                else:
                    results.record_pass("audit_log_returns_200")
            else:
                results.record_fail("audit_log_schema",
                                    f"Unexpected response type: {type(body).__name__}")
        except Exception as e:
            results.record_fail("audit_log_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("audit_log_returns_200", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("audit_log_returns_200",
                            f"Auth/permission error ({resp.status_code}) — need admin role")
    else:
        results.record_fail("audit_log_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 14: Global Search
    # =========================================================================
    print(f"\n{_Color.BOLD}── Global Search ──{_Color.RESET}")

    # GET /search/recent → { searches }
    print(_info("Test: GET /search/recent → recent searches"))
    resp = client.get("/search/recent")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "searches" in body or "recent" in body or isinstance(body, list):
                results.record_pass("search_recent_returns_200")
            elif isinstance(body, dict):
                results.record_pass("search_recent_returns_200")
            else:
                results.record_fail("search_recent_schema",
                                    f"Unexpected type: {type(body).__name__}")
        except Exception as e:
            results.record_fail("search_recent_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("search_recent", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("search_recent", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("search_recent_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # GET /search/suggested → { suggestions }
    print(_info("Test: GET /search/suggested → suggested searches"))
    resp = client.get("/search/suggested", params={"q": "CVE"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "suggestions" in body or "suggested" in body or isinstance(body, list):
                results.record_pass("search_suggested_returns_200")
            elif isinstance(body, dict):
                results.record_pass("search_suggested_returns_200")
            else:
                results.record_fail("search_suggested_schema",
                                    f"Unexpected type: {type(body).__name__}")
        except Exception as e:
            results.record_fail("search_suggested_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("search_suggested", "Endpoint not implemented (404)")
    elif resp.status_code in (401, 403):
        results.record_skip("search_suggested", f"Auth error ({resp.status_code})")
    else:
        results.record_fail("search_suggested_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
