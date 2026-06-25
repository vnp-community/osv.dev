# TASK-INDEX: BUG-SEED-001 — Danh sách tác vụ thực thi

**Nguồn:** [SOL-BUG-SEED-001 v2](../solutions/SOL-BUG-SEED-001.md)  
**Cập nhật:** 2026-06-19 — Sau khi phân tích `deploy/dev/`  
**Deploy target:** `172.20.2.48` (docker-compose.server.yml)  
**Deploy script:** `bash deploy/dev/deploy_backend.sh`

---

## Thứ tự thực thi

```
TASK-FIX-003  ← ✅ CODE ĐÃ FIX (wire.go) — CẦN DEPLOY
     ↓
TASK-FIX-004  ← ⚠️ CÒN BUG (notification embedded.go nil handlers) — CẦN FIX CODE
     ↓
bash deploy/dev/deploy_backend.sh  ← Deploy binary mới lên 172.20.2.48
     ↓
TASK-FIX-001  ← Verify identity routes sau deploy
     ↓
TASK-FIX-002  ← Verify finding-service runtime wiring sau deploy
```

---

## Bảng tóm tắt (cập nhật)

| Task | Mô tả | Loại | Priority | Trạng thái |
|------|--------|------|----------|-----------|
| [TASK-FIX-003](./TASK-FIX-003.md) | Fix SLA Port Mismatch trong `wire.go` | Code change | P0 | ✅ Code fixed — cần deploy |
| [TASK-FIX-004](./TASK-FIX-004.md) | Fix Notification `nil` AlertHandler + SSEHandler trong `embedded.go` | Code change | P1 | ⚠️ Cần fix code |
| [TASK-FIX-001](./TASK-FIX-001.md) | Verify Identity Service routes | Verify | P1 | ✅ Code OK — verify sau deploy |
| [TASK-FIX-002](./TASK-FIX-002.md) | Verify Finding Service wiring | Verify | P1 | ✅ Code OK — verify sau deploy |

---

## Phát hiện mới từ phân tích deploy/dev/

### 1. Bug bổ sung trong `notification/embedded.go` (TASK-FIX-004)

```go
// embedded.go dòng 45 — HIỆN TẠI (SAI)
r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, nil, nil, rhHandler)
//                                                              ^^^  ^^^
//                                                              ah   sse  ← nil → routes /api/v2/notifications không mount
```

`AlertsHandler` và `SSEHandler` đang là `nil` → toàn bộ routes `/api/v2/notifications/*` không được đăng ký.  
Cần khởi tạo hai handlers này trong `embedded.go`.

### 2. Port consistency trong embedded mode

Trong embedded mode (`OSV_MODE=microservices`), port được hardcode bởi `NewEmbeddedService()` trong `wire.go` — KHÔNG đọc từ env `NOTIFICATION_HTTP_PORT`. Do đó:

- `notification-service:8087` trong gateway → `notifSvc` bind `:8087` trong wire.go → **Nhất quán**
- Env `NOTIFICATION_HTTP_PORT=8086` trong `.env` chỉ có hiệu lực khi chạy standalone binary — **bỏ qua trong embedded mode**

### 3. Deploy infrastructure

```
Server backend:  172.20.2.48  (docker compose)
Server gateway:  172.20.2.16  (nginx proxy)
Domain:          c12.openledger.vn → 172.20.2.16 → 172.20.2.48:8080
Binary path:     /opt/osv-backend/osv-server
Deploy cmd:      bash deploy/dev/deploy_backend.sh  (từ máy local)
```

---

## Smoke test tổng thể sau deploy

```bash
# SSH lên server hoặc chạy từ ngoài qua domain
BASE="http://172.20.2.48:8080"
# Hoặc qua domain: BASE="https://c12.openledger.vn"

TOKEN=$(curl -s -X POST $BASE/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123!ChangeMe"}' | jq -r '.token')

echo "Token acquired: ${TOKEN:0:20}..."

check() {
  local DESC="$1"; local METHOD="$2"; local URL="$3"; local DATA="$4"
  CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X "$METHOD" "$BASE$URL" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    ${DATA:+-d "$DATA"})
  echo "[$CODE] $DESC"
}

echo "=== Identity ==="
check "POST admin/users"      POST "/api/v1/admin/users" \
  '{"email":"smoke1@t.com","password":"Test@123","role":"user","is_active":true}'
check "POST admin/users/bulk" POST "/api/v1/admin/users/bulk" \
  '{"users":[{"email":"smoke2@t.com","password":"Test@123","role":"user"}]}'

echo "=== Finding ==="
check "POST product-types"    POST "/api/v2/product-types"  '{"name":"SmokeTest PT"}'
check "POST finding-groups"   POST "/api/v2/finding-groups" '{"name":"SmokeTest FG","finding_ids":[]}'

echo "=== SLA ==="
check "POST sla-configurations" POST "/api/v2/sla-configurations" \
  '{"name":"SmokeTest SLA","critical_days":7,"high_days":30,"medium_days":90,"low_days":180}'

echo "=== Notification ==="
check "POST notification-rules" POST "/api/v2/notification-rules" \
  '{"name":"Smoke Rule","event_type":"finding.created","channel":"email","enabled":true}'
check "POST subscriptions"    POST "/api/v2/subscriptions" \
  '{"event_type":"finding.sla.breached","channels":["email"]}'
check "POST webhooks"         POST "/api/v1/webhooks" \
  '{"name":"Smoke WH","url":"https://httpbin.org/post","secret":"s3cr3t"}'
check "GET notifications"     GET  "/api/v2/notifications" ""

echo "=== Asset ==="
check "POST assets"           POST "/api/v1/assets" \
  '{"name":"Smoke Asset","asset_type":"hostname","value":"smoke.test"}'
```

**Kỳ vọng:** Tất cả status code là `2xx` — không có `404` hay `502`.

---

## Ghi chú cho AI thực thi

1. **Ưu tiên:** TASK-FIX-004 (fix code) → TASK-FIX-003 deploy → verify
2. **TASK-FIX-003:** Code đã fix trong `wire.go`, chỉ cần chạy `deploy_backend.sh`
3. **TASK-FIX-004:** Cần tìm constructor đúng trong `notification-service/internal/delivery/http/` trước khi sửa
4. Sau khi fix và deploy, chạy smoke test ở trên để xác nhận
5. Tham chiếu [SOL-BUG-SEED-001.md](../solutions/SOL-BUG-SEED-001.md) cho full context
