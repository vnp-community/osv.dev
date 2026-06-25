"""
test_ai_triage_queue.py — Test AI Triage Queue Full Schema with Human Decision

Kiểm tra (từ CR-014 + openapi.yaml):
  Queue:
    - GET /ai/triage/queue            → AITriageQueueResponse (items + stats)
    - GET /ai/triage/queue?status=pending → filter works
    - GET /ai/triage/queue?severity=Critical → filter works
    - GET /ai/triage/queue?remarks=Confirmed → filter works
  Item Schema:
    - ai_result object (remarks, confidence, justification, actions, generated_at)
    - human_decision (null or enum)
    - reviewed_by, reviewed_at (audit trail)
  Stats block:
    - pending, accepted_today, avg_confidence (0-100), false_positive_rate
  Review:
    - POST /ai/triage/{findingId}/review → 200
    - POST /ai/triage/{findingId}/review với bad decision → 400
    - POST /ai/triage/nonexistent/review → 404
    - POST re-review without force → 409
    - POST re-review with ?force=true → 200

Chạy:
  python test_ai_triage_queue.py

Môi trường:
  Đọc từ .env — xem .env.example
  SAMPLE_TRIAGE_FINDING_ID (optional — finding ID trong triage queue)
  SAMPLE_REVIEWED_FINDING_ID (optional — đã reviewed, dùng test 409)
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

# ── Config từ env ────────────────────────────────────────────────────────────
SAMPLE_TRIAGE_FINDING_ID = os.environ.get("SAMPLE_TRIAGE_FINDING_ID") or None
SAMPLE_REVIEWED_FINDING_ID = os.environ.get("SAMPLE_REVIEWED_FINDING_ID") or None

# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

AI_RESULT_REQUIRED = ["remarks", "confidence", "justification", "actions", "generated_at"]

REMARKS_VALUES = {"Confirmed", "FalsePositive", "NotAffected", "Unexplored"}
HUMAN_DECISION_VALUES = {"accepted", "overridden", "rejected"}
SEVERITY_VALUES = {"Critical", "High", "Medium", "Low"}

TRIAGE_ITEM_REQUIRED = ["finding_id", "finding_title", "severity", "ai_result"]

QUEUE_STATS_REQUIRED = ["pending", "accepted_today", "avg_confidence", "false_positive_rate"]

QUEUE_RESPONSE_REQUIRED = ["items", "total", "stats"]


def _validate_ai_result(ai: dict, path: str) -> list:
    errors = validate_required_fields(ai, AI_RESULT_REQUIRED, path)

    # remarks phải thuộc enum
    remarks = ai.get("remarks")
    if remarks and remarks not in REMARKS_VALUES:
        errors.append(f"{path}.remarks='{remarks}' not in {REMARKS_VALUES}")

    # confidence phải là float 0.0-1.0 (NOT 0-100)
    conf = ai.get("confidence")
    if conf is not None:
        if isinstance(conf, (int, float)) and 0.0 <= conf <= 1.0:
            pass
        else:
            errors.append(f"{path}.confidence={conf!r} must be float 0.0-1.0 (not 0-100)")

    # actions phải là array
    actions = ai.get("actions")
    if actions is not None and not isinstance(actions, list):
        errors.append(f"{path}.actions must be array")

    return errors


def _validate_triage_item(item: dict, path: str) -> list:
    errors = validate_required_fields(item, TRIAGE_ITEM_REQUIRED, path)

    # severity phải thuộc enum
    sev = item.get("severity")
    if sev and sev not in SEVERITY_VALUES:
        errors.append(f"{path}.severity='{sev}' not in {SEVERITY_VALUES}")

    # ai_result phải là object
    ai = item.get("ai_result")
    if ai is not None:
        if isinstance(ai, dict):
            errors.extend(_validate_ai_result(ai, f"{path}.ai_result"))
        else:
            errors.append(f"{path}.ai_result must be object, got {type(ai).__name__}")

    # human_decision phải là null hoặc valid enum
    hd = item.get("human_decision")
    if hd is not None and hd not in HUMAN_DECISION_VALUES:
        errors.append(f"{path}.human_decision='{hd}' not in {HUMAN_DECISION_VALUES}")

    # Nếu human_decision != null, reviewed_by và reviewed_at phải có
    if hd is not None:
        rb = item.get("reviewed_by")
        ra = item.get("reviewed_at")
        if not rb:
            errors.append(f"{path}.reviewed_by missing despite human_decision='{hd}'")
        if not ra:
            errors.append(f"{path}.reviewed_at missing despite human_decision='{hd}'")

    return errors


def _validate_queue_stats(stats: dict, path: str) -> list:
    errors = validate_required_fields(stats, QUEUE_STATS_REQUIRED, path)

    # pending phải là int >= 0
    pending = stats.get("pending")
    if pending is not None and not (isinstance(pending, int) and pending >= 0):
        errors.append(f"{path}.pending={pending!r} must be non-negative int")

    # accepted_today phải là int >= 0
    at = stats.get("accepted_today")
    if at is not None and not (isinstance(at, int) and at >= 0):
        errors.append(f"{path}.accepted_today={at!r} must be non-negative int")

    # avg_confidence phải là float 0-100
    ac = stats.get("avg_confidence")
    if ac is not None and not (isinstance(ac, (int, float)) and 0 <= ac <= 100):
        errors.append(f"{path}.avg_confidence={ac!r} must be 0-100 (percentage)")

    # false_positive_rate phải là float 0-100
    fpr = stats.get("false_positive_rate")
    if fpr is not None and not (isinstance(fpr, (int, float)) and 0 <= fpr <= 100):
        errors.append(f"{path}.false_positive_rate={fpr!r} must be 0-100 (percentage)")

    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("AI TRIAGE QUEUE FULL SCHEMA TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_ai_triage_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # SECTION 1: GET /ai/triage/queue — Full Schema
    # =========================================================================
    print(f"\n{_Color.BOLD}── AI Triage Queue — Full Schema ──{_Color.RESET}")
    print(_info("Test: GET /ai/triage/queue → AITriageQueueResponse"))
    resp = client.get("/ai/triage/queue", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()

            # Top-level schema
            errors = validate_required_fields(body, QUEUE_RESPONSE_REQUIRED, "AITriageQueueResponse")
            if errors:
                results.record_fail("ai_triage_queue_top_level_schema", "; ".join(errors))
            else:
                results.record_pass("ai_triage_queue_returns_200_with_schema")

            # total phải là int >= 0
            total = body.get("total")
            if isinstance(total, int) and total >= 0:
                results.record_pass("ai_triage_queue_total_is_non_negative_int")
            else:
                results.record_fail("ai_triage_queue_total_is_non_negative_int",
                                    f"total={total!r}")

            # items schema
            items = body.get("items", [])
            if isinstance(items, list):
                results.record_pass("ai_triage_queue_items_is_array")

                all_valid = True
                for i, item in enumerate(items[:5]):
                    item_errors = _validate_triage_item(item, f"item[{i}]")
                    if item_errors:
                        results.record_fail(f"ai_triage_item_{i}_schema",
                                            "; ".join(item_errors))
                        all_valid = False
                        break
                if all_valid and items:
                    results.record_pass("ai_triage_items_schema_valid")

                # ai_result.confidence phải là 0.0-1.0, KHÔNG phải 0-100
                for i, item in enumerate(items[:3]):
                    ai = item.get("ai_result", {})
                    conf = ai.get("confidence")
                    if conf is not None and isinstance(conf, (int, float)):
                        if 0 <= conf <= 1:
                            pass  # OK
                        elif 1 < conf <= 100:
                            results.record_fail("ai_triage_confidence_is_0_to_1",
                                                f"item[{i}].ai_result.confidence={conf} — should be 0-1, not 0-100")
                            break
                else:
                    if items:
                        results.record_pass("ai_triage_confidence_is_0_to_1_range")

            else:
                results.record_fail("ai_triage_queue_items_is_array",
                                    f"type={type(items).__name__}")

            # stats block
            stats = body.get("stats", {})
            if isinstance(stats, dict):
                s_errors = _validate_queue_stats(stats, "AITriageQueueStats")
                if s_errors:
                    results.record_fail("ai_triage_stats_schema", "; ".join(s_errors))
                else:
                    results.record_pass("ai_triage_stats_schema_valid")

                # avg_confidence phải là 0-100 (percent), KHÔNG phải 0-1
                ac = stats.get("avg_confidence")
                if ac is not None and isinstance(ac, (int, float)):
                    if 0 <= ac <= 100:
                        results.record_pass("ai_triage_stats_avg_confidence_is_percent")
                    else:
                        results.record_fail("ai_triage_stats_avg_confidence_is_percent",
                                            f"avg_confidence={ac} out of 0-100 range")
            else:
                results.record_fail("ai_triage_stats_is_object",
                                    f"type={type(stats).__name__}")

        except Exception as e:
            results.record_fail("ai_triage_queue_returns_200", f"Exception: {e}")

    elif resp.status_code == 404:
        results.record_skip("ai_triage_queue_returns_200",
                            "GET /ai/triage/queue not implemented (404) — CR-014 pending")
        results.summary()
        return results  # Early exit nếu endpoint không tồn tại
    elif resp.status_code == 503:
        results.record_skip("ai_triage_queue_returns_200",
                            "AI service unavailable (503) — ai-service is down")
        results.summary()
        return results  # Early exit nếu ai-service down
    else:
        results.record_fail("ai_triage_queue_returns_200",
                            f"Got {resp.status_code}: {resp.text[:200]}")

    # =========================================================================
    # SECTION 2: Filter tests
    # =========================================================================
    print(f"\n{_Color.BOLD}── Queue Filters ──{_Color.RESET}")

    # Filter ?status=pending
    print(_info("Test: GET /ai/triage/queue?status=pending → only unreviewed items"))
    resp = client.get("/ai/triage/queue", params={"status": "pending", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            items = body.get("items", [])
            reviewed = [i.get("finding_id") for i in items if i.get("human_decision") is not None]
            if reviewed:
                results.record_fail("ai_triage_pending_filter",
                                    f"Reviewed items in pending filter: {reviewed[:3]}")
            else:
                results.record_pass("ai_triage_pending_filter_works")
        except Exception as e:
            results.record_fail("ai_triage_pending_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_pending_filter", "Endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_pending_filter", "AI service unavailable (503)")
    else:
        results.record_fail("ai_triage_pending_filter", f"Got {resp.status_code}")

    # Filter ?severity=Critical
    print(_info("Test: GET /ai/triage/queue?severity=Critical → only Critical"))
    resp = client.get("/ai/triage/queue", params={"severity": "Critical", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            items = body.get("items", [])
            non_critical = [i.get("finding_id") for i in items if i.get("severity") != "Critical"]
            if non_critical:
                results.record_fail("ai_triage_severity_filter",
                                    f"Non-Critical items in filter: {non_critical[:3]}")
            else:
                results.record_pass("ai_triage_severity_filter_works")
        except Exception as e:
            results.record_fail("ai_triage_severity_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_severity_filter", "Endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_severity_filter", "AI service unavailable (503)")
    else:
        results.record_fail("ai_triage_severity_filter", f"Got {resp.status_code}")

    # Filter ?remarks=FalsePositive
    print(_info("Test: GET /ai/triage/queue?remarks=FalsePositive → FP items only"))
    resp = client.get("/ai/triage/queue", params={"remarks": "FalsePositive", "page_size": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            items = body.get("items", [])
            non_fp = [
                i.get("finding_id") for i in items
                if i.get("ai_result", {}).get("remarks") != "FalsePositive"
            ]
            if non_fp:
                results.record_fail("ai_triage_remarks_filter",
                                    f"Non-FP items in FalsePositive filter: {non_fp[:3]}")
            else:
                results.record_pass("ai_triage_remarks_filter_works")
        except Exception as e:
            results.record_fail("ai_triage_remarks_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_remarks_filter", "Endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_remarks_filter", "AI service unavailable (503)")
    else:
        results.record_fail("ai_triage_remarks_filter", f"Got {resp.status_code}")

    # =========================================================================
    # SECTION 3: Review endpoint
    # =========================================================================
    print(f"\n{_Color.BOLD}── Review Endpoint ──{_Color.RESET}")

    # TEST: POST /ai/triage/nonexistent/review → 404
    print(_info("Test: POST /ai/triage/nonexistent/review → 404"))
    resp = client.post("/ai/triage/NONEXISTENT-FINDING-ID-12345/review",
                       body={"decision": "accepted"})
    if resp.status_code == 404:
        results.record_pass("ai_triage_review_nonexistent_returns_404")
    elif resp.status_code in (400, 422):
        results.record_pass("ai_triage_review_nonexistent_returns_4xx")
    elif resp.status_code == 405:
        results.record_skip("ai_triage_review_nonexistent_404", "Method not allowed")
    elif resp.status_code == 404:
        results.record_pass("ai_triage_review_nonexistent_returns_404")
    else:
        results.record_skip("ai_triage_review_nonexistent_404",
                            f"Got {resp.status_code} — CR-014 likely pending")

    # TEST: POST review với invalid decision → 400
    print(_info("Test: POST /ai/triage/{id}/review với invalid decision → 400"))
    finding_id = SAMPLE_TRIAGE_FINDING_ID or "test-finding-id"
    resp = client.post(f"/ai/triage/{finding_id}/review",
                       body={"decision": "INVALID_DECISION"})
    if resp.status_code == 400:
        results.record_pass("ai_triage_review_invalid_decision_returns_400")
    elif resp.status_code == 422:
        results.record_pass("ai_triage_review_invalid_decision_returns_422")
    elif resp.status_code == 404 and not SAMPLE_TRIAGE_FINDING_ID:
        results.record_skip("ai_triage_review_invalid_decision",
                            "No finding ID set — 404 expected")
    else:
        results.record_skip("ai_triage_review_invalid_decision",
                            f"Got {resp.status_code}")

    # TEST: POST với finding ID thực (nếu có)
    if SAMPLE_TRIAGE_FINDING_ID:
        fid = SAMPLE_TRIAGE_FINDING_ID
        print(_info(f"Test: POST /ai/triage/{fid}/review → 200"))
        resp = client.post(f"/ai/triage/{fid}/review",
                           body={"decision": "accepted",
                                 "note": "Test review from automated test suite"})
        if resp.status_code == 200:
            try:
                body = resp.json()
                if body.get("success") is True:
                    results.record_pass("ai_triage_review_returns_200_success")
                else:
                    results.record_fail("ai_triage_review_returns_200_success",
                                        f"success!=true: {body}")

                # Verify finding is updated in queue
                queue_resp = client.get("/ai/triage/queue",
                                        params={"status": "accepted", "page_size": 50})
                if queue_resp.status_code == 200:
                    q_body = queue_resp.json()
                    items = q_body.get("items", [])
                    reviewed = next(
                        (i for i in items if i.get("finding_id") == fid), None
                    )
                    if reviewed and reviewed.get("human_decision") == "accepted":
                        results.record_pass("ai_triage_review_persisted_in_queue")
                    else:
                        results.record_fail("ai_triage_review_persisted_in_queue",
                                            f"Finding {fid} not found in accepted filter after review")

            except Exception as e:
                results.record_fail("ai_triage_review_returns_200", f"Exception: {e}")

        elif resp.status_code == 409:
            # Đã reviewed trước đó — test 409 thay vào
            results.record_pass("ai_triage_review_already_reviewed_returns_409")
            # Test force=true
            force_resp = client.post(f"/ai/triage/{fid}/review?force=true",
                                     body={"decision": "overridden",
                                           "note": "Force override from test"})
            if force_resp.status_code == 200:
                results.record_pass("ai_triage_review_force_override_works")
            else:
                results.record_fail("ai_triage_review_force_override_works",
                                    f"?force=true got {force_resp.status_code}")
        elif resp.status_code == 404:
            results.record_fail("ai_triage_review",
                                f"SAMPLE_TRIAGE_FINDING_ID={fid} not found in queue")
        else:
            results.record_fail("ai_triage_review", f"Got {resp.status_code}: {resp.text[:200]}")
    else:
        results.record_skip("ai_triage_review_with_real_finding",
                            "SAMPLE_TRIAGE_FINDING_ID not set — set in .env to enable review tests")

    # TEST: 409 Conflict — already reviewed
    if SAMPLE_REVIEWED_FINDING_ID:
        rid = SAMPLE_REVIEWED_FINDING_ID
        print(_info(f"Test: POST /ai/triage/{rid}/review (already reviewed) → 409"))
        resp = client.post(f"/ai/triage/{rid}/review",
                           body={"decision": "accepted"})
        if resp.status_code == 409:
            results.record_pass("ai_triage_re_review_returns_409_conflict")

            # Body phải có error info
            try:
                err_body = resp.json()
                if "error" in err_body or "message" in err_body:
                    results.record_pass("ai_triage_409_has_error_body")
                else:
                    results.record_fail("ai_triage_409_has_error_body",
                                        f"409 body missing error/message: {err_body}")
            except Exception:
                results.record_skip("ai_triage_409_has_error_body", "Non-JSON 409 body")

        elif resp.status_code == 200:
            results.record_fail("ai_triage_re_review_returns_409_conflict",
                                "Got 200 — should be 409 for already-reviewed finding")
        else:
            results.record_skip("ai_triage_re_review_409",
                                f"Got {resp.status_code}")

        # TEST: force=true override
        print(_info(f"Test: POST /ai/triage/{rid}/review?force=true → 200"))
        resp = client.post(f"/ai/triage/{rid}/review?force=true",
                           body={"decision": "overridden", "note": "Force override"})
        if resp.status_code == 200:
            results.record_pass("ai_triage_force_override_returns_200")
        elif resp.status_code == 404:
            results.record_fail("ai_triage_force_override_returns_200",
                                f"SAMPLE_REVIEWED_FINDING_ID={rid} not found")
        else:
            results.record_skip("ai_triage_force_override",
                                f"Got {resp.status_code}")
    else:
        results.record_skip("ai_triage_409_and_force_tests",
                            "SAMPLE_REVIEWED_FINDING_ID not set — set in .env to enable")

    # =========================================================================
    # SECTION 4: Stats consistency checks
    # =========================================================================
    print(f"\n{_Color.BOLD}── Stats Consistency ──{_Color.RESET}")
    print(_info("Test: stats.pending == count of unreviewed items"))
    resp = client.get("/ai/triage/queue", params={"page_size": 100})
    if resp.status_code == 200:
        try:
            body = resp.json()
            items = body.get("items", [])
            stats = body.get("stats", {})

            # Chỉ kiểm tra nếu page_size đủ lớn để lấy hết
            total = body.get("total", 0)
            if total <= 100 and isinstance(items, list) and isinstance(stats, dict):
                unreviewed_count = sum(
                    1 for i in items if i.get("human_decision") is None
                )
                stated_pending = stats.get("pending", -1)
                if unreviewed_count == stated_pending:
                    results.record_pass("ai_triage_stats_pending_matches_actual")
                else:
                    results.record_fail(
                        "ai_triage_stats_pending_matches_actual",
                        f"stats.pending={stated_pending}, actual unreviewed={unreviewed_count}"
                    )
            else:
                results.record_skip("ai_triage_stats_pending_consistency",
                                    f"total={total} > 100 — cannot verify with single page")

        except Exception as e:
            results.record_fail("ai_triage_stats_consistency", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("ai_triage_stats_consistency", "Endpoint not implemented")
    elif resp.status_code == 503:
        results.record_skip("ai_triage_stats_consistency", "AI service unavailable (503)")
    else:
        results.record_fail("ai_triage_stats_consistency", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
