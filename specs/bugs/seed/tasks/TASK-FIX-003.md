# TASK-FIX-003: Fix SLA Service Port Mismatch [CRITICAL CODE CHANGE]

**Trạng thái:** ✅ CODE ĐÃ FIX — Cần deploy binary mới  
**Liên kết bug:** [BUG-SEED-001 §4](../001-seed-script-missing-routes.md)  
**Liên kết giải pháp:** [SOL-BUG-SEED-001 §Bug4a](../solutions/SOL-BUG-SEED-001.md)  
**Priority:** P0 — Nguyên nhân trực tiếp gây lỗi 404 cho toàn bộ SLA routes  
**Cập nhật:** 2026-06-19 — Code đã fix trong `wire.go`, cần deploy

---

## Trạng thái sau khi thực thi

### Code đã thay đổi trong `apps/osv/internal/config/wire.go`:

```diff
- aiSvc := adapters.NewEmbeddedService("ai-service", 8086)
+ aiSvc := adapters.NewEmbeddedService("ai-service", 9103) // port 9103 per architecture.md

- slaSvc := adapters.NewEmbeddedService("sla-service", 8093) // 8093 to avoid conflict with AI (8086)
+ slaSvc := adapters.NewEmbeddedService("sla-service", 8086) // port 8086 per architecture.md

- SLAAddr: "http://localhost:8093", // sla-service binds :8093
+ SLAAddr: "http://localhost:8086", // sla-service binds :8086 per architecture.md
```

**Build verification:** ✅ `go build ./...` và `go vet ./...` trong `apps/osv/` — PASS (không lỗi)

---

## Bối cảnh — Root Cause (đã phân tích)

Có mâu thuẫn port giữa:
- `gateway/router.go` (dòng 407–416): `sla-service:8086`
- `wire.go` cũ (dòng 106): `sla-service:8093`

Nguyên nhân: AI service chiếm port `8086` trong `wire.go` (sai, AI phải dùng `9103`), khiến SLA bị đẩy sang `8093`. Gateway router không được cập nhật → 404.

---

## Thông tin deployment thực tế (từ deploy/dev/)

| File | Nội dung liên quan |
|------|-------------------|
| `deploy_backend.sh` | Cross-compile `apps/osv` → rsync → `docker compose up` trên `172.20.2.48` |
| `docker-compose.server.yml` | Binary `osv-server` chạy trong container distroless, expose `:8080` |
| `.env` | Không có `SLA_ADDR` env — wire.go dùng hardcode `http://localhost:8086` |

Deploy workflow:
```
local: go build → deploy/dev/osv-server (Linux/amd64)
       ↓ rsync
172.20.2.48:/opt/osv-backend/osv-server
       ↓ docker compose up
container: /service (distroless) → binds :8080, :8082, :8081→9101
```

---

## Việc cần làm

### Bước 1: Deploy binary mới (bắt buộc)

```bash
# Từ máy local — chạy deploy script
cd /Users/binhnt/Lab/sec/cve/osv.dev
bash deploy/dev/deploy_backend.sh
```

Script sẽ:
1. `GOOS=linux GOARCH=amd64 go build -o ../../deploy/dev/osv-server ./cmd/osv/`
2. `rsync osv-server docker-compose.server.yml .env ubuntu@172.20.2.48:/opt/osv-backend/`
3. `docker compose down && up -d`
4. Wait health check tại `http://localhost:8080/health`

### Bước 2: Verify SLA service trên server

```bash
ssh ubuntu@172.20.2.48

# Health check tổng thể
curl -s http://localhost:8080/health

# Lấy token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@123!ChangeMe"}' | jq -r '.token')

# Test POST sla-configuration
curl -s -w "\n[HTTP %{http_code}]\n" \
  -X POST http://localhost:8080/api/v2/sla-configurations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Default SLA","critical_days":7,"high_days":30,"medium_days":90,"low_days":180,"is_default":true}'
# Kỳ vọng: 201 Created — KHÔNG phải 404/502
```

### Bước 3: Kiểm tra log nếu vẫn lỗi

```bash
cd /opt/osv-backend
docker compose -f docker-compose.server.yml logs --tail=100 osv-server \
  | grep -E "sla|8086|8093|error"
```

---

## Định nghĩa hoàn thành

- [x] `wire.go` dòng 95: `ai-service` dùng port `9103`
- [x] `wire.go` dòng 106: `sla-service` dùng port `8086`
- [x] `wire.go` dòng 156: `SLAAddr` = `http://localhost:8086`
- [x] Build `go build ./...` trong `apps/osv` — PASS
- [ ] `deploy_backend.sh` chạy thành công (rsync + docker compose up + health)
- [ ] `POST http://172.20.2.48:8080/api/v2/sla-configurations` trả về 201
