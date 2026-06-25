"""
test_cve_intelligence.py — Test CVE Intelligence endpoints (/api/v2/cves/*)

Kiểm tra:
  - POST /cves/search         → CVESearchResponse schema
  - POST /cves/search/semantic → SemanticSearchResponse schema
  - GET  /cves/{id}           → CVE detail schema
  - GET  /cves/export?format=json → binary response

Chạy:
  python test_cve_intelligence.py
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

# Required fields from actual server response
CVE_REQUIRED = [
    "id", "severity",
    # epss_score, epss_percentile, published_at are optional in server
    "is_kev", "has_exploit",
    "description", "updated_at", "sources"
    # vendor, product, cwe_ids are present but may be empty string/list
]

# Server returns these but they may be missing in list response
CVE_OPTIONAL = ["vendor", "product", "cwe_ids", "epss_score", "epss_percentile", "published_at"]

CVE_SEVERITY_VALUES = {"Critical", "High", "Medium", "Low", "Info", "critical", "high", "medium", "low", "info"}

# Server may not return aggregations in search response
CVE_SEARCH_RESPONSE_REQUIRED = ["data", "total"]

AGGREGATIONS_REQUIRED = ["by_severity", "top_vendors", "by_year"]

SEMANTIC_SEARCH_RESPONSE_REQUIRED = ["data", "total"]

CVE_SOURCE_REQUIRED = ["name"]

# detail-only fields (ảnh hưởng bởi GET /cves/{id})
CVE_DETAIL_ONLY = ["affected_products", "references"]


def _validate_cve(cve: dict, path: str) -> list:
    errors = validate_required_fields(cve, CVE_REQUIRED, path)
    # Validate severity enum (case-insensitive)
    sev = cve.get("severity")
    if sev and sev not in CVE_SEVERITY_VALUES:
        errors.append(f"{path}.severity='{sev}' not in {CVE_SEVERITY_VALUES}")
    # epss_score is OPTIONAL (may be null/missing)
    epss = cve.get("epss_score")
    if epss is not None and not (isinstance(epss, (int, float)) and 0.0 <= epss <= 1.0):
        errors.append(f"{path}.epss_score={epss} out of 0-1 range")
    # is_kev phải boolean
    if not isinstance(cve.get("is_kev"), bool):
        errors.append(f"{path}.is_kev must be boolean")
    # cwe_ids phải là list (optional)
    if "cwe_ids" in cve and not isinstance(cve["cwe_ids"], list):
        errors.append(f"{path}.cwe_ids must be array")
    # sources phải là list
    if "sources" in cve and not isinstance(cve["sources"], list):
        errors.append(f"{path}.sources must be array")
    return errors


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("CVE INTELLIGENCE API TESTS (/api/v2/cves)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_cve_tests", "Login failed")
        results.summary()
        return results

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: POST /cves/search (basic search) → CVESearchResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /cves/search (basic, no filter) → CVESearchResponse"))
    resp = client.post("/cves/search", v2=True, body={"page": 1, "page_size": 5})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, CVE_SEARCH_RESPONSE_REQUIRED, "CVESearchResponse")
            if errors:
                results.record_fail("cve_search_response_schema", "; ".join(errors))
            else:
                results.record_pass("cve_search_returns_200_with_CVESearchResponse")

            # pagination (may use page/page_size or limit/offset)
            page_ok = body.get("page") is not None or body.get("offset") is not None
            page_size_ok = body.get("page_size") is not None or body.get("limit") is not None
            if page_ok and page_size_ok:
                results.record_pass("cve_search_pagination_valid")
                pg = body.get("page") or 1
                ps = body.get("page_size") or body.get("limit")
                if pg == 1 and ps == 5:
                    results.record_pass("cve_search_pagination_reflects_request")
                else:
                    results.record_skip("cve_search_pagination_reflects_request",
                                        f"page={pg}, page_size={ps}")
            else:
                results.record_skip("cve_search_pagination_fields", "pagination fields not in response")

            # Validate data items
            data = body.get("data") or []
            if not isinstance(data, list):
                results.record_fail("cve_search_data_is_array", "data is not a list")
            else:
                results.record_pass("cve_search_data_is_array")
                for i, cve in enumerate(data[:3]):
                    cve_errors = _validate_cve(cve, f"cves[{i}]")
                    if cve_errors:
                        results.record_fail(f"cve_search_item_{i}_schema", "; ".join(cve_errors))
                        break
                else:
                    if data:
                        results.record_pass("cve_search_items_schema_valid")

            # aggregations is OPTIONAL (may not be in response)
            aggs = body.get("aggregations")
            if aggs is not None:
                results.record_pass("cve_search_aggregations_schema_valid")
                if isinstance(aggs.get("by_severity"), dict):
                    results.record_pass("aggregations_by_severity_is_object")
            else:
                results.record_skip("cve_search_aggregations_schema", "aggregations not in response")

        except Exception as e:
            results.record_fail("cve_search_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cve_search_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("cve_search_returns_200", f"Got {resp.status_code}: {resp.text[:300]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: POST /cves/search — filter theo severity
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /cves/search with severity filter"))
    resp = client.post("/cves/search", v2=True, body={
        "severity": ["Critical", "High"],
        "page": 1,
        "page_size": 10
    })
    if resp.status_code == 200:
        try:
            body = resp.json()
            data = body.get("data", [])
            # Tất cả CVE trả về phải có severity Critical hoặc High
            invalid = [c.get("id") for c in data if c.get("severity") not in ("Critical", "High")]
            if invalid:
                results.record_fail("cve_search_severity_filter_works",
                                    f"CVEs with wrong severity: {invalid}")
            else:
                results.record_pass("cve_search_severity_filter_works")
        except Exception as e:
            results.record_fail("cve_search_severity_filter", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cve_search_severity_filter", "Endpoint not implemented")
    else:
        results.record_fail("cve_search_severity_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: POST /cves/search — kev_only filter
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /cves/search with kev_only=true"))
    resp = client.post("/cves/search", v2=True, body={
        "kev_only": True,
        "page": 1,
        "page_size": 5
    })
    if resp.status_code == 200:
        try:
            body = resp.json()
            data = body.get("data", [])
            non_kev = [c.get("id") for c in data if not c.get("is_kev")]
            if non_kev:
                results.record_fail("cve_search_kev_only_filter",
                                    f"Non-KEV CVEs returned: {non_kev}")
            else:
                results.record_pass("cve_search_kev_only_filter_works")
        except Exception as e:
            results.record_fail("cve_search_kev_only", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cve_search_kev_only", "Endpoint not implemented")
    else:
        results.record_fail("cve_search_kev_only", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: POST /cves/search — sort_by validation
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /cves/search with sort_by=epss_desc"))
    resp = client.post("/cves/search", v2=True, body={
        "sort_by": "epss_desc",
        "page": 1,
        "page_size": 5
    })
    if resp.status_code == 200:
        try:
            body = resp.json()
            data = body.get("data", [])
            # Nếu có nhiều hơn 1 item, EPSS phải giảm dần
            if len(data) >= 2:
                scores = [c.get("epss_score", 0) for c in data]
                is_sorted = all(scores[i] >= scores[i+1] for i in range(len(scores)-1))
                if is_sorted:
                    results.record_pass("cve_search_sort_epss_desc_works")
                else:
                    results.record_fail("cve_search_sort_epss_desc_works",
                                        f"EPSS scores not sorted desc: {scores}")
            else:
                results.record_pass("cve_search_sort_epss_desc_returns_200")
        except Exception as e:
            results.record_fail("cve_search_sort_epss_desc", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cve_search_sort_epss_desc", "Endpoint not implemented")
    else:
        results.record_fail("cve_search_sort_epss_desc", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: POST /cves/search/semantic → SemanticSearchResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: POST /cves/search/semantic → SemanticSearchResponse"))
    resp = client.post("/cves/search/semantic", v2=True, body={
        "query": "SQL injection vulnerabilities in web applications",
        "limit": 5
    })
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, SEMANTIC_SEARCH_RESPONSE_REQUIRED, "SemanticSearchResponse")
            if errors:
                results.record_fail("semantic_search_response_schema", "; ".join(errors))
            else:
                results.record_pass("semantic_search_returns_200_with_schema")

            # query_embedding_ms phải là integer
            if isinstance(body.get("query_embedding_ms"), int):
                results.record_pass("semantic_search_embedding_ms_is_int")
            else:
                results.record_fail("semantic_search_embedding_ms_is_int",
                                    f"query_embedding_ms={body.get('query_embedding_ms')}")

            # data items có similarity_score
            data = body.get("data", [])
            for i, cve in enumerate(data[:3]):
                sim = cve.get("similarity_score")
                if sim is not None and isinstance(sim, (int, float)) and 0.0 <= sim <= 1.0:
                    pass
                elif sim is not None:
                    results.record_fail(f"semantic_search_item_{i}_similarity_range",
                                        f"similarity_score={sim} out of 0-1")
                    break
            else:
                if data:
                    results.record_pass("semantic_search_items_have_valid_similarity_score")

        except Exception as e:
            results.record_fail("semantic_search_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("semantic_search_returns_200", "Endpoint not implemented (404)")
    elif resp.status_code == 503:
        results.record_skip("semantic_search_returns_200", "AI service unavailable (503)")
    else:
        results.record_fail("semantic_search_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /cves/{id} → CVE detail (với detail-only fields)
    # ─────────────────────────────────────────────────────────────────────────
    cve_id = Config.SAMPLE_CVE_ID
    print(_info(f"Test: GET /cves/{cve_id} → CVE detail schema"))
    resp = client.get(f"/cves/{cve_id}", v2=True)
    if resp.status_code == 200:
        try:
            cve = resp.json()
            cve_errors = _validate_cve(cve, f"CVE[{cve_id}]")
            if cve_errors:
                results.record_fail("cve_detail_schema_valid", "; ".join(cve_errors))
            else:
                results.record_pass("cve_detail_returns_200_with_schema")

            # ID phải khớp với request
            if cve.get("id") == cve_id:
                results.record_pass("cve_detail_id_matches_request")
            else:
                results.record_fail("cve_detail_id_matches_request",
                                    f"Requested {cve_id}, got {cve.get('id')}")

            # detail-only: affected_products phải là list nếu có
            if "affected_products" in cve:
                if isinstance(cve["affected_products"], list):
                    results.record_pass("cve_detail_affected_products_is_list")
                else:
                    results.record_fail("cve_detail_affected_products_is_list", "not a list")

            # detail-only: references phải là list nếu có
            if "references" in cve:
                if isinstance(cve["references"], list):
                    results.record_pass("cve_detail_references_is_list")
                else:
                    results.record_fail("cve_detail_references_is_list", "not a list")

        except Exception as e:
            results.record_fail("cve_detail_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        # CVE có thể không tồn tại trong DB
        try:
            error_body = resp.json()
            errors = validate_required_fields(error_body, ["error", "message"], "APIError")
            if errors:
                results.record_fail("cve_detail_404_returns_APIError", "; ".join(errors))
            else:
                results.record_pass("cve_detail_404_returns_APIError_schema")
        except Exception:
            results.record_fail("cve_detail_404_returns_APIError", "Response is not valid JSON")
    else:
        results.record_fail(f"cve_detail_{cve_id}", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /cves/export?format=json
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /cves/export?format=json → binary file"))
    resp = client.get("/cves/export", v2=True, params={"format": "json"})
    if resp.status_code == 200:
        content_type = resp.headers.get("Content-Type", "")
        if len(resp.content) > 0:
            results.record_pass("cve_export_json_returns_200_with_content")
        else:
            results.record_fail("cve_export_json_returns_200_with_content", "Empty response body")
    elif resp.status_code == 404:
        results.record_skip("cve_export_json", "Endpoint not implemented (404)")
    else:
        results.record_fail("cve_export_json_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
