# SOL-V5-004: Sửa Permission Enum và Ollama Embedding Model

## BUG-V5-010: Permission `scan:delete` không hợp lệ
### Vấn đề
`GET /api/v1/profile` trả về `permissions: ['scan:delete']` nhưng test script kiểm tra enum không bao gồm `scan:delete`.

### Root Cause
Enum hợp lệ trong test:
```
scan:read, scan:write, scan:execute, scan:manage,
finding:read, finding:write, finding:manage, ...
```
`scan:delete` không nằm trong danh sách enum được định nghĩa.

### Giải pháp
**Option A (preferred):** Thêm `scan:delete` vào enum hợp lệ trong test script và OpenAPI spec.  
**Option B:** Đổi permission thành `scan:manage` (bao gồm cả delete).

### Files cần thay đổi
- `apps/osv/internal/gateway/auth/permissions.go` hoặc tương đương
- Seed data / migration cho admin user permissions

---

## BUG-V5-011: Ollama nomic-embed-text chưa được load
### Vấn đề
`POST /api/v2/cves/search/semantic` → 500 `{"error":"semantic search failed"}`

Ollama container chạy nhưng model `nomic-embed-text` chưa được pull.

### Root Cause
Khi khởi động container Ollama, không có step tự động pull model.

### Giải pháp
Thêm step pull model vào docker-compose startup script hoặc init script.

```bash
# Trên server sau khi docker compose up
docker exec osv-ollama ollama pull nomic-embed-text
```

### Files cần thay đổi
- `deploy/dev/docker-compose.server.yml` — thêm healthcheck/init command cho ollama
- Hoặc thêm step vào `deploy_backend.sh`
