# Change Request 007: KEV Catalog v2, Notifications v1, Profile & Admin Gaps

**Cập nhật:** 2026-06-18  
**Status:** New — Các gaps còn lại sau khi tổng hợp toàn bộ frontend openapi.yaml.

## 1. Bối cảnh

Sau khi đối chiếu đầy đủ frontend openapi.yaml với backend specs:

| Endpoint frontend | Backend hiện tại | Trạng thái |
|---|---|---|
| `GET /api/v2/kev` (với filter: ransomware_only, vendor, date_from…) | `GET /api/v2/kev` | ⚠️ Exists nhưng cần verify query params filter |
| `GET /api/v2/kev/stats` (response: `KEVStatsResponse`) | `GET /api/v2/kev/stats` | ⚠️ Cần verify response schema (`by_vendor`, `recent_additions`) |
| `GET /api/v2/kev/ransomware` | `GET /api/v2/kev/ransomware` | ✅ Có |
| `GET /api/v2/kev/{cveId}` | `GET /api/v2/kev/{cveId}` | ✅ Có |
| `GET /api/v1/notifications` | `GET /api/v1/notifications` | ✅ Có |
| `GET /api/v1/notifications/stream?token=<jwt>` | `GET /api/v1/notifications/stream` | ⚠️ Cần verify `?token` query param auth |
| `PATCH /api/v1/notifications/{id}/read` | `PATCH /api/v1/notifications/{id}/read` | ✅ Có |
| `POST /api/v1/notifications/mark-all-read` | `POST /api/v1/notifications/mark-all-read` | ✅ Có |
| `GET /api/v1/notifications/unread-count` | `GET /api/v1/notifications/unread-count` | ✅ Có |
| `GET /api/v1/profile` | `GET /api/v1/profile` | ✅ Có |
| `PATCH /api/v1/profile` | `PATCH /api/v1/profile` | ✅ Có |
| `POST /api/v1/profile/change-password` | `POST /api/v1/profile/change-password` | ✅ Có |
| `GET /api/v1/api-keys` | `GET /api/v1/api-keys` | ✅ Có |
| `POST /api/v1/api-keys` | `POST /api/v1/api-keys` | ✅ Có |
| `DELETE /api/v1/api-keys/{id}` | `DELETE /api/v1/api-keys/{id}` | ✅ Có |
| `GET /api/v1/webhooks` | `GET /api/v1/webhooks` | ✅ Có |
| `POST /api/v1/webhooks` | `POST /api/v1/webhooks` | ✅ Có |
| `DELETE /api/v1/webhooks/{id}` | `DELETE /api/v1/webhooks/{id}` | ✅ Có |
| `POST /api/v1/webhooks/{id}/test` | `POST /api/v1/webhooks/{id}/test` (Admin only) | ⚠️ Frontend không phân biệt role, cần verify |
| `GET /api/v1/admin/health` | `GET /api/v1/admin/health` | ✅ Có |
| `GET /api/v1/admin/settings` | `GET /api/v1/admin/settings` | ✅ Có |
| `PATCH /api/v1/admin/settings` | `PATCH /api/v1/admin/settings` | ✅ Có |
| `GET /api/v1/audit-log` | `GET /api/v1/audit-log` (Admin only) | ⚠️ Frontend gọi không phân biệt role |
| `GET /api/v2/audit-log` | `GET /api/v2/audit-log` | ✅ Có |
| `GET /api/v1/scans` | `GET /api/v1/scans` | ✅ Có |
| `GET /api/v1/scans/scheduled` | `GET /api/v1/scans/scheduled` | ✅ Có |
| `POST /api/v1/scans/import` | `POST /api/v1/scans/import` | ✅ Có (rate limited, max 500MB) |
| `GET /api/v1/scans/{id}/stream?token=<jwt>` | `GET /api/v1/scans/{id}/stream` (SSE) | ⚠️ Cần verify `?token` query param auth |
| `GET /api/v1/sla/config` | `GET /api/v1/sla/config` | ✅ Có |
| `PUT /api/v1/sla/config` | `PUT /api/v1/sla/config` (Admin only) | ✅ Có |

## 2. Vấn đề cần giải quyết

### 2.1 [HIGH] KEV Stats response schema mismatch

Frontend yêu cầu `GET /api/v2/kev/stats` trả về `KEVStatsResponse`:
```json
{
  "stats": {
    "total": 1200,
    "added_last_30_days": 15,
    "ransomware_related": 340,
    "unmitigated_in_platform": 45
  },
  "by_vendor": [
    { "vendor": "microsoft", "count": 120 }
  ],
  "recent_additions": [ KEVEntry ]
}
```

Backend data-service cần verify và cập nhật response để bao gồm `by_vendor` và `recent_additions`.

**Cập nhật handler tại data-service** (`/api/v2/kev/stats` hoặc `/api/v1/kev/stats`):
```go
type KEVStatsResponse struct {
    Stats            KEVStats   `json:"stats"`
    ByVendor         []VendorCount `json:"by_vendor"`
    RecentAdditions  []KEVEntry    `json:"recent_additions"`
}
```

### 2.2 [HIGH] KEV List — Hỗ trợ filter params

Frontend gọi `GET /api/v2/kev` với query params:
- `page`, `page_size`
- `ransomware_only` (boolean)
- `vendor` (string)
- `date_from`, `date_to` (date format)
- `sort_by`: `date_added_desc | date_added_asc | vendor_asc`

Data-service handler cần hỗ trợ tất cả params này.

Response phải là `KEVListResponse`:
```json
{
  "data": [ KEVEntry ],
  "total": 1200,
  "page": 1,
  "page_size": 20,
  "stats": { "total": 1200, "added_last_30_days": 15, ... }
}
```

### 2.3 [HIGH] SSE Endpoints — Xác thực qua `?token` query param

Hai SSE endpoints dùng query param thay vì Authorization header (vì EventSource API không hỗ trợ custom headers):

- `GET /api/v1/notifications/stream?token=<jwt>`
- `GET /api/v1/scans/{id}/stream?token=<jwt>`

**Hiện tại**: Gateway authenticate qua middleware `authMW.Authenticate` (đọc `Authorization: Bearer <token>` header).

**Cần thêm**: SSE middleware đọc `?token` query param như một fallback cho SSE endpoints.

```go
// apps/osv/internal/gateway/auth/middleware.go
func (m *AuthMiddleware) AuthenticateSSE(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.URL.Query().Get("token")
        if token == "" {
            // Fallback to Authorization header
            token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        }
        // Validate token...
        next.ServeHTTP(w, r)
    })
}
```

```go
// router.go — dùng sseAuth thay vì protected cho SSE endpoints
sseAuth := chain(authMW.AuthenticateSSE, transform.InjectUserHeaders)
mux.Handle("GET /api/v1/notifications/stream", sseAuth(proxy.ForwardSSE("notification-service:8087")))
mux.Handle("GET /api/v1/scans/{id}/stream",    sseAuth(proxy.ForwardSSE("scan-service:8084")))
```

### 2.4 [MEDIUM] KEVEntry response — field `notes` nullable

Frontend schema `KEVEntry` có field `notes: string | null`. Cần verify data-service response có include field này.

### 2.5 [LOW] Audit log access control

`GET /api/v1/audit-log` hiện là `adminOnly`. Frontend gọi từ Admin panel (chỉ Admin access) — **không cần thay đổi**, nhưng cần document rõ ràng.

`GET /api/v2/audit-log` là `protected` (tất cả auth users). Cần confirm đây là intentional.

### 2.6 [LOW] CreateAPIKeyResponse — field `secret` chỉ hiển thị một lần

`POST /api/v1/api-keys` response phải bao gồm:
```json
{
  "api_key": { APIKey object },
  "secret": "osv_sk_abc123..."  // Full key — shown ONCE only
}
```

Cần verify identity-service trả về đúng format này.

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

1. `GET /api/v2/kev/stats` trả về `{ stats: {...}, by_vendor: [...], recent_additions: [...] }`.
2. `GET /api/v2/kev?ransomware_only=true&page=1&page_size=20` filter đúng, trả về `KEVListResponse`.
3. `GET /api/v2/kev?sort_by=date_added_desc` sort đúng thứ tự.
4. `GET /api/v1/notifications/stream?token=<valid_jwt>` kết nối SSE thành công, không bị `401 Unauthorized`.
5. `GET /api/v1/scans/{id}/stream?token=<valid_jwt>` kết nối SSE thành công.
6. `POST /api/v1/api-keys` trả về `{ api_key: {...}, secret: "..." }` — secret chỉ có trong response create, không có ở list/get.
