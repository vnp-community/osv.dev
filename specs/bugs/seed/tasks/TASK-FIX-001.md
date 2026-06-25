# TASK-FIX-001: Verify Identity Service — Admin Users & API Keys

**Trạng thái:** ✅ Code đã hoàn chỉnh — Cần verify deployment  
**Liên kết bug:** [BUG-SEED-001 §1 & §2](../001-seed-script-missing-routes.md)  
**Liên kết giải pháp:** [SOL-BUG-SEED-001 §Bug1&2](../solutions/SOL-BUG-SEED-001.md)  
**Priority:** P1 (Verify sau khi deploy binary mới)
**Cập nhật:** 2026-06-19 — Bổ sung thông tin từ `deploy/dev/`

---

## Bối cảnh

Bug report ban đầu ghi nhận lỗi 404 cho các routes:
- `POST /api/v1/admin/users`
- `POST /api/v1/admin/users/bulk`
- `POST /api/v1/admin/users/{id}/api-keys`

Sau khi đối chiếu source code, tất cả routes và handlers **đã được implement**:
- `apps/osv/internal/gateway/router.go` (dòng 100–102): routes đã có
- `services/identity-service/adapter/handler/http/router.go` (dòng 132–139): handlers đã mount
- `services/identity-service/adapter/handler/http/admin_handler.go`: `CreateUser`, `BulkCreateUsers`, `CreateAPIKeyForUser` đã implement

---

## Thông tin deployment thực tế (từ deploy/dev/)

| Layer | Cấu hình |
|-------|-----------|
| `wire.go` dòng 73 | `identity-service` bind port `8081` (nội bộ) |
| `docker-compose.server.yml` dòng 149 | Map `9101:8081` → container port `8081`, host port `9101` |
| `.env` dòng 72 | `IDENTITY_SERVICE_HTTP=http://localhost:9101` |
| Deploy script | `deploy_backend.sh` → cross-compile → rsync → `docker compose up` |

> **Lưu ý:** Trong container, identity bind `:8081`. Từ ngoài Docker, phải dùng `:9101`. Seed script chạy trên **host** nên cần dùng gateway `:8080`.

---

## Việc cần làm

### Bước 1: Deploy binary mới lên server (sau khi TASK-FIX-003 hoàn tất)

```bash
# Từ máy local — cross-compile và deploy
cd /Users/binhnt/Lab/sec/cve/osv.dev
bash deploy/dev/deploy_backend.sh
```

Script sẽ:
1. Cross-compile `apps/osv` → `deploy/dev/osv-server` (Linux/amd64)
2. rsync binary + docker-compose + .env lên `172.20.2.48:/opt/osv-backend/`
3. `docker compose down && up -d` trên server
4. Chờ health check tại `:8080/health`

### Bước 2: Verify identity routes từ host server

```bash
# SSH lên server
ssh ubuntu@172.20.2.48

# Lấy admin token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123!ChangeMe"}' | jq -r '.token')
echo "Token: ${TOKEN:0:30}..."

# Test POST /admin/users
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"email":"verify@test.com","password":"Test@123","role":"user","is_active":true}'
# Kỳ vọng: 201 hoặc 409 — KHÔNG phải 404

# Test POST /admin/users/bulk
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v1/admin/users/bulk \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"users":[{"email":"bulk1@test.com","password":"Test@123","role":"user"}]}'
# Kỳ vọng: 207 MultiStatus
```

### Bước 3: Kiểm tra log container nếu vẫn lỗi

```bash
# SSH lên server
cd /opt/osv-backend
docker compose -f docker-compose.server.yml logs --tail=50 osv-server | grep -E "identity|error|8081"
```

---

## Định nghĩa hoàn thành

- [ ] `deploy_backend.sh` hoàn thành thành công (health check pass)
- [ ] `POST http://172.20.2.48:8080/api/v1/admin/users` trả về 201/409
- [ ] `POST http://172.20.2.48:8080/api/v1/admin/users/bulk` trả về 207
- [ ] `POST http://172.20.2.48:8080/api/v1/admin/users/{id}/api-keys` trả về 201
- [ ] Log không có `failed to wire embedded identity-service`
