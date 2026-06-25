# BUG-BE-007 — KEV API Response Schema Không Khớp Spec

| Trường | Giá trị |
|---|---|
| **ID** | BUG-BE-007 |
| **Severity** | 🟠 High |
| **Priority** | P1 |
| **Component** | Backend / Data Service / KEV Handler |
| **Endpoints** | `GET /api/v2/kev`, `GET /api/v2/kev/ransomware`, `GET /api/v2/kev/stats` |
| **Phát hiện** | 2026-06-19 |
| **Server** | https://c12.openledger.vn |
| **Status** | Open |

---

## Mô tả

KEV endpoints trả về HTTP 200 nhưng **cấu trúc JSON không khớp** với OpenAPI spec. Data service dùng field names cũ (`items`, `count`) thay vì (`data`, `total`, `page_size`). Các fields thống kê (`stats`, `added_last_30_days`, `ransomware_related`) hoàn toàn vắng mặt.

## So sánh Schema

### `GET /api/v2/kev`

| Field | Spec | Thực tế |
|---|---|---|
| List field name | `data` | `items` |
| Count field name | `total` | `count` |
| `page` | ✅ required | ❓ |
| `page_size` | ✅ required | ❌ vắng mặt |
| `stats.total` | ✅ required | ❌ vắng mặt |
| `stats.added_last_30_days` | ✅ required | ❌ vắng mặt |
| `stats.ransomware_related` | ✅ required | ❌ vắng mặt |

**Actual response:**
```json
{
  "items": [...],
  "count": 1100
}
```

**Expected response (theo spec):**
```json
{
  "data": [
    {
      "cve_id": "CVE-2021-44228",
      "vendor": "Apache",
      "product": "Log4j",
      "vulnerability_name": "...",
      "date_added": "2021-12-10",
      "due_date": "2022-01-03",
      "known_ransomware_campaign_use": false
    }
  ],
  "total": 1100,
  "page": 1,
  "page_size": 20,
  "stats": {
    "total": 1100,
    "added_last_30_days": 15,
    "ransomware_related": 200
  }
}
```

### `GET /api/v2/kev/stats`

**Expected (theo spec):**
```json
{
  "stats": {
    "total": 1100,
    "added_last_30_days": 15,
    "ransomware_related": 200
  },
  "by_vendor": [
    { "vendor": "Microsoft", "count": 250 }
  ],
  "recent_additions": [...]
}
```
**Actual:** Trả `{ "total": N }` flat, thiếu `stats`, `by_vendor`, `recent_additions`.

## Fix

Cập nhật KEV handler trong data-service để trả đúng schema:

```go
// Đổi field names trong response struct
type KEVListResponse struct {
    Data     []KEVEntry `json:"data"`      // thay vì "items"
    Total    int        `json:"total"`     // thay vì "count"
    Page     int        `json:"page"`
    PageSize int        `json:"page_size"` // thêm mới
    Stats    KEVStats   `json:"stats"`     // thêm mới
}

type KEVStats struct {
    Total              int `json:"total"`
    AddedLast30Days    int `json:"added_last_30_days"`
    RansomwareRelated  int `json:"ransomware_related"`
}
```

## Ảnh hưởng

- Trang KEV Catalog (`/cve/kev`) load được nhưng thiếu stats widget
- Bộ lọc pagination có thể không hoạt động đúng (`page_size` thiếu)
- Dashboard `kev_alerts` widget thiếu context thống kê
