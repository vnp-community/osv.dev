# TASK-FIX-002: Verify Finding Service — Product/Engagement/Test/FindingGroup Creation

**Trạng thái:** ✅ Code đã hoàn chỉnh — Cần verify runtime wiring  
**Liên kết bug:** [BUG-SEED-001 §3](../001-seed-script-missing-routes.md)  
**Liên kết giải pháp:** [SOL-BUG-SEED-001 §Bug3](../solutions/SOL-BUG-SEED-001.md)  
**Priority:** P1 (Verify sau khi deploy binary mới)  
**Cập nhật:** 2026-06-19 — Bổ sung thông tin từ `deploy/dev/`


---

## Bối cảnh

Bug report ban đầu ghi nhận lỗi 404 cho:
- `POST /api/v2/product-types` / `/bulk`
- `POST /api/v2/products` / `/bulk`
- `POST /api/v2/engagements`
- `POST /api/v2/tests`
- `POST /api/v2/finding-groups`
- `POST /api/v2/findings/{id}/notes`

Sau khi đối chiếu source code, tất cả đều đã implement:
- Gateway router đã có đủ bulk & single routes
- Finding service router đã fix trailing-slash bằng khai báo flat (`r.Get(...)` / `r.Post(...)` thay vì `r.Route()`)
- `FindingGroupHandler.Create` đã mount tại dòng 234–236
- `NoteHandler` đã mount trong `/api/v2/findings/{id}` block

**Rủi ro còn lại:** `findingGroup` hoặc `findingSeed` có thể là `nil` nếu `WireEmbedded` của finding-service không khởi tạo đầy đủ dependencies → route sẽ không được đăng ký.

---

## Việc cần làm

### Bước 1: Kiểm tra finding-service start thành công

```bash
grep "failed to wire embedded finding-service" /var/log/osv/orchestrator.log
# Nếu có lỗi → xem bước 4
```

### Bước 2: Test trực tiếp finding-service

```bash
# Health check
curl -s http://localhost:8085/health
# Kỳ vọng: {"status":"ok","service":"finding-service"}

# Test POST product-type
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8085/api/v2/product-types \
  -H "Content-Type: application/json" \
  -d '{"name":"Web Application","description":"Test"}'
# Kỳ vọng: 201 hoặc 409 — KHÔNG phải 404
```

### Bước 3: Test các routes quan trọng qua Gateway

```bash
TOKEN="<admin_token>"

# Test product-type single create
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8080/api/v2/product-types \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Web Application"}'

# Test product-type bulk create
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8080/api/v2/product-types/bulk \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"product_types":[{"name":"API"}]}'

# Test finding-groups
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:8080/api/v2/finding-groups \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Group 1","finding_ids":[]}'
```

### Bước 4: Nếu finding-group trả về 404

Kiểm tra `WireEmbedded` trong `services/finding-service` — hàm này phải truyền `findingGroup != nil` vào `NewRouter()`. Nếu `FindingGroupHandler` không được khởi tạo, thêm vào dependency injection:

```go
// Trong services/finding-service/embedded.go (hoặc tương đương)
findingGroupH := http.NewFindingGroupHandler(findingGroupRepo, log)
// Đảm bảo findingGroupH được truyền vào NewRouter(...)
```

---

## Định nghĩa hoàn thành

- [ ] Health check finding-service trả về `{"status":"ok"}`
- [ ] `POST /api/v2/product-types` trả về 201/409 (không phải 404)
- [ ] `POST /api/v2/finding-groups` trả về 201/404-with-JSON (không phải `404 page not found`)
- [ ] Log không có `failed to wire embedded finding-service`
