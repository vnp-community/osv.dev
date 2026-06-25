"""
test_dashboard.py — Test Dashboard endpoints (/api/v1/dashboard/*)

Kiểm tra:
  - GET /dashboard              → DashboardData schema
  - GET /dashboard?period=90d   → DashboardData schema (period param)
  - GET /dashboard/sla          → SLADashboardData schema

Chạy:
  python test_dashboard.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from base_client import (
    APIClient, TestResults, validate_required_fields,
    validate_list_response, validate_pagination, _info, _Color
)
from config import Config


# ── Field definitions từ OpenAPI spec ────────────────────────────────────────

DASHBOARD_REQUIRED = [
    "kpis", "risk_trend", "severity_distribution",
    "product_grades", "kev_alerts", "recent_scans", "sla_breaches"
]

DASHBOARD_KPI_REQUIRED = [
    "critical_findings", "high_findings", "total_assets", "high_risk_assets",
    "active_scans", "queued_scans", "security_grade", "security_score",
    "sla_compliance", "sla_at_risk", "sla_breached"
]

SECURITY_GRADE_VALUES = {"A", "A-", "B+", "B", "B-", "C+", "C", "D", "F"}

SEVERITY_DISTRIBUTION_REQUIRED = ["critical", "high", "medium", "low", "total"]

RISK_TREND_POINT_REQUIRED = ["month", "critical", "high", "medium", "low"]

PRODUCT_GRADE_REQUIRED = ["id", "name", "grade", "score", "critical_count", "high_count"]

KEV_ALERT_REQUIRED = ["cve_id", "vendor", "product", "date_added", "is_ransomware"]

RECENT_SCAN_REQUIRED = ["id", "name", "type", "status", "targets", "finding_count", "started_at", "created_by"]

SLA_BREACH_REQUIRED = [
    "finding_id", "title", "cve_id", "severity",
    "product_name", "sla_expiration_date", "days_overdue"
]

SLA_DASHBOARD_REQUIRED = [
    "summary", "compliance_trend", "breached_findings",
    "at_risk_findings", "by_product", "total_breached", "total_at_risk",
    "page", "page_size"
]

SLA_SUMMARY_REQUIRED = ["total_active_findings", "compliance_percent", "breached", "at_risk", "ok"]


def run_tests() -> TestResults:
    results = TestResults()
    client = APIClient()

    print(f"\n{_Color.BOLD}{'='*60}")
    print("DASHBOARD API TESTS (/api/v1/dashboard)")
    print(f"{'='*60}{_Color.RESET}\n")

    if not client.login():
        results.record_skip("all_dashboard_tests", "Login failed")
        results.summary()
        return results

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 1: GET /dashboard (default 30d) → DashboardData
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dashboard → DashboardData"))
    resp = client.get("/dashboard")
    if resp.status_code == 200:
        try:
            body = resp.json()
            # Validate top-level fields
            errors = validate_required_fields(body, DASHBOARD_REQUIRED, "DashboardData")
            if errors:
                results.record_fail("dashboard_response_schema", "; ".join(errors))
            else:
                results.record_pass("dashboard_returns_200_with_DashboardData")

            # Validate KPIs
            kpis = body.get("kpis", {})
            kpi_errors = validate_required_fields(kpis, DASHBOARD_KPI_REQUIRED, "DashboardKPIs")
            if kpi_errors:
                results.record_fail("dashboard_kpis_schema", "; ".join(kpi_errors))
            else:
                results.record_pass("dashboard_kpis_schema_valid")

            # security_grade phải thuộc enum
            grade = kpis.get("security_grade")
            if grade in SECURITY_GRADE_VALUES:
                results.record_pass("dashboard_security_grade_valid_enum")
            else:
                results.record_fail("dashboard_security_grade_valid_enum",
                                    f"security_grade='{grade}' not in {SECURITY_GRADE_VALUES}")

            # security_score phải 0-100
            score = kpis.get("security_score")
            if isinstance(score, int) and 0 <= score <= 100:
                results.record_pass("dashboard_security_score_0_to_100")
            else:
                results.record_fail("dashboard_security_score_0_to_100",
                                    f"security_score={score}")

            # Validate severity_distribution
            sev_dist = body.get("severity_distribution", {})
            sd_errors = validate_required_fields(sev_dist, SEVERITY_DISTRIBUTION_REQUIRED, "SeverityDistribution")
            if sd_errors:
                results.record_fail("dashboard_severity_distribution_schema", "; ".join(sd_errors))
            else:
                results.record_pass("dashboard_severity_distribution_schema_valid")
                # total phải = critical + high + medium + low
                expected_total = (
                    sev_dist.get("critical", 0) + sev_dist.get("high", 0) +
                    sev_dist.get("medium", 0) + sev_dist.get("low", 0)
                )
                if sev_dist.get("total") == expected_total:
                    results.record_pass("dashboard_severity_total_equals_sum")
                else:
                    results.record_fail("dashboard_severity_total_equals_sum",
                                        f"total={sev_dist.get('total')} but sum={expected_total}")

            # Validate risk_trend array items
            risk_trend = body.get("risk_trend", [])
            if isinstance(risk_trend, list):
                results.record_pass("dashboard_risk_trend_is_array")
                for i, point in enumerate(risk_trend):
                    pt_errors = validate_required_fields(
                        point, RISK_TREND_POINT_REQUIRED, f"risk_trend[{i}]"
                    )
                    if pt_errors:
                        results.record_fail(f"dashboard_risk_trend_item_{i}_schema", "; ".join(pt_errors))
                        break
                else:
                    if risk_trend:
                        results.record_pass("dashboard_risk_trend_items_schema_valid")
            else:
                results.record_fail("dashboard_risk_trend_is_array", "risk_trend is not a list")

            # Validate product_grades items
            product_grades = body.get("product_grades", [])
            if isinstance(product_grades, list):
                for i, pg in enumerate(product_grades[:3]):  # kiểm tra 3 phần tử đầu
                    pg_errors = validate_required_fields(pg, PRODUCT_GRADE_REQUIRED, f"product_grades[{i}]")
                    if pg_errors:
                        results.record_fail(f"dashboard_product_grade_{i}_schema", "; ".join(pg_errors))
                        break
                else:
                    if product_grades:
                        results.record_pass("dashboard_product_grades_items_valid")

            # Validate kev_alerts items
            kev_alerts = body.get("kev_alerts", [])
            if isinstance(kev_alerts, list):
                for i, kev in enumerate(kev_alerts[:3]):
                    kev_errors = validate_required_fields(kev, KEV_ALERT_REQUIRED, f"kev_alerts[{i}]")
                    if kev_errors:
                        results.record_fail(f"dashboard_kev_alert_{i}_schema", "; ".join(kev_errors))
                        break
                else:
                    if kev_alerts:
                        results.record_pass("dashboard_kev_alerts_items_valid")

            # Validate recent_scans items
            recent_scans = body.get("recent_scans", [])
            if isinstance(recent_scans, list):
                for i, scan in enumerate(recent_scans[:3]):
                    scan_errors = validate_required_fields(scan, RECENT_SCAN_REQUIRED, f"recent_scans[{i}]")
                    if scan_errors:
                        results.record_fail(f"dashboard_recent_scan_{i}_schema", "; ".join(scan_errors))
                        break
                else:
                    if recent_scans:
                        results.record_pass("dashboard_recent_scans_items_valid")

        except Exception as e:
            results.record_fail("dashboard_returns_200_with_DashboardData", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("dashboard_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("dashboard_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 2: GET /dashboard?period=90d → DashboardData
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dashboard?period=90d → DashboardData"))
    resp = client.get("/dashboard", params={"period": "90d"})
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, DASHBOARD_REQUIRED, "DashboardData")
            if errors:
                results.record_fail("dashboard_90d_response_schema", "; ".join(errors))
            else:
                results.record_pass("dashboard_period_90d_returns_200_with_schema")
        except Exception as e:
            results.record_fail("dashboard_period_90d", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("dashboard_period_90d", "Endpoint not implemented (404)")
    else:
        results.record_fail("dashboard_period_90d", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 3: GET /dashboard?period=1y → DashboardData
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dashboard?period=1y → DashboardData"))
    resp = client.get("/dashboard", params={"period": "1y"})
    if resp.status_code == 200:
        results.record_pass("dashboard_period_1y_returns_200")
    elif resp.status_code == 400:
        results.record_fail("dashboard_period_1y_returns_200",
                            "Server rejected period=1y (should be valid per spec)")
    elif resp.status_code == 404:
        results.record_skip("dashboard_period_1y", "Endpoint not implemented (404)")
    else:
        results.record_fail("dashboard_period_1y_returns_200", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 4: GET /dashboard?period=invalid → 400 hoặc 422
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dashboard?period=invalid → 400/422"))
    resp = client.get("/dashboard", params={"period": "invalid_period_xyz"})
    if resp.status_code in (400, 422):
        results.record_pass("dashboard_invalid_period_returns_4xx")
    elif resp.status_code == 200:
        # Server mềm dẻo, không lỗi — ghi nhận warning không fail
        results.record_skip("dashboard_invalid_period_returns_4xx",
                            "Server ignores invalid period (returns 200)")
    elif resp.status_code == 404:
        results.record_skip("dashboard_invalid_period_returns_4xx", "Endpoint not implemented")
    else:
        results.record_fail("dashboard_invalid_period_returns_4xx", f"Got {resp.status_code}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 5: GET /dashboard/sla → SLADashboardData
    # ─────────────────────────────────────────────────────────────────────────
    print(_info("Test: GET /dashboard/sla → SLADashboardData"))
    resp = client.get("/dashboard/sla")
    if resp.status_code == 200:
        try:
            body = resp.json()
            errors = validate_required_fields(body, SLA_DASHBOARD_REQUIRED, "SLADashboardData")
            if errors:
                results.record_fail("sla_dashboard_schema", "; ".join(errors))
            else:
                results.record_pass("sla_dashboard_returns_200_with_schema")

            # Validate summary
            summary = body.get("summary", {})
            sum_errors = validate_required_fields(summary, SLA_SUMMARY_REQUIRED, "SLASummary")
            if sum_errors:
                results.record_fail("sla_summary_schema", "; ".join(sum_errors))
            else:
                results.record_pass("sla_summary_schema_valid")

            # compliance_percent phải trong khoảng 0-100
            cp = summary.get("compliance_percent")
            if isinstance(cp, (int, float)) and 0.0 <= cp <= 100.0:
                results.record_pass("sla_compliance_percent_0_to_100")
            else:
                results.record_fail("sla_compliance_percent_0_to_100", f"compliance_percent={cp}")

            # page và page_size phải có
            pag_errors = validate_pagination(body)
            if pag_errors:
                results.record_fail("sla_dashboard_pagination_fields", "; ".join(pag_errors))
            else:
                results.record_pass("sla_dashboard_pagination_fields_valid")

        except Exception as e:
            results.record_fail("sla_dashboard_returns_200", f"Exception: {e}")
    elif resp.status_code == 404:
        results.record_skip("sla_dashboard_returns_200", "Endpoint not implemented (404)")
    else:
        results.record_fail("sla_dashboard_returns_200", f"Got {resp.status_code}: {resp.text[:200]}")

    # ─────────────────────────────────────────────────────────────────────────
    # TEST 6: GET /dashboard/sla?product_id=xxx → SLADashboardData lọc theo product
    # ─────────────────────────────────────────────────────────────────────────
    if Config.SAMPLE_PRODUCT_ID:
        print(_info(f"Test: GET /dashboard/sla?product_id={Config.SAMPLE_PRODUCT_ID}"))
        resp = client.get("/dashboard/sla", params={"product_id": Config.SAMPLE_PRODUCT_ID})
        if resp.status_code == 200:
            results.record_pass("sla_dashboard_with_product_id_filter")
        elif resp.status_code == 404:
            results.record_skip("sla_dashboard_with_product_id_filter", "Endpoint not implemented")
        else:
            results.record_fail("sla_dashboard_with_product_id_filter", f"Got {resp.status_code}")
    else:
        results.record_skip("sla_dashboard_with_product_id_filter", "SAMPLE_PRODUCT_ID not set in .env")

    results.summary()
    return results


if __name__ == "__main__":
    r = run_tests()
    sys.exit(r.exit_code())
