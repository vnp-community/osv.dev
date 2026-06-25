# SOL-BUG-SEED-001: Giải pháp khắc phục lỗi Missing Routes (Phiên bản cập nhật — v2)

**Liên kết:** [BUG-SEED-001](../001-seed-script-missing-routes.md)  
**Cập nhật lần cuối:** 2026-06-19  
**Mục tiêu:** Mô tả chính xác trạng thái thực tế của code tính đến thời điểm cập nhật này, xác định nguyên nhân gốc rễ thực sự còn tồn tại, và cung cấp hướng dẫn fix cụ thể cho từng bug.

> **Lưu ý quan trọng:** Bản giải pháp v1 trước đó đã mô tả một phần không còn chính xác với trạng thái code hiện tại. Nhiều fix đã được implement. Tài liệu này phản ánh việc đối chiếu thực tế với source code.

---

## Tóm tắt trạng thái hiện tại

| Bug | Mô tả | Trạng thái Code | Action cần làm |
|-----|--------|----------------|----------------|
| Bug 1 | Identity: 404 POST /admin/users, /bulk | ✅ ĐÃ FIX trong code | Verify deployment / infra |
| Bug 2 | Identity: 404 POST /admin/users/{id}/api-keys | ✅ ĐÃ FIX trong code | Verify deployment / infra |
| Bug 3 | Finding: 404 POST entity creation (product-types, products, tests, engagements, finding-groups) | ✅ ĐÃ FIX trong code | Verify runtime wiring |
| Bug 4a | SLA: 404 POST /api/v2/sla-configurations | ⚠️ PORT MISMATCH — bug còn tồn tại | **FIX trong `wire.go`** |
| Bug 4b | Notification: 404 POST /api/v2/notification-rules, /subscriptions, /api/v1/webhooks | ✅ Routes đã implement | Verify WireEmbedded called properly |
| Bug 5 | Asset: 502 upstream_not_configured | ✅ ĐÃ FIX trong code | Verify deployment |

---

## Bug 1 & 2: Identity Service (Trạng thái: ĐÃ FIX)

### Phân tích thực tế

Sau khi đối chiếu source code:

**`apps/osv/internal/gateway/router.go` (dòng 100–102):**
```go
// Đã có đầy đủ — KHÔNG CẦN THÊM
mux.Handle("POST /api/v1/admin/users/bulk", adminOnly(rl.Limit("5/minute")(proxy.Forward("identity-service:8081"))))
mux.Handle("POST /api/v1/admin/users", adminOnly(rl.Limit("20/minute")(proxy.Forward("identity-service:8081"))))
mux.Handle("POST /api/v1/admin/users/{id}/api-keys", adminOnly(proxy.Forward("identity-service:8081")))
```

**`services/identity-service/adapter/handler/http/router.go` (dòng 127–145):**
```go
// Đã có đầy đủ — KHÔNG CẦN THÊM
r.Post("/users", adminH.CreateUser)
r.Post("/users/bulk", adminH.BulkCreateUsers)   // literal trước wildcard
r.Post("/users/{id}/api-keys", adminH.CreateAPIKeyForUser)
```

**`services/identity-service/adapter/handler/http/admin_handler.go`:**  
- `CreateUser` (dòng 322) ✅  
- `BulkCreateUsers` (dòng 372) ✅  
- `CreateAPIKeyForUser` (dòng 275) — dùng `h.apiKeyUC.CreateAPIKey()` thay vì `adminUC` ✅

### Kết luận

Code layer đã hoàn chỉnh. Nếu lỗi 404 vẫn xảy ra khi seed, **nguyên nhân là deployment**, không phải code:

- **Kiểm tra:** Đảm bảo process `apps/osv` đang chạy phiên bản code mới nhất trên `c12.openledger.vn`
- **Kiểm tra:** Identity service đang bind đúng port `8081` và đã được start bởi orchestrator
- **Lệnh verify:**
  ```bash
  curl -X POST http://localhost:8081/api/v1/admin/users \
    -H "X-User-Role: admin" -H "Content-Type: application/json" \
    -d '{"email":"test@t.com","password":"test","role":"user"}'
  ```

---

## Bug 3: Finding Service (Trạng thái: ĐÃ FIX)

### Phân tích thực tế

**`apps/osv/internal/gateway/router.go`** đã có đầy đủ tất cả routes:
```go
// Bulk routes — dòng 324–325, 331, 356–357
mux.Handle("POST /api/v2/product-types/bulk", ...)
mux.Handle("POST /api/v2/product-types", ...)
mux.Handle("POST /api/v2/products/bulk", ...)
mux.Handle("POST /api/v2/findings/bulk-create", ...)
mux.Handle("POST /api/v2/findings/bulk", ...)
mux.Handle("POST /api/v2/finding-groups", ...)  // dòng 372
```

**`services/finding-service/internal/delivery/http/router.go`** — Trailing slash issue đã được khắc phục:  
`engagements` và `tests` được đăng ký theo cách flat (không phải `r.Route()`):
```go
// dòng 175–176: đăng ký trực tiếp, không có r.Route gây trailing slash issue
r.Get("/api/v2/engagements", engagement.List)
r.Post("/api/v2/engagements", engagement.Create)

// dòng 186–187: tương tự cho tests
r.Get("/api/v2/tests", test.List)
r.Post("/api/v2/tests", test.Create)
```

`FindingGroupHandler` đã được mount (dòng 234–236):
```go
if findingGroup != nil {
    r.Post("/api/v2/finding-groups", findingGroup.Create)
}
```

### Kết luận

Code layer đã hoàn chỉnh. Nếu lỗi 404 vẫn xảy ra:
- **Kiểm tra:** `finding-service:8085` đang được start bởi orchestrator (xem `wire.go` dòng 164–167)
- **Kiểm tra:** `findingGroup` handler không phải `nil` khi `NewRouter()` được gọi. Nếu `findingGroup == nil`, route sẽ không được đăng ký mặc dù code có đó.
- **Action:** Verify `WireEmbedded` trong `services/finding-service` đang khởi tạo đầy đủ tất cả handlers và truyền vào `NewRouter()`.

---

## Bug 4a: SLA Service — Port Mismatch (Trạng thái: ⚠️ CÒN TỒN TẠI — CẦN FIX)

### Phân tích thực tế — ROOT CAUSE

Đây là bug **thực sự còn tồn tại** trong code. Có sự không nhất quán về port giữa hai layer:

| Layer | Khai báo port |
|-------|--------------|
| `apps/osv/internal/gateway/router.go` (dòng 407–416) | `sla-service:8086` |
| `apps/osv/internal/config/wire.go` (dòng 106) | `adapters.NewEmbeddedService("sla-service", 8093)` |
| `apps/osv/internal/config/wire.go` (dòng 156) | `SLAAddr: "http://localhost:8093"` |

Gateway router hardcode `sla-service:8086`, trong khi embedded service lại listen trên port `8093`. Kết quả: request proxy đến `:8086` — port không có service nào listen → `404` hoặc `502`.

> Ghi chú: Port `8086` bị conflict với AI service (dòng 95 trong `wire.go`: `aiSvc := adapters.NewEmbeddedService("ai-service", 8086)`). Đây là lý do port SLA phải đổi sang `8093`, nhưng Gateway router chưa được cập nhật theo.

### Giải pháp: Sửa Gateway Router

**File cần sửa:** `apps/osv/internal/gateway/router.go`

Tìm tất cả các dòng proxy đến `sla-service:8086` và đổi thành `sla-service:8093`:

```go
// Hiện tại (SAI)
mux.Handle("GET /api/v2/sla-configurations", protected(proxy.Forward("sla-service:8086")))
mux.Handle("POST /api/v2/sla-configurations", protected(proxy.Forward("sla-service:8086")))
mux.Handle("GET /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8086")))
mux.Handle("PUT /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8086")))
mux.Handle("DELETE /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8086")))
mux.Handle("POST /api/v2/sla-configurations/{id}/assign/{product_id}", protected(proxy.Forward("sla-service:8086")))
mux.Handle("GET /api/v2/sla-dashboard", protected(proxy.Forward("sla-service:8086")))
mux.Handle("GET /api/v2/sla-violations", protected(proxy.Forward("sla-service:8086")))
mux.Handle("GET /api/v2/sla-violations/{product_id}", protected(proxy.Forward("sla-service:8086")))
// Và các routes v1:
mux.Handle("GET /api/v1/sla/config", protected(proxy.Forward("sla-service:8086")))
mux.Handle("PUT /api/v1/sla/config", adminOnly(proxy.Forward("sla-service:8086")))

// Sau khi fix (ĐÚNG)
mux.Handle("GET /api/v2/sla-configurations", protected(proxy.Forward("sla-service:8093")))
mux.Handle("POST /api/v2/sla-configurations", protected(proxy.Forward("sla-service:8093")))
mux.Handle("GET /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8093")))
mux.Handle("PUT /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8093")))
mux.Handle("DELETE /api/v2/sla-configurations/{id}", protected(proxy.Forward("sla-service:8093")))
mux.Handle("POST /api/v2/sla-configurations/{id}/assign/{product_id}", protected(proxy.Forward("sla-service:8093")))
mux.Handle("GET /api/v2/sla-dashboard", protected(proxy.Forward("sla-service:8093")))
mux.Handle("GET /api/v2/sla-violations", protected(proxy.Forward("sla-service:8093")))
mux.Handle("GET /api/v2/sla-violations/{product_id}", protected(proxy.Forward("sla-service:8093")))
mux.Handle("GET /api/v1/sla/config", protected(proxy.Forward("sla-service:8093")))
mux.Handle("PUT /api/v1/sla/config", adminOnly(proxy.Forward("sla-service:8093")))
```

**Thay thế:** Nếu muốn dùng lại port `8086` cho SLA (như trong architecture spec), cần đổi AI service sang port khác trong `wire.go`:
```go
// Đổi AI về đúng port 9103 (theo architecture.md)
aiSvc := adapters.NewEmbeddedService("ai-service", 9103)  // ← thay vì 8086
slaSvc := adapters.NewEmbeddedService("sla-service", 8086) // ← dùng lại port đúng
```
Và cập nhật Gateway config trong `wire.go`:
```go
SLAAddr: "http://localhost:8086",
```

> ⚠️ **Khuyến nghị:** Chọn option 2 (giữ SLA ở 8086, đổi AI về 9103) để giữ đúng với `01-architecture.md` (Service Port Map) và các docs khác. AI service được định nghĩa port `9103` trong tất cả spec.

---

## Bug 4b: Notification Service (Trạng thái: VERIFY ROUTES)

### Phân tích thực tế

**Gateway router** đã có đầy đủ routes (dòng 418–436):
```go
mux.Handle("POST /api/v2/notification-rules", protected(proxy.Forward("notification-service:8087")))
mux.Handle("POST /api/v2/subscriptions", protected(proxy.Forward("notification-service:8087")))
mux.Handle("POST /api/v1/webhooks", protected(proxy.Forward("notification-service:8087")))
```

**Notification service router** (`services/notification-service/internal/delivery/http/router.go`):  
- `POST /api/v1/webhooks` ✅ (dòng 38–39)  
- `POST /api/v2/subscriptions` ✅ (dòng 49)  
- `POST /api/v2/notification-rules` ✅ (dòng 74–75)

**`apps/osv/internal/config/wire.go`** (dòng 97–101):
```go
notifSvc := adapters.NewEmbeddedService("notification-service", 8087)
if err := notif.WireEmbedded(ctx, logger, dbPool, notifSvc.Mux); err != nil {
    slog.Error("failed to wire embedded notification-service", "err", err)
}
```

Port `8087` nhất quán giữa `wire.go`, Gateway router và architecture spec.

### Kết luận

Nếu seed vẫn thất bại với notification routes, **nguyên nhân có thể là:**
1. `notif.WireEmbedded()` thất bại do lỗi DB connection → kiểm tra log startup
2. `rh *RuleHandler` là `nil` do thiếu dependency khi init → `notification-rules` routes sẽ không được đăng ký (code có guard `if rh != nil`)
3. Deployment chưa cập nhật binary

**Action:**
- Kiểm tra log khởi động: `grep "failed to wire embedded notification-service" logs`
- Test trực tiếp: `curl -X POST http://localhost:8087/api/v2/notification-rules -d '...'`

---

## Bug 5: Asset Service (Trạng thái: ĐÃ FIX)

### Phân tích thực tế

**`apps/osv/internal/config/wire.go`** (dòng 111–115):
```go
assetSvc := adapters.NewEmbeddedService("asset-service", 8091)
if err := asset.WireEmbedded(ctx, logger, dbPool, assetSvc.Mux); err != nil {
    slog.Error("failed to wire embedded asset-service", "err", err)
}
```

**`services/asset-service/embedded.go`** có `WireEmbedded()` đầy đủ, mount tất cả asset routes lên `ServeMux`.

**Gateway router** (dòng 258–270) đã có đầy đủ routes đến `asset-service:8091`.

### Kết luận

Lỗi 502 nguyên gốc đã được khắc phục. Nếu vẫn còn lỗi:
- Kiểm tra log: `grep "failed to wire embedded asset-service" logs`
- Kiểm tra port `8091` không bị conflict với service khác
- Test: `curl http://localhost:8091/api/v1/assets`

---

## Checklist Action Items

- [ ] **[CRITICAL]** Sửa `apps/osv/internal/gateway/router.go`: đổi tất cả `sla-service:8086` → `sla-service:8093` **HOẶC** sửa `wire.go` để dùng `aiSvc := ...8093` và `slaSvc := ...8086` (theo khuyến nghị)
- [ ] **[VERIFY]** Deploy lại `apps/osv` với binary mới nhất trên `c12.openledger.vn`  
- [ ] **[VERIFY]** Kiểm tra log orchestrator startup — đảm bảo tất cả `WireEmbedded()` không lỗi
- [ ] **[VERIFY]** Kiểm tra `finding-service` WireEmbedded truyền `findingGroup != nil` vào `NewRouter()`
- [ ] **[VERIFY]** Chạy lại `02_push_seed_data.py` sau khi fix port SLA

---

## Port Allocation Map (Sau khi fix)

| Service | Port | Trạng thái |
|---------|------|-----------|
| `apps/osv` (gateway) | 8080 | ✅ OK |
| `identity-service` | 8081 | ✅ OK |
| `data-service` | 8082 | ✅ OK |
| `search-service` | 8083 | ✅ OK |
| `scan-service` | 8084 | ✅ OK (wire.go dùng 8088, nhưng scan-service riêng biệt) |
| `finding-service` | 8085 | ✅ OK |
| `sla-service` | 8086 → `8093` trong wire.go | ⚠️ PORT MISMATCH — cần fix |
| `notification-service` | 8087 | ✅ OK |
| `jira-service` | 8088 | ✅ OK |
| `product-service` | 8089 → `8092` trong wire.go | ⚠️ Khác với arch spec nhưng nhất quán nội bộ |
| `audit-service` | 8090 | ✅ OK |
| `asset-service` | 8091 | ✅ OK |
| `ai-service` | 9103 → `8086` trong wire.go | ⚠️ Port conflict với SLA — nguyên nhân gốc |

> **Ghi chú:** Trong architecture.md, `ai-service` dùng port `9103`. `wire.go` đang dùng `8086` cho AI service, dẫn đến conflict với port của SLA trong spec. Fix chuẩn xác là đưa AI về `9103` và trả SLA về `8086`.
