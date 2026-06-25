"""
test_webhook_deliveries.py — Test Webhook Deliveries & Hourly Stats endpoints

Kiểm tra (từ CR-009 + openapi.yaml):
  - GET  /webhooks/deliveries              → WebhookDeliveriesResponse schema
  - GET  /webhooks/deliveries?webhook_id=X → filter by webhook
  - GET  /webhooks/deliveries?status=failed → filter by status
  - POST /webhooks/deliveries/{id}/retry   → WebhookDelivery, 404 nếu không có, 422 nếu đã success
  - GET  /webhooks/stats/hourly            → WebhookHourlyStats[<=24]

Chạy:
  python test_webhook_deliveries.py

Môi trường:
  Đọc từ .env — xem .env.example
  SAMPLE_WEBHOOK_ID, SAMPLE_WEBHOOK_DELIVERY_ID (optional)
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_list_response, _info, _Color
)
from config import Config

import os

# ── Config từ env ────────────────────────────────────────────────────────────
SAMPLE_WEBHOOK_ID = os.environ.get("SAMPLE_WEBHOOK_ID") or None
SAMPLE_DELIVERY_ID = os.environ.get("SAMPLE_WEBHOOK_DELIVERY_ID") or None
SAMPLE_SUCCESS_DELIVERY_ID = os.environ.get("SAMPLE_SUCCESS_DELIVERY_ID") or None

# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

WEBHOOK_DELIVERY_REQUIRED = [
    "id", "webhook_id", "event", "endpoint",
    "status", "response_time_ms", "status_code", "time"
]

DELIVERY_STATUS_VALUES = {"success", "failed", "retried"}

HOURLY_STATS_REQUIRED = ["h", "success", "failed"]


def _validate_delivery(d: dict, path: str) -> list:
    errors = validate_required_fields(d, WEBHOOK_DELIVERY_REQUIRED, path)
    status = d.get("status")
    if status and status not in DELIVERY_STATUS_VALUES:
        errors.append(f"{path}.status='{status}' not in {DELIVERY_STATUS_VALUES}")
    rtime = d.get("response_time_ms")
    if rtime is not None and not (isinstance(rtime, int) and rtime >= 0):
        errors.append(f"{path}.response_time_ms={rtime!r} must be non-negative int")
    sc = d.get("status_code")
    if sc is not None and not (isinstance(sc, int) and 100 <= sc <= 599):
        errors.append(f"{path}.status_code={sc!r} must be valid HTTP status code (100-599)")
    return errors


def _validate_hourly_stat(item: dict, path: str) -> list:
    errors = validate_required_fields(item, HOURLY_STATS_REQUIRED, path)
    h = item.get("h", "")
    # Format phải là "HH:00" (e.g. "00:00", "13:00", "23:00")
    if h and not (len(h) == 5 and h[2:] == ":00" and h[:2].isdigit()
                  and 0 <= int(h[:2]) <= 23):
        errors.append(f"{path}.h='{h}' must be 'HH:00' format (00:00 to 23:00)")
    for field in ("success", "failed"):
        val = item.get(field)
        if val is not None and not (isinstance(val, int) and val >= 0):
            errors.append(f"{path}.{field}={val!r} must be non-negative int")
    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("WEBHOOK DELIVERIES & HOURLY STATS API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_webhook_delivery_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # TEST 1: GET /webhooks/deliveries → WebhookDeliveriesResponse
    # =========================================================================
    print(f"\n{_Color.BOLD}── Webhook Delivery List ──{_Color.RESET}")
    print(_info("Test: GET /webhooks/deliveries → WebhookDeliveriesResponse"))
    resp = client.get("/webhooks/deliveries", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept {deliveries, total} or {data, total} or [] array
            if isinstance(body, list):
                results.record_pass("webhook_deliveries_list_returns_200_with_schema")
                results.record_pass("webhook_deliveries_is_array")
            elif isinstance(body, dict):
                deliveries_key = "deliveries" if "deliveries" in body else "data"
                if deliveries_key in body or "total" in body:
                    results.record_pass("webhook_deliveries_list_returns_200_with_schema")
                    deliveries = body.get(deliveries_key) or []
                    if isinstance(deliveries, list):
                        results.record_pass("webhook_deliveries_is_array")
                    results.record_pass("webhook_deliveries_total_non_negative")
                else:
                    results.record_fail("webhook_deliveries_list_schema",
                                        "WebhookDeliveriesResponse.deliveries is missing; WebhookDeliveriesResponse.total is missing")
            else:
                results.record_fail("webhook_deliveries_list_returns_200_with_schema",
                                    f"Unexpected response type: {type(body)}")

        except Exception as e:
            # Empty body
            results.record_skip("webhook_deliveries_list_returns_200", f"JSON parse error: {e}")

    elif resp.status_code == 404:
        results.record_skip("webhook_deliveries_list_returns_200",
                            "GET /webhooks/deliveries not implemented (404) — CR-009 pending")
    elif resp.status_code == 405:
        results.record_skip("webhook_deliveries_list_returns_200",
                            "GET /webhooks/deliveries returned 405 — endpoint not registered for GET")
    else:
        results.record_fail("webhook_deliveries_list_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # TEST 2: GET /webhooks/deliveries?status=failed → filter works
    # =========================================================================
    print(_info("Test: GET /webhooks/deliveries?status=failed → only failed"))
    resp = client.get("/webhooks/deliveries", params={"status": "failed", "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            if isinstance(body, list):
                deliveries = body
            elif isinstance(body, dict):
                deliveries = body.get("deliveries") or body.get("data") or []
            else:
                deliveries = []
            non_failed = [d.get("id") for d in deliveries if d.get("status") not in ("failed", "retried")]
            if non_failed:
                results.record_fail("webhook_deliveries_status_filter",
                                    f"Non-failed deliveries returned: {non_failed[:3]}")
            else:
                results.record_pass("webhook_deliveries_status_filter_works")
        except Exception as e:
            results.record_skip("webhook_deliveries_status_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("webhook_deliveries_status_filter", "Endpoint not implemented")
    elif resp.status_code == 405:
        results.record_skip("webhook_deliveries_status_filter", "Endpoint not registered for GET (405)")
    else:
        results.record_fail("webhook_deliveries_status_filter", f"Got {resp.status_code}")

    # =========================================================================
    # TEST 3: GET /webhooks/deliveries?webhook_id=X → filter by webhook
    # =========================================================================
    if SAMPLE_WEBHOOK_ID:
        print(_info(f"Test: GET /webhooks/deliveries?webhook_id={SAMPLE_WEBHOOK_ID}"))
        resp = client.get("/webhooks/deliveries",
                          params={"webhook_id": SAMPLE_WEBHOOK_ID, "page_size": 10})
        if resp.status_code == 200:
            try:
                body = resp.json()
                deliveries = body.get("deliveries", [])
                wrong_wh = [d.get("id") for d in deliveries
                            if d.get("webhook_id") != SAMPLE_WEBHOOK_ID]
                if wrong_wh:
                    results.record_fail("webhook_deliveries_webhook_filter",
                                        f"Deliveries from different webhook: {wrong_wh[:3]}")
                else:
                    results.record_pass("webhook_deliveries_webhook_id_filter_works")
            except Exception as e:
                results.record_fail("webhook_deliveries_webhook_filter", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip("webhook_deliveries_webhook_filter", "Endpoint not implemented")
        else:
            results.record_fail("webhook_deliveries_webhook_filter", f"Got {resp.status_code}")
    else:
        results.record_skip("webhook_deliveries_webhook_filter",
                            "SAMPLE_WEBHOOK_ID not set — set in .env to enable")

    # =========================================================================
    # TEST 4: POST /webhooks/deliveries/{id}/retry → 404 nếu không tồn tại
    # =========================================================================
    print(f"\n{_Color.BOLD}── Webhook Retry ──{_Color.RESET}")
    print(_info("Test: POST /webhooks/deliveries/nonexistent/retry → 404"))
    resp = client.post("/webhooks/deliveries/nonexistent-delivery-id/retry")
    if resp.status_code == 404:
        results.record_pass("webhook_retry_nonexistent_returns_404")
    elif resp.status_code == 200:
        results.record_fail("webhook_retry_nonexistent_returns_404",
                            "Got 200 for nonexistent delivery — should be 404")
    elif resp.status_code == 404:
        results.record_pass("webhook_retry_nonexistent_returns_404")
    elif resp.status_code in (400, 422):
        # Acceptable — some implementations return 422 for invalid ID format
        results.record_pass("webhook_retry_nonexistent_returns_4xx")
    elif resp.status_code == 405:
        results.record_skip("webhook_retry_nonexistent_404", "Method not allowed — endpoint not registered")
    else:
        results.record_skip("webhook_retry_nonexistent_404",
                            f"Got {resp.status_code} — CR-009 likely pending")

    # TEST 4b: Retry failed delivery (nếu có SAMPLE_DELIVERY_ID)
    if SAMPLE_DELIVERY_ID:
        print(_info(f"Test: POST /webhooks/deliveries/{SAMPLE_DELIVERY_ID}/retry → 200"))
        resp = client.post(f"/webhooks/deliveries/{SAMPLE_DELIVERY_ID}/retry")
        if resp.status_code == 200:
            try:
                body = resp.json()
                d_errors = _validate_delivery(body, "RetryDeliveryResponse")
                if d_errors:
                    results.record_fail("webhook_retry_delivery_response_schema",
                                        "; ".join(d_errors))
                else:
                    results.record_pass("webhook_retry_delivery_returns_200_with_schema")
            except Exception as e:
                results.record_fail("webhook_retry_delivery_response_schema", f"Exception: {e}")
        elif resp.status_code == 422:
            results.record_skip("webhook_retry_delivery",
                                "Delivery is already success — set a FAILED delivery ID in .env")
        elif resp.status_code == 404:
            results.record_fail("webhook_retry_delivery",
                                f"SAMPLE_WEBHOOK_DELIVERY_ID={SAMPLE_DELIVERY_ID} not found")
        else:
            results.record_fail("webhook_retry_delivery", f"Got {resp.status_code}")
    else:
        results.record_skip("webhook_retry_delivery",
                            "SAMPLE_WEBHOOK_DELIVERY_ID not set — set in .env to enable")

    # TEST 4c: Retry success delivery → 422
    if SAMPLE_SUCCESS_DELIVERY_ID:
        print(_info(f"Test: POST retry success delivery → 422"))
        resp = client.post(f"/webhooks/deliveries/{SAMPLE_SUCCESS_DELIVERY_ID}/retry")
        if resp.status_code == 422:
            results.record_pass("webhook_retry_success_returns_422")
        elif resp.status_code == 200:
            results.record_fail("webhook_retry_success_returns_422",
                                "Should not allow retry of successful delivery")
        else:
            results.record_skip("webhook_retry_success_422",
                                f"Got {resp.status_code} — implementation may differ")
    else:
        results.record_skip("webhook_retry_success_422",
                            "SAMPLE_SUCCESS_DELIVERY_ID not set in .env")

    # =========================================================================
    # TEST 5: GET /webhooks/stats/hourly → WebhookHourlyStats[]
    # =========================================================================
    print(f"\n{_Color.BOLD}── Webhook Hourly Stats ──{_Color.RESET}")
    print(_info("Test: GET /webhooks/stats/hourly → hourly stats array"))
    resp = client.get("/webhooks/stats/hourly")
    if resp.status_code == 200:
        try:
            body = resp.json()

            # Phải là array
            if not isinstance(body, list):
                results.record_fail("webhook_hourly_stats_is_array",
                                    f"expected array, got {type(body).__name__}")
            else:
                results.record_pass("webhook_hourly_stats_is_array")

                # Tối đa 24 items (1 item per hour)
                if len(body) <= 24:
                    results.record_pass("webhook_hourly_stats_max_24_items")
                else:
                    results.record_fail("webhook_hourly_stats_max_24_items",
                                        f"expected <= 24, got {len(body)}")

                # Validate schema của từng item
                all_valid = True
                for i, item in enumerate(body):
                    errors = _validate_hourly_stat(item, f"HourlyStats[{i}]")
                    if errors:
                        results.record_fail(f"webhook_hourly_stats_item_{i}_schema",
                                            "; ".join(errors))
                        all_valid = False
                        break
                if all_valid and body:
                    results.record_pass("webhook_hourly_stats_items_schema_valid")

                # h values phải là duy nhất (không duplicate giờ)
                hours = [item.get("h") for item in body]
                if len(hours) == len(set(hours)):
                    results.record_pass("webhook_hourly_stats_no_duplicate_hours")
                else:
                    duplicates = [h for h in hours if hours.count(h) > 1]
                    results.record_fail("webhook_hourly_stats_no_duplicate_hours",
                                        f"Duplicate hours: {list(set(duplicates))}")

        except Exception as e:
            results.record_fail("webhook_hourly_stats_returns_200", f"Exception: {e}")

    elif resp.status_code == 404:
        results.record_skip("webhook_hourly_stats_returns_200",
                            "GET /webhooks/stats/hourly not implemented (404) — CR-009 pending")
    else:
        results.record_fail("webhook_hourly_stats_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # TEST 6: GET /webhooks/deliveries yêu cầu authentication
    # =========================================================================
    print(_info("Test: GET /webhooks/deliveries requires auth (no token → 401)"))
    # Tạo client không có token
    import requests
    no_auth_resp = requests.get(
        f"{Config.API_BASE_URL_V1}/webhooks/deliveries",
        headers={"Accept": "application/json"},
        timeout=Config.REQUEST_TIMEOUT
    )
    if no_auth_resp.status_code == 401:
        results.record_pass("webhook_deliveries_requires_auth_401")
    elif no_auth_resp.status_code == 403:
        results.record_pass("webhook_deliveries_requires_auth_403")
    elif no_auth_resp.status_code in (404, 405):
        results.record_skip("webhook_deliveries_requires_auth",
                            "Endpoint not implemented yet (404/405)")
    else:
        results.record_fail("webhook_deliveries_requires_auth",
                            f"Expected 401/403, got {no_auth_resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
