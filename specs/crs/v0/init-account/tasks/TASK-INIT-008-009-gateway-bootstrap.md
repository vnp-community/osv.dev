# TASK-INIT-008 — gateway-service & apps/osv: JWT_SECRET guard + init scripts

> **Solution**: [SOL-INIT-008](../solutions/SOL-INIT-007-to-008-ai-gateway-osv.md)  
> **Files thực tế**: `gateway-service/embedded.go` (139 dòng), `gateway-service/internal/auth/osv_middleware.go` (210 dòng)

---

## Phân tích hiện trạng

**Phát hiện quan trọng từ đọc code thực tế**:

`embedded.go` dòng 49-54: địa chỉ upstream bị **hardcode** — không đọc từ `EmbeddedConfig`:

```go
identityHTTP     := "http://localhost:8081"  // ← BUG: hardcode, không dùng cfg.IdentityAddr
dataHTTP         := "http://localhost:8082"  // ← hardcode
findingHTTP      := "http://localhost:8085"  // ← hardcode
scanHTTP         := "http://localhost:8088"  // ← hardcode
notificationHTTP := "http://localhost:8084"  // ← hardcode
aiHTTP           := "http://localhost:8086"  // ← hardcode
```

`auth/osv_middleware.go` dòng 40: `AuthVerify(secret string, ...)` — nhận JWT secret từ caller (không tự đọc env). Secret phải được pass đúng từ main.go.

---

## Bước 1 — Sửa `embedded.go`: dùng EmbeddedConfig thay hardcoded addresses

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/embedded.go`

### Thay đổi 1.1 — Mở rộng `EmbeddedConfig` struct (dòng 21-29)

```diff
 type EmbeddedConfig struct {
 	JWTSecret        string
 	IdentityAddr     string
 	DataAddr         string
 	SearchAddr       string
 	FindingAddr      string
 	ScanAddr         string
 	NotificationAddr string
+	AIAddr           string
+	RankingAddr      string
 }
```

### Thay đổi 1.2 — Dùng config thay hardcoded (dòng 49-54)

```diff
-	identityHTTP      := "http://localhost:8081"
-	dataHTTP          := "http://localhost:8082"
-	findingHTTP       := "http://localhost:8085"
-	scanHTTP          := "http://localhost:8088"
-	notificationHTTP  := "http://localhost:8084"
-	aiHTTP            := "http://localhost:8086"
+	identityHTTP     := coalesce(cfg.IdentityAddr, "http://localhost:9101")
+	dataHTTP         := coalesce(cfg.DataAddr, "http://localhost:8082")
+	findingHTTP      := coalesce(cfg.FindingAddr, "http://localhost:8085")
+	scanHTTP         := coalesce(cfg.ScanAddr, "http://localhost:8087")
+	notificationHTTP := coalesce(cfg.NotificationAddr, "http://localhost:8086")
+	aiHTTP           := coalesce(cfg.AIAddr, "http://localhost:8086")
```

### Thay đổi 1.3 — Thêm helper `coalesce` (append cuối file trước closing brace `}`)

```go
// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

---

## Bước 2 — Tạo `scripts/init.sh` cho gateway-service

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/scripts/init.sh`

Script đơn giản:
- Kiểm tra upstream services health
- Validate JWT_SECRET không là default

**Sau khi tạo**: `chmod +x services/gateway-service/scripts/init.sh`

---

## Bước 3 — Tạo `scripts/init.sh` cho apps/osv

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/scripts/init.sh`

Script:
1. Load `.env` từ app dir và project root
2. Validate `JWT_SECRET` — fail nếu là default (trừ `FORCE_INSECURE=true`)
3. Kiểm tra upstream services health (identity, data, search, ranking, notification)
4. Verify JWKS endpoint từ identity-service

**Sau khi tạo**: `chmod +x apps/osv/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `EmbeddedConfig.IdentityAddr`, `AIAddr`, `RankingAddr` được dùng trong `WireEmbedded()`
- [ ] Upstream addresses đọc từ config, không hardcode
- [ ] `gateway-service/scripts/init.sh` tồn tại, executable
- [ ] `apps/osv/scripts/init.sh` fail khi `JWT_SECRET=CHANGE_ME_...` và `FORCE_INSECURE` không set
- [ ] `go build ./...` trong gateway-service không lỗi

---

## Kiểm tra

```bash
# gateway-service build
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service
go build ./...

# apps/osv build  
cd /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv
go build ./...
```

---

# TASK-INIT-009 — Master Bootstrap Scripts

> **Solution**: [SOL-INIT-009](../solutions/SOL-INIT-009-master-bootstrap.md)

---

## Tổng quan

Tạo 3 files mới tại project root `scripts/`:
1. `scripts/bootstrap.sh` — Orchestrate toàn bộ init, security check, summary
2. `scripts/health-check.sh` — Verify tất cả services healthy sau khi start
3. `scripts/start-all.sh` — Start binaries theo đúng thứ tự phụ thuộc

---

## Bước 1 — Tạo `scripts/bootstrap.sh`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/scripts/bootstrap.sh`

Logic:
1. Load `.env` — fail nếu không có (chỉ dẫn `cp .env.bootstrap .env`)
2. Security validation — fail nếu `JWT_SECRET` là default value (trừ `FORCE_INSECURE=true`)
3. Warn nếu `INIT_ADMIN_PASSWORD` là default
4. Infra check: PostgreSQL, Redis, MongoDB, NATS (warn không fail, trừ PostgreSQL)
5. Gọi từng service init script theo thứ tự:
   - `services/identity-service/scripts/init.sh`
   - `services/data-service/scripts/init.sh`
   - `services/search-service/scripts/init.sh`
   - `services/ranking-service/scripts/init.sh`
   - `services/notification-service/scripts/init.sh`
   - `services/ai-service/scripts/init.sh`
   - `services/gateway-service/scripts/init.sh`
   - `apps/osv/scripts/init.sh`
6. In summary table với tất cả endpoints
7. Exit với code 1 nếu có lỗi

Nội dung đầy đủ từ SOL-INIT-009.

**Sau khi tạo**: `chmod +x scripts/bootstrap.sh`

---

## Bước 2 — Tạo `scripts/health-check.sh`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/scripts/health-check.sh`

Logic: curl `/health` endpoint của từng service, parse JSON response, báo cáo status.

```bash
check_http "identity-service"     "http://localhost:${IDENTITY_HTTP_PORT:-9101}/health"
check_http "data-service"         "http://localhost:${DATA_HTTP_PORT:-8082}/health"
check_http "search-service"       "http://localhost:${SEARCH_HTTP_PORT:-8083}/health"
check_http "ranking-service"      "http://localhost:${RANKING_PORT:-8088}/health"
check_http "notification-service" "http://localhost:${NOTIFICATION_HTTP_PORT:-8086}/health"
check_http "apps/osv gateway"     "http://localhost:${HTTP_PORT:-8080}/health"
```

Cũng test JWKS endpoint và admin login.

**Sau khi tạo**: `chmod +x scripts/health-check.sh`

---

## Bước 3 — Tạo `scripts/start-all.sh`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/scripts/start-all.sh`

Logic: Start binaries theo thứ tự phụ thuộc với background processes.  
Thứ tự:
1. `services/identity-service/server` → sleep 1s
2. `services/data-service/server`
3. `services/search-service/server`
4. `services/ranking-service/server`
5. `services/notification-service/server`
6. `services/ai-service/server`
7. `services/gateway-service/server`
8. `apps/osv/server` → sleep 5s
9. Gọi `health-check.sh`

**Sau khi tạo**: `chmod +x scripts/start-all.sh`

---

## Acceptance Criteria

- [ ] `scripts/bootstrap.sh` tồn tại, executable
- [ ] Bootstrap fail khi không có `.env` (error message + hướng dẫn)
- [ ] Bootstrap fail khi `JWT_SECRET` là default
- [ ] Bootstrap success in summary table với tất cả endpoints
- [ ] `scripts/health-check.sh` curl và parse JSON health responses
- [ ] `scripts/start-all.sh` start theo đúng thứ tự

---

## End-to-End Test

```bash
# Setup
cp .env.bootstrap .env
echo "JWT_SECRET=$(openssl rand -hex 32)" >> .env

# Bootstrap
./scripts/bootstrap.sh

# Build all
cd /Users/binhnt/Lab/sec/cve/osv.dev
for svc in services/*/; do
  (cd "$svc" && go build -o server ./cmd/server 2>/dev/null || true)
done
(cd apps/osv && go build -o server ./cmd/server 2>/dev/null || true)

# Start + verify
./scripts/start-all.sh
```
