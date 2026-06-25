# Data Models — data-service

> **Service**: `services/data-service`  
> **Mô tả**: Nguồn dữ liệu CVE trung tâm. Fetch, normalize và lưu trữ vulnerability data từ NVD, CIRCL, JVN, ExploitDB, v.v. Hỗ trợ cả PostgreSQL (CVE Binary Tool mode) và MongoDB (cve-search mode).  
> **Storage**: PostgreSQL (structured CVE, ranges, severity), MongoDB (cve-search format), Redis (CPE cache)

---

## 1. CVE

Bản ghi lỗ hổng bảo mật chuẩn (canonical vulnerability record). Hỗ trợ dual-backend.

| Trường | Kiểu | Nullable | Backend | Mô tả |
|--------|------|----------|---------|-------|
| `id` | string | No | MongoDB | CVE ID string, ví dụ `CVE-2021-44228` |
| `published` | timestamp | No | Both | Ngày công bố |
| `modified` | timestamp | No | Both | Lần cập nhật cuối |
| `summary` | string | No | Both | Tóm tắt ngắn |
| `description` | string | Yes | Both | Mô tả chi tiết |
| `status` | string | Yes | MongoDB | Trạng thái CVE record |
| `assigner` | string | Yes | MongoDB | Tổ chức gán CVE |
| `cvss` | float64 | Yes | Both | CVSS v2 score |
| `cvss_vector` | string | Yes | Both | CVSS v2 vector |
| `cvss3` | float64 | Yes | Both | CVSS v3 score |
| `cvss3_vector` | string | Yes | Both | CVSS v3 vector |
| `cvss4` | float64 | Yes | MongoDB | CVSS v4 score |
| `epss` | float64 | Yes | Both | EPSS probability 0–1 |
| `epss_percentile` | float64 | Yes | Both | Vị trí phần trăm EPSS |
| `severity` | Severity | No | Both | Mức nghiêm trọng |
| `references` | []string | Yes | MongoDB | URLs tham khảo |
| `cwe` | []string | Yes | MongoDB | Danh sách CWE IDs |
| `vendors` | []string | Yes | MongoDB | CPE vendors |
| `products` | []string | Yes | MongoDB | CPE products |
| `vulnerable_configuration` | []string | Yes | MongoDB | CPE match strings |
| `vulnerable_product` | []string | Yes | MongoDB | CPE products bị ảnh hưởng |
| `source` | string | Yes | Both | `NVD`\|`CIRCL`\|`JVN`\|`EXPLOITDB`\|`CVE.ORG`\|`CNNVD` |
| `is_kev` | bool | No | Both | Có trong CISA KEV catalog |
| `is_exploit` | bool | No | Both | Có public exploit |
| `link` | string | Yes | Both | URL tham chiếu chính |
| `jvn_id` | string | Yes | MongoDB | ID JVN, ví dụ `JVNDB-2021-002374` |
| `affected_packages` | []AffectedPackage | Yes | PostgreSQL | Packages bị ảnh hưởng |
| `embedding` | []float32 | Yes | PostgreSQL | Vector embedding cho AI |
| `embedding_model` | string | Yes | PostgreSQL | Model tạo embedding |
| `remediation` | string | Yes | PostgreSQL | Hướng dẫn vá |
| `sources` | []string | Yes | PostgreSQL | Danh sách nguồn đóng góp |
| `remarks` | Remarks | Yes | PostgreSQL | Trạng thái triage |
| `justification` | string | Yes | PostgreSQL | Lý do triage |
| `response` | []string | Yes | PostgreSQL | Phản hồi VEX |
| `created_at` | timestamp | No | PostgreSQL | |
| `updated_at` | timestamp | No | PostgreSQL | |
| `last_fetched_at` | timestamp | No | PostgreSQL | |

**Enums — Severity**: `critical` (CVSS ≥ 9.0), `high` (7.0–8.9), `medium` (4.0–6.9), `low` (0.1–3.9), `none`

---

## 2. AffectedPackage

Package version bị ảnh hưởng bởi CVE.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `ecosystem` | string | No | Hệ sinh thái: `npm`, `pypi`, `go`, `maven`, v.v. |
| `package_name` | string | No | Tên package |
| `versions` | []string | No | Danh sách versions bị ảnh hưởng |
| `fixed_version` | string | Yes | Version đã vá |

---

## 3. CVERange

Khoảng phiên bản bị ảnh hưởng theo format NVD CPE / OSV.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `cve_number` | string | No | CVE ID |
| `vendor` | string | No | CPE vendor (lowercase) |
| `product` | string | No | CPE product (lowercase) |
| `version` | string | No | `*` = any version; exact version otherwise |
| `version_start_including` | string | Yes | product >= value |
| `version_start_excluding` | string | Yes | product > value |
| `version_end_including` | string | Yes | product <= value |
| `version_end_excluding` | string | Yes | product < value |
| `data_source` | string | No | `NVD`\|`OSV`\|`GAD`\|... |

---

## 4. CVESeverity

Scoring chi tiết cho CVE.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `cve_number` | string | No | CVE ID |
| `severity` | string | No | `CRITICAL`\|`HIGH`\|`MEDIUM`\|`LOW`\|`NONE` |
| `description` | string | Yes | Mô tả severity |
| `score` | float64 | No | CVSS base score |
| `cvss_version` | int | No | 2 hoặc 3 |
| `cvss_vector` | string | Yes | CVSS vector string |
| `data_source` | string | No | Nguồn scoring |
| `last_modified` | string | Yes | RFC3339 timestamp |

---

## 5. TriageEntry

Quyết định triage/review cho một CVE cụ thể.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `remarks` | Remarks | No | Trạng thái triage |
| `comments` | string | Yes | Ghi chú của reviewer |
| `response` | []string | Yes | VEX response strings |
| `justification` | string | Yes | Lý do quyết định |

**Enums — Remarks**:

| Giá trị | Tên | Mô tả |
|---------|-----|-------|
| 1 | `NewFound` | Vừa phát hiện, chưa review |
| 2 | `Unexplored` | Đang điều tra |
| 3 | `Confirmed` | Đã xác nhận áp dụng |
| 4 | `Mitigated` | Đã vá hoặc giảm thiểu |
| 5 | `FalsePositive` | Xác nhận false positive |
| 6 | `NotAffected` | Vendor xác nhận không bị ảnh hưởng (VEX) |

---

## 6. PURL2CPE

Ánh xạ từ Package URL sang CPE string.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `purl` | string | Package URL (ví dụ `pkg:npm/lodash@4.17.20`) |
| `cpe` | string | CPE string tương ứng |

---

## 7. DBState

Metadata về local CVE database.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `schema_version` | string | Phiên bản schema |
| `last_updated` | string | RFC3339 timestamp |
| `cve_count` | int64 | Tổng số CVE records |
| `range_count` | int64 | Tổng số version ranges |

---

## 8. Reference

URL tham chiếu CVE.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `url` | string | URL tham chiếu |
| `type` | string | `ADVISORY`\|`PATCH`\|`WEB`\|`REPORT` |

---

## 9. Relationships

```
CVE ──────────────────── CVERange (1:N, theo cve_number)
CVE ──────────────────── CVESeverity (1:N, theo cve_number)
CVE ──────────────────── AffectedPackage (1:N, embedded)
PURL2CPE ─────────────── (standalone lookup table)
TriageData ───────────── CVE (map[cve_number]TriageEntry)
```

---

## 10. Nguồn dữ liệu (Sources)

| Source | Mô tả |
|--------|-------|
| `NVD` | National Vulnerability Database (NIST) |
| `CIRCL` | Luxembourg CIRCL CVE feed |
| `JVN` | Japan Vulnerability Notes |
| `EXPLOITDB` | Exploit-DB public exploits |
| `CVE.ORG` | CVE.org official feed |
| `CNNVD` | China National Vulnerability DB |
