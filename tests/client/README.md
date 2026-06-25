# OSV Platform API — Test Client

Bộ Python scripts dùng để **query server thực tế** và **validate dữ liệu trả về** có đúng theo [OpenAPI Spec](../ui/specs/openapi.yaml) không.

## Cấu trúc thư mục

```
tests/client/
├── .env.example              # Template cấu hình (copy → .env)
├── config.py                 # Đọc .env và expose Config class
├── base_client.py            # HTTP client, logger, schema validator dùng chung
├── run_all.py                # Runner tổng hợp — chạy tất cả test modules
│
├── test_auth.py              # Auth: login, refresh, me, MFA, logout
├── test_dashboard.py         # Dashboard: KPIs, risk trend, SLA dashboard
├── test_cve_intelligence.py  # CVE: search, semantic search, detail, export
├── test_kev_epss.py          # KEV Catalog + EPSS Analytics
├── test_taxonomy.py          # CWE/CAPEC + Vendor Browse + DBInfo
├── test_findings_scans.py    # Findings, Scans, SLA Config, Risk Acceptances
├── test_assets_products.py   # Assets + Products + Engagements
├── test_admin_notifications.py # Admin, Profile, Notifications
└── test_ai_reports.py        # AI Triage, Enrichment, Reports, Webhooks, JIRA
```

## Yêu cầu

- Python 3.9+
- Thư viện: `requests` (không cần gì khác — không phụ thuộc `pytest` hay `jsonschema`)

```bash
pip install requests
```

## Cấu hình

```bash
cp .env.example .env
```

Chỉnh sửa `.env`:

| Biến | Mô tả | Mặc định |
|---|---|---|
| `API_BASE_URL_V1` | Base URL v1 | `http://localhost:8080/api/v1` |
| `API_BASE_URL_V2` | Base URL v2 | `http://localhost:8080/api/v2` |
| `TEST_EMAIL` | Email đăng nhập | `admin@osv.local` |
| `TEST_PASSWORD` | Mật khẩu | `changeme` |
| `ACCESS_TOKEN` | JWT token (bỏ qua auto-login) | _(rỗng)_ |
| `SAMPLE_CVE_ID` | CVE ID để test GET /cves/{id} | `CVE-2021-44228` |
| `SAMPLE_CWE_ID` | CWE ID để test GET /cwe/{id} | `CWE-89` |
| `SAMPLE_VENDOR` | Vendor để test browse | `apache` |
| `SAMPLE_PRODUCT_NAME` | Product để test browse | `log4j` |
| `SAMPLE_SCAN_ID` | Scan ID (optional) | _(rỗng)_ |
| `SAMPLE_FINDING_ID` | Finding ID (optional) | _(rỗng)_ |
| `SAMPLE_PRODUCT_ID` | Product ID (optional) | _(rỗng)_ |
| `SAMPLE_ASSET_ID` | Asset ID (optional) | _(rỗng)_ |

### Ví dụ cho môi trường staging c12.openledger.vn

```env
API_BASE_URL_V1=https://c12.openledger.vn/api/v1
API_BASE_URL_V2=https://c12.openledger.vn/api/v2
TEST_EMAIL=admin@example.com
TEST_PASSWORD=your_password
SAMPLE_CVE_ID=CVE-2021-44228
SAMPLE_VENDOR=apache
SAMPLE_PRODUCT_NAME=log4j
VERBOSE=true
```

## Cách chạy

### Chạy toàn bộ test suite

```bash
cd tests/client
python run_all.py
```

### Chạy một module cụ thể

```bash
python run_all.py auth
python run_all.py dashboard cve
python run_all.py kev_epss taxonomy
```

### Liệt kê các module

```bash
python run_all.py --list
```

### Chạy từng script đơn lẻ

```bash
python test_auth.py
python test_dashboard.py
python test_cve_intelligence.py
python test_kev_epss.py
python test_taxonomy.py
python test_findings_scans.py
python test_assets_products.py
python test_admin_notifications.py
python test_ai_reports.py
```

### Options

```bash
python run_all.py --verbose          # Log chi tiết mọi HTTP request
python run_all.py --no-stop-on-fail  # Tiếp tục dù có failure
python run_all.py auth --verbose     # Kết hợp module + verbose
```

## Cách đọc kết quả

```
✓ login_valid_credentials_returns_200       ← PASS
✗ cve_search_sort_epss_desc_works: ...      ← FAIL (kèm lý do)
⚠ SKIP semantic_search_returns_200: ...    ← SKIP (endpoint chưa có)
```

Cuối mỗi module và cuối run_all.py sẽ có **summary**:

```
============================================================
TEST SUMMARY
============================================================
  Total   : 42
  Passed  : 38
  Failed  : 2
  Skipped : 2
  Time    : 3.14s
============================================================
```

## Logic validation

Mỗi test script thực hiện:

1. **Status code check** — HTTP response code đúng không (200, 201, 202, 204...)
2. **Required fields** — Tất cả `required` fields trong OpenAPI schema phải có
3. **Type check** — integer/string/boolean/array đúng kiểu
4. **Enum check** — `severity`, `status`, `grade`... phải thuộc tập giá trị định nghĩa
5. **Range check** — `epss_score ∈ [0,1]`, `security_score ∈ [0,100]`, `sla_compliance_percent ∈ [0,100]`...
6. **Consistency check** — `severity.total = sum(critical+high+medium+low)`, sort order...
7. **Filter check** — Filter params có được server tôn trọng không (kev_only, status=active...)

## Ghi chú

- Các test **không làm thay đổi dữ liệu** (chủ yếu là GET). Một số POST an toàn (create report, trigger enrichment) được thực hiện nhưng không xoá.
- Test sẽ **SKIP** (không FAIL) khi server trả 404 (endpoint chưa implement).
- Test sẽ **SKIP** khi `SAMPLE_*_ID` không được set trong `.env`.
- Dùng `ACCESS_TOKEN` trong `.env` để bypass login nếu server MFA bắt buộc.
