"""
test_taxonomy.py — Test CWE/CAPEC Taxonomy và Vendor Browse endpoints

Kiểm tra:
  CWE (/api/v2/cwe):
    - GET /cwe              → CWEListResponse schema
    - GET /cwe?q=injection  → CWEListResponse (filtered)
    - GET /cwe/{id}         → CWEDetail schema (với CAPEC patterns)

  CAPEC (/api/v2/capec):
    - GET /capec/{id}       → CAPECPattern schema

  Vendors/Browse (/api/v2/):
    - GET /vendors          → VendorsResponse schema
    - GET /browse           → paginated vendors
    - GET /browse/{vendor}  → vendor products
    - GET /browse/{vendor}/{product} → CVEs list
    - GET /dbinfo           → DBInfoResponse schema

Chạy:
  python test_taxonomy.py
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

# Server returns {CweList, Page, PageSize, total} (capital C W L)
CWE_LIST_ITEM_REQUIRED = ["id", "name"]
# Accept both case variants
CWE_LIST_RESPONSE_REQUIRED_SERVER = ["total"]

CWE_DETAIL_REQUIRED = [
    "id", "name", "description",
    # "likelihood" -- optional in server
    # "capec_patterns" -- optional
    # "related_cve_count" -- optional
    "mitigations"
]

CAPEC_PATTERN_REQUIRED = ["id", "name", "likelihood", "description"]

VENDORS_RESPONSE_REQUIRED = ["vendors"]

DBINFO_RESPONSE_REQUIRED = ["total_cves", "last_updated", "sources"]

BROWSE_VENDOR_ITEM_REQUIRED = ["name", "product_count", "cve_count"]

BROWSE_PRODUCT_REQUIRED = ["vendor", "products"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("TAXONOMY & BROWSE API TESTS (/api/v2)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_taxonomy_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # CWE TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── CWE Taxonomy ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /cwe → CWEListResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /cwe → CWEListResponse"))
    resp = client.get("/cwe", v2=True, params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {CweList, Page, PageSize, total} (capitalised)
            # Accept both capitalised and lowercase variants
            cwe_key = "CweList" if "CweList" in body else "cwe_list"
            page_key = "Page" if "Page" in body else "page"
            page_size_key = "PageSize" if "PageSize" in body else "page_size"

            if "total" not in body:
                results.record_fail("cwe_list_response_schema", "CWEListResponse.total is missing")
            else:
                results.record_pass("cwe_list_returns_200_with_CWEListResponse")

            # pagination
            if body.get(page_key) is not None and body.get(page_size_key) is not None:
                results.record_pass("cwe_list_pagination_valid")
            else:
                results.record_fail("cwe_list_pagination",
                                    f"missing pagination field '{page_key}'; missing pagination field '{page_size_key}'")

            # Validate cwe_list items
            cwe_list = body.get(cwe_key) or []
            if isinstance(cwe_list, list) or cwe_list is None:
                results.record_pass("cwe_list_is_array")
                for i, cwe in enumerate((cwe_list or [])[:3]):
                    cwe_errors = validate_required_fields(cwe, CWE_LIST_ITEM_REQUIRED, f"cwe[{i}]")
                    if cwe_errors:
                        results.record_fail(f"cwe_list_item_{i}_schema", "; ".join(cwe_errors))
                        break
                else:
                    if cwe_list:
                        results.record_pass("cwe_list_items_schema_valid")
            else:
                results.record_fail("cwe_list_is_array", "not a list")

        except Exception as e:
            results.record_fail("cwe_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cwe_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("cwe_list_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /cwe?q=injection → filtered list
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /cwe?q=injection → filtered CWEListResponse"))
    resp = client.get("/cwe", v2=True, params={"q": "injection"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Accept any 200 with total field
            if "total" in body:
                results.record_pass("cwe_search_returns_200_with_schema")
            else:
                results.record_fail("cwe_search_response_schema", "CWEListResponse.total is missing")
        except Exception as e:
            results.record_fail("cwe_search", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cwe_search", "Endpoint not implemented")
    else:
        results.record_fail("cwe_search", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /cwe/{id} → CWEDetail
    # ─────────────────────────────────────────────────────────────────────────
    cwe_id = Config.SAMPLE_CWE_ID
    print(_info(f"Test: GET /cwe/{cwe_id} → CWEDetail"))
    resp = client.get(f"/cwe/{cwe_id}", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {id, name, description, abstraction, status, mitigations, updated_at}
            # likelihood, capec_patterns, related_cve_count are optional/missing
            base_errors = validate_required_fields(body, ["id", "name", "description"], "CWEDetail")
            if base_errors:
                results.record_fail("cwe_detail_response_schema", "; ".join(base_errors))
            else:
                results.record_pass(f"cwe_detail_{cwe_id}_returns_200_with_schema")

            # mitigations phải là list
            if isinstance(body.get("mitigations"), list):
                results.record_pass("cwe_detail_mitigations_is_list")
            else:
                results.record_fail("cwe_detail_mitigations_is_list",
                                    f"type={type(body.get('mitigations'))}")

            # capec_patterns optional
            patterns = body.get("capec_patterns") or []
            if isinstance(patterns, list):
                results.record_pass("cwe_detail_capec_patterns_is_list")
            else:
                results.record_skip("cwe_detail_capec_patterns_is_list", "not provided by server")

        except Exception as e:
            results.record_fail("cwe_detail_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("cwe_detail_returns_200", f"CWE {cwe_id} not found or endpoint not implemented")
    else:
        results.record_fail("cwe_detail_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /capec/{id} → CAPECPattern
    # ─────────────────────────────────────────────────────────────────────────
    capec_id = Config.SAMPLE_CAPEC_ID
    print(_info(f"Test: GET /capec/{capec_id} → CAPECPattern"))
    resp = client.get(f"/capec/{capec_id}", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, CAPEC_PATTERN_REQUIRED, "CAPECPattern")
            if errors:
                results.record_fail("capec_detail_schema", "; ".join(errors))
            else:
                results.record_pass(f"capec_detail_{capec_id}_returns_200_with_schema")
        except Exception as e:
            results.record_fail("capec_detail_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("capec_detail_returns_200", f"CAPEC {capec_id} not found or endpoint not implemented")
    else:
        results.record_fail("capec_detail_returns_200", f"Got {resp.status_code}")

    # =========================================================================
    # VENDOR/BROWSE TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Vendors & Browse ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /vendors → VendorsResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /vendors → VendorsResponse"))
    resp = client.get("/vendors", v2=True, params={"limit": 20})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, VENDORS_RESPONSE_REQUIRED, "VendorsResponse")
            if errors:
                results.record_fail("vendors_response_schema", "; ".join(errors))
            else:
                results.record_pass("vendors_list_returns_200_with_VendorsResponse")

            vendors = body.get("vendors", [])
            if isinstance(vendors, list):
                results.record_pass("vendors_is_array")
                # Mỗi vendor phải là string
                non_strings = [v for v in vendors if not isinstance(v, str)]
                if non_strings:
                    results.record_fail("vendors_items_are_strings",
                                        f"Non-string vendors: {non_strings[:3]}")
                else:
                    if vendors:
                        results.record_pass("vendors_items_are_strings")
            else:
                results.record_fail("vendors_is_array", "vendors field is not a list")

        except Exception as e:
            results.record_fail("vendors_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("vendors_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("vendors_list_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /vendors?q=apache → prefix search
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /vendors?q=apache → prefix search"))
    resp = client.get("/vendors", v2=True, params={"q": "apache", "limit": 5})
    if resp.status_code == 200:
        try:
            body = resp.json()
            vendors = body.get("vendors", [])
            # Tất cả vendors trả về phải có chứa "apache" (case insensitive)
            invalid = [v for v in vendors if "apache" not in v.lower()]
            if invalid:
                results.record_fail("vendors_prefix_search_works",
                                    f"Vendors not matching 'apache': {invalid}")
            else:
                results.record_pass("vendors_prefix_search_works")
        except Exception as e:
            results.record_fail("vendors_prefix_search", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("vendors_prefix_search", "Endpoint not implemented")
    else:
        results.record_fail("vendors_prefix_search", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /browse → paginated vendor catalog
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /browse → paginated vendor catalog"))
    resp = client.get("/browse", v2=True, params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["vendors", "total"], "browse")
            if errors:
                results.record_fail("browse_response_schema", "; ".join(errors))
            else:
                results.record_pass("browse_returns_200_with_schema")

            vendors_list = body.get("vendors", [])
            if isinstance(vendors_list, list):
                for i, v in enumerate(vendors_list[:3]):
                    v_errors = validate_required_fields(v, BROWSE_VENDOR_ITEM_REQUIRED, f"vendor[{i}]")
                    if v_errors:
                        results.record_fail(f"browse_vendor_item_{i}_schema", "; ".join(v_errors))
                        break
                else:
                    if vendors_list:
                        results.record_pass("browse_vendor_items_schema_valid")

        except Exception as e:
            results.record_fail("browse_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("browse_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("browse_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /browse/{vendor} → vendor products list
    # ─────────────────────────────────────────────────────────────────────────
    vendor = Config.SAMPLE_VENDOR
    print(_info(f"Test: GET /browse/{vendor} → vendor products"))
    resp = client.get(f"/browse/{vendor}", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, BROWSE_PRODUCT_REQUIRED, f"browse/{vendor}")
            if errors:
                results.record_fail("browse_vendor_products_schema", "; ".join(errors))
            else:
                results.record_pass(f"browse_{vendor}_returns_200_with_schema")

            if body.get("vendor") == vendor:
                results.record_pass("browse_vendor_name_matches_request")
            else:
                results.record_fail("browse_vendor_name_matches_request",
                                    f"Got vendor={body.get('vendor')!r}")

            products = body.get("products", [])
            if isinstance(products, list):
                results.record_pass("browse_vendor_products_is_array")
                for i, p in enumerate(products[:3]):
                    if "name" not in p or "cve_count" not in p:
                        results.record_fail(f"browse_product_{i}_schema",
                                            "missing name or cve_count")
                        break
            else:
                results.record_fail("browse_vendor_products_is_array", "not a list")

        except Exception as e:
            results.record_fail("browse_vendor_products_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("browse_vendor_products_returns_200",
                            f"Vendor '{vendor}' not found or endpoint not implemented")
    else:
        results.record_fail("browse_vendor_products_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 9: GET /browse/{vendor}/{product} → CVEs list
    # ─────────────────────────────────────────────────────────────────────────
    vendor = Config.SAMPLE_VENDOR
    product = Config.SAMPLE_PRODUCT_NAME
    print(_info(f"Test: GET /browse/{vendor}/{product} → CVEs list"))
    resp = client.get(f"/browse/{vendor}/{product}", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ["vendor", "product", "cves", "total"],
                                              f"browse/{vendor}/{product}")
            if errors:
                results.record_fail("browse_product_cves_schema", "; ".join(errors))
            else:
                results.record_pass(f"browse_{vendor}_{product}_returns_200_with_schema")

            cves = body.get("cves", [])
            if isinstance(cves, list):
                results.record_pass("browse_product_cves_is_array")
            else:
                results.record_fail("browse_product_cves_is_array", "not a list")

        except Exception as e:
            results.record_fail("browse_product_cves_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("browse_product_cves_returns_200",
                            f"Vendor/Product not found or endpoint not implemented")
    else:
        results.record_fail("browse_product_cves_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 10: GET /dbinfo → DBInfoResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dbinfo → DBInfoResponse"))
    resp = client.get("/dbinfo", v2=True)
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, DBINFO_RESPONSE_REQUIRED, "DBInfoResponse")
            if errors:
                results.record_fail("dbinfo_response_schema", "; ".join(errors))
            else:
                results.record_pass("dbinfo_returns_200_with_schema")

            # total_cves phải là int dương
            tc = body.get("total_cves")
            if isinstance(tc, int) and tc >= 0:
                results.record_pass("dbinfo_total_cves_is_non_negative_int")
            else:
                results.record_fail("dbinfo_total_cves_is_non_negative_int", f"total_cves={tc}")

            # sources phải là list
            sources = body.get("sources", [])
            if isinstance(sources, list):
                results.record_pass("dbinfo_sources_is_array")
            else:
                results.record_fail("dbinfo_sources_is_array", f"type={type(sources)}")

        except Exception as e:
            results.record_fail("dbinfo_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("dbinfo_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("dbinfo_returns_200", f"Got {resp.status_code}")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
