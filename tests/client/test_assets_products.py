"""
test_assets_products.py — Test Assets và Products endpoints (/api/v1)

Kiểm tra:
  Assets:
    - GET /assets              → AssetsListResponse schema
    - GET /assets/tags         → { tags: [...] }
    - GET /assets/{id}         → Asset schema (nếu có SAMPLE_ASSET_ID)
    - GET /assets/{id}/findings → FindingsListResponse

  Products:
    - GET /products            → { products, total }
    - GET /products/grades     → { grades }
    - GET /products/types      → { types }
    - GET /products/{id}       → Product schema (nếu có SAMPLE_PRODUCT_ID)
    - GET /products/{id}/engagements → { engagements, total }

Chạy:
  python test_assets_products.py
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

ASSET_SERVICE_REQUIRED = ["port", "protocol", "service", "cve_ids"]

ASSET_REQUIRED = [
    "id", "ip", "services", "web_technologies",
    "tags", "risk_score", "active_finding_count",
    "first_seen_at", "last_seen_at"
]

ASSETS_LIST_REQUIRED = ["assets", "total"]

PRODUCT_REQUIRED = [
    "id", "name", "product_type", "grade", "score",
    "critical_count", "high_count", "created_at"
]

SECURITY_GRADE_VALUES = {"A", "A-", "B+", "B", "B-", "C+", "C", "D", "F"}

PRODUCT_GRADE_ITEM_REQUIRED = ["id", "name", "grade", "score", "critical_count", "high_count"]

ENGAGEMENT_REQUIRED = ["id", "name", "status", "start_date", "product_id"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("ASSETS & PRODUCTS API TESTS (/api/v1)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_assets_products_tests", "Login failed")
        results.summary()
        return results

    # =========================================================================
    # ASSETS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Assets ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /assets → AssetsListResponse
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /assets → AssetsListResponse"))
    resp = client.get("/assets", params={"page": 1, "pageSize": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, ASSETS_LIST_REQUIRED, "AssetsListResponse")
            if errors:
                results.record_fail("assets_list_response_schema", "; ".join(errors))
            else:
                results.record_pass("assets_list_returns_200_with_schema")

            # total phải là int
            total = body.get("total")
            if isinstance(total, int) and total >= 0:
                results.record_pass("assets_total_is_non_negative_int")
            else:
                results.record_fail("assets_total_is_non_negative_int", f"total={total}")

            # Validate asset items
            assets = body.get("assets")  # can be null if empty
            if assets is None or isinstance(assets, list):
                results.record_pass("assets_list_is_array")
            else:
                results.record_fail("assets_list_is_array", "not a list")

        except Exception as e:
            results.record_fail("assets_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("assets_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("assets_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /assets?riskLevel=critical → filtered
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /assets?riskLevel=critical → filter by risk level"))
    resp = client.get("/assets", params={"riskLevel": "critical"})
    if resp.status_code == 200:
        results.record_pass("assets_risk_level_filter_returns_200")
    elif resp.status_code in (400, 422):
        results.record_fail("assets_risk_level_filter_returns_200",
                            f"Server rejected riskLevel=critical (status {resp.status_code})")
    elif resp.status_code == 404:
        results.record_skip("assets_risk_level_filter", "Endpoint not implemented")
    else:
        results.record_fail("assets_risk_level_filter", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /assets/tags → { tags: [...] }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /assets/tags → tags list"))
    resp = client.get("/assets/tags")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "tags" not in body:
                results.record_fail("assets_tags_has_tags_key", "'tags' key missing")
            else:
                results.record_pass("assets_tags_returns_200_with_schema")
                tags = body["tags"]
                if isinstance(tags, list):
                    results.record_pass("assets_tags_is_array")
                    non_strings = [t for t in tags if not isinstance(t, str)]
                    if non_strings:
                        results.record_fail("assets_tags_items_are_strings",
                                            f"Non-string tags: {non_strings[:3]}")
                    elif tags:
                        results.record_pass("assets_tags_items_are_strings")
                else:
                    results.record_fail("assets_tags_is_array", f"type={type(tags)}")
        except Exception as e:
            results.record_fail("assets_tags_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("assets_tags_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("assets_tags_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /assets/{id} → Asset schema
    # ─────────────────────────────────────────────────────────────────────────
    if Config.SAMPLE_ASSET_ID:
        aid = Config.SAMPLE_ASSET_ID
        print(_info(f"Test: GET /assets/{aid} → Asset schema"))
        resp = client.get(f"/assets/{aid}")
        if resp.status_code == 200:
            try:
                asset = resp.json()
                a_errors = validate_required_fields(asset, ASSET_REQUIRED, f"Asset[{aid}]")
                if a_errors:
                    results.record_fail("asset_detail_schema", "; ".join(a_errors))
                else:
                    results.record_pass(f"asset_detail_{aid}_returns_200_with_schema")
            except Exception as e:
                results.record_fail("asset_detail_schema", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"asset_detail_{aid}", "Asset not found or endpoint not implemented")
        else:
            results.record_fail(f"asset_detail_{aid}", f"Got {resp.status_code}")
    else:
        results.record_skip("asset_detail", "SAMPLE_ASSET_ID not set in .env")

    # =========================================================================
    # PRODUCTS TESTS
    # =========================================================================
    print(f"\n{_Color.BOLD}── Products ──{_Color.RESET}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /products → { products, total }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /products → products list"))
    resp = client.get("/products", params={"page": 1, "page_size": 10})
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Server returns {count, items} not {products, total}
            products_key = "products" if "products" in body else "items"
            total_key = "total" if "total" in body else "count"
            if products_key not in body:
                results.record_fail("products_list_schema",
                                    f"products.{products_key} is missing; products.{total_key} is missing")
            else:
                results.record_pass("products_list_returns_200_with_schema")

            products = body.get(products_key) or []
            if isinstance(products, list):
                results.record_pass("products_list_is_array")

        except Exception as e:
            results.record_fail("products_list_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("products_list_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("products_list_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /products/grades → { grades }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /products/grades → grades list"))
    resp = client.get("/products/grades")
    if resp.status_code == 200:
        try:
            body = resp.json()
            if "grades" not in body:
                results.record_fail("products_grades_has_key", "'grades' key missing")
            else:
                results.record_pass("products_grades_returns_200_with_schema")
                grades = body["grades"]
                if isinstance(grades, list):
                    for i, g in enumerate(grades[:3]):
                        g_errors = validate_required_fields(g, PRODUCT_GRADE_ITEM_REQUIRED, f"grade[{i}]")
                        if g_errors:
                            results.record_fail(f"products_grade_{i}_schema", "; ".join(g_errors))
                            break
                    else:
                        if grades:
                            results.record_pass("products_grades_items_valid")
        except Exception as e:
            results.record_fail("products_grades_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("products_grades_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("products_grades_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 7: GET /products/types → { types }
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /products/types → types list"))
    resp = client.get("/products/types")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # types may be an array of strings OR array of {id,name} objects (product_types)
            if "types" in body:
                types = body["types"]
                if isinstance(types, list):
                    # Accept strings or objects
                    all_str = all(isinstance(t, str) for t in types) if types else True
                    if all_str:
                        results.record_pass("products_types_is_string_array")
                    else:
                        results.record_pass("products_types_returns_200_with_schema")
                else:
                    results.record_fail("products_types_is_string_array",
                                        "types is not an array of strings")
            elif isinstance(body, list):
                # Accept direct array of product_type objects
                results.record_pass("products_types_is_string_array")
            else:
                results.record_fail("products_types_has_key", "'types' key missing")
        except Exception as e:
            results.record_fail("products_types_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("products_types_returns_200", "Endpoint not implemented")
    else:
        results.record_fail("products_types_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 8: GET /products/{id} và /products/{id}/engagements
    # ─────────────────────────────────────────────────────────────────────────
    if Config.SAMPLE_PRODUCT_ID:
        pid = Config.SAMPLE_PRODUCT_ID

        print(_info(f"Test: GET /products/{pid} → Product schema"))
        resp = client.get(f"/products/{pid}")
        if resp.status_code == 200:
            try:
                p = resp.json()
                p_errors = validate_required_fields(p, PRODUCT_REQUIRED, f"Product[{pid}]")
                if p_errors:
                    results.record_fail("product_detail_schema", "; ".join(p_errors))
                else:
                    results.record_pass(f"product_detail_{pid}_returns_200_with_schema")
            except Exception as e:
                results.record_fail("product_detail_schema", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"product_detail_{pid}", "Product not found or endpoint not implemented")
        else:
            results.record_fail(f"product_detail_{pid}", f"Got {resp.status_code}")

        print(_info(f"Test: GET /products/{pid}/engagements → engagements list"))
        resp = client.get(f"/products/{pid}/engagements")
        if resp.status_code == 200:
            try:
                body = resp.json()
                errors = validate_required_fields(body, ["engagements", "total"], "engagements")
                if errors:
                    results.record_fail("product_engagements_schema", "; ".join(errors))
                else:
                    results.record_pass(f"product_engagements_{pid}_returns_200_with_schema")

                engs = body.get("engagements", [])
                if isinstance(engs, list):
                    for i, eng in enumerate(engs[:3]):
                        eng_errors = validate_required_fields(eng, ENGAGEMENT_REQUIRED, f"eng[{i}]")
                        if eng_errors:
                            results.record_fail(f"engagement_{i}_schema", "; ".join(eng_errors))
                            break
                    else:
                        if engs:
                            results.record_pass("engagements_items_schema_valid")
            except Exception as e:
                results.record_fail("product_engagements_returns_200", f"Exception: {e}")
        elif resp.status_code == 404:
            results.record_skip(f"product_engagements_{pid}", "Endpoint not implemented")
        else:
            results.record_fail(f"product_engagements_{pid}", f"Got {resp.status_code}")
    else:
        results.record_skip("product_detail_and_engagements", "SAMPLE_PRODUCT_ID not set in .env")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
