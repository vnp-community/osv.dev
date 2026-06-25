# Data Models — ranking-service

> **Service**: `services/ranking-service`  
> **Mô tả**: Quản lý CPE-based priority ranking theo nhóm người dùng (e.g. IT, accounting, security). Cho phép tổ chức ưu tiên CVE theo nhóm nghiệp vụ. Lookup hỗ trợ fuzzy CPE matching.  
> **Storage**: MongoDB (collection: `ranking`)  
> **Go package**: `services/ranking-service/internal/domain`

---

## 1. RankingEntry

Ánh xạ một CPE string fragment sang priority ranks theo từng nhóm.  
MongoDB document trong collection `ranking`.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | Yes | MongoDB `_id` (auto-generated) |
| `cpe` | string | No | CPE fragment, e.g. `sap:netweaver` hoặc `apache:log4j` |
| `rank` | []GroupRank | No | Danh sách rank theo nhóm (ít nhất 1) |

**Validation**: `cpe` bắt buộc, `rank` phải có ít nhất 1 entry, mỗi `group` không được rỗng, `rank` ≥ 0.

---

## 2. GroupRank

Liên kết một group name với priority integer.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `group` | string | No | Tên nhóm, e.g. `it`, `accounting`, `security`, `finance` |
| `rank` | int | No | Mức độ ưu tiên; giá trị cao hơn = quan trọng hơn với nhóm này. 0 = ưu tiên thấp nhất |

---

## 3. LookupResult

Kết quả trả về từ fuzzy CPE ranking lookup.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `cpe` | string | No | CPE fragment đã match |
| `ranks` | []GroupRank | No | Danh sách ranks theo nhóm |
| `matched_part` | string | No | Phần CPE đã match (cho trường hợp partial match) |

---

## 4. Ví dụ Document

```json
{
  "cpe": "sap:netweaver",
  "rank": [
    {"group": "it", "rank": 3},
    {"group": "accounting", "rank": 5}
  ]
}
```

---

## 5. HTTP API Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/v1/ranking` | Lookup ranking cho một CPE |
| `POST` | `/api/v1/ranking` | Tạo mới ranking entry |
| `PUT` | `/api/v1/ranking/{cpe}` | Cập nhật ranking entry |
| `DELETE` | `/api/v1/ranking/{cpe}` | Xóa ranking entry |
| `GET` | `/api/v1/ranking/groups` | Danh sách các groups hiện có |

---

## 6. Relationships

```
RankingEntry ─── GroupRank (1:N, embedded array)
CVE (data-service) ─── RankingEntry (N:1, via CPE matching)
LookupResult ─── RankingEntry (query result)
```
