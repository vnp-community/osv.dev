# Data Models — search-service

> **Service**: `services/search-service`  
> **Mô tả**: CVE search engine hỗ trợ full-text search, filter đa chiều (severity, EPSS, KEV, exploit, vendor/product), browse catalog và taxonomy (CWE/CAPEC).  
> **Storage**: MongoDB (CVE documents, full-text index), PostgreSQL (taxonomy CWE/CAPEC), Redis (vendor/product CPE cache)  
> **Go package**: `services/search-service/internal/domain/entity`

---

## 1. CVE (Search Read Model)

Read model tối ưu cho tìm kiếm. Khác với CVE entity của data-service ở chỗ chỉ chứa các trường cần thiết cho display/filter.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | CVE ID, ví dụ `CVE-2021-44228` |
| `description` | string | No | Mô tả lỗ hổng |
| `severity` | Severity | No | Mức nghiêm trọng |
| `published` | timestamp | No | Ngày công bố |
| `source` | Source | No | Nguồn dữ liệu |
| `is_kev` | bool | No | Có trong CISA KEV catalog |
| `is_exploit` | bool | No | Có public exploit |
| `link` | string | Yes | URL tham chiếu |
| `cvss_score` | float64 | Yes | CVSS v2 score |
| `cvss3_score` | float64 | Yes | CVSS v3 score |
| `epss` | float64 | Yes | EPSS probability |
| `epss_percentile` | float64 | Yes | EPSS percentile |
| `vendors` | []string | Yes | CPE vendors |
| `products` | []string | Yes | CPE products |
| `created_at` | timestamp | No | |
| `updated_at` | timestamp | No | |

**Enums — Severity**: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `UNKNOWN`

**Helper**: `IsValidID(id string) bool` — validates CVE ID format với regex `^CVE-\d{4}-\d{4,}$` (case-insensitive).

**Enums — Source**:

| Giá trị | Mô tả |
|---------|-------|
| `NVD` | National Vulnerability Database |
| `CIRCL` | Luxembourg CIRCL CVE feed |
| `JVN` | Japan Vulnerability Notes |
| `EXPLOITDB` | Exploit-DB |
| `CVE.ORG` | CVE.org official |
| `ARCHIVE` | Archived sources |

---

## 2. CVESummary

Lightweight search result (không có đầy đủ thông tin).

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | CVE ID |
| `summary` | string | No | Tóm tắt ngắn |
| `cvss3` | float64 | Yes | CVSS v3 score |
| `score` | float64 | Yes | Text search relevance score |

---

## 3. SearchFilter

Tham số tìm kiếm đầy đủ.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `query` | string | Yes | Từ khóa tìm kiếm |
| `severity` | Severity | Yes | Lọc theo severity |
| `source` | Source | Yes | Lọc theo nguồn |
| `sort` | SortOrder | No | Thứ tự sắp xếp |
| `page` | int | No | Trang (bắt đầu từ 0) |
| `limit` | int | No | Số kết quả mỗi trang (mặc định 50, tối đa 100) |
| `min_epss` | float64 | Yes | EPSS tối thiểu |
| `max_epss` | float64 | Yes | EPSS tối đa |
| `is_kev` | bool | Yes | Chỉ hiện KEV entries |
| `is_exploit` | bool | Yes | Chỉ hiện CVE có exploit |
| `cwe` | string | Yes | Lọc theo CWE, ví dụ `CWE-89` |
| `vendor` | string | Yes | Lọc theo vendor CPE |
| `product` | string | Yes | Lọc theo product CPE |

**Enums — SortOrder**:

| Giá trị | Mô tả |
|---------|-------|
| `newest` | Mới nhất trước (mặc định) |
| `oldest` | Cũ nhất trước |
| `epss_desc` | EPSS cao nhất trước |
| `cvss3_desc` | CVSS3 cao nhất trước |

---

## 4. SearchRequest (Full-text)

Yêu cầu full-text search qua MongoDB.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `keywords` | []string | Từ khóa (AND semantics: tất cả phải xuất hiện) |
| `limit` | int | Số kết quả |
| `full_data` | bool | Trả về đầy đủ CVE hay chỉ summary |

---

## 5. MongoSearchResult

Kết quả từ MongoDB full-text search.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `results` | []CVESummary | Danh sách kết quả |
| `total` | int | Tổng số kết quả |
| `query` | []string | Từ khóa đã tìm |

---

## 6. VendorCatalog

Danh sách vendors từ Redis CPE cache.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `vendors` | []string | Danh sách vendor names |
| `total` | int | Tổng số vendors |
| `cached_at` | timestamp | Thời điểm cache |

> **Redis key schema**: `"a"` → SET of application vendors, `"o"` → OS vendors, `"h"` → hardware vendors

---

## 7. ProductCatalog

Danh sách products của một vendor.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `vendor` | string | Tên vendor |
| `products` | []string | Danh sách products |
| `total` | int | |
| `cached_at` | timestamp | Thời điểm cache |

> **Redis key**: `"v:{vendor}"` → SET of products

---

## 8. CWEEntry

Weakness từ MITRE CWE catalog.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | CWE ID, ví dụ `CWE-89` |
| `name` | string | No | Tên weakness |
| `description` | string | No | Mô tả chi tiết |
| `abstraction` | string | No | `Base` \| `Class` \| `Variant` |
| `status` | string | No | Trạng thái CWE |
| `capec_ids` | []string | Yes | CAPEC patterns liên quan |
| `updated_at` | timestamp | No | |

---

## 9. CAPECEntry

Attack pattern từ MITRE CAPEC catalog.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | CAPEC ID, ví dụ `CAPEC-66` |
| `name` | string | No | Tên attack pattern |
| `description` | string | No | |
| `likelihood` | string | No | `High` \| `Medium` \| `Low` |
| `severity` | string | No | Mức nghiêm trọng |
| `cwe_ids` | []string | Yes | CWE IDs liên quan |
| `updated_at` | timestamp | No | |

---

## 10. Relationships

```
CVE (read model) ─────────── SearchFilter (query parameters)
VendorCatalog ─────────────── ProductCatalog (1:N, by vendor)
CWEEntry ──────────────────── CAPECEntry (N:M, via capec_ids/cwe_ids)
MongoSearchResult ─────────── CVESummary (1:N)
```

---

## 11. Go Source Files

| File | Nội dung |
|------|----------|
| `domain/entity/cve.go` | CVE, SearchFilter, SortOrder, Source, Severity, IsValidID |
| `domain/entity/search.go` | SearchRequest, CVESummary, MongoSearchResult |
| `domain/entity/vendor_catalog.go` | VendorCatalog, ProductCatalog |
| `domain/entity/taxonomy.go` | CWEEntry, CAPECEntry |
| `domain/entity/entity.go` | Base/shared entity types |
