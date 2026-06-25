# TASK-INIT-002 — identity-service: Tạo init.sh + Fix migrations + JWKS/Health endpoint + Env vars

> **Solution**: [SOL-INIT-002](../solutions/SOL-INIT-002-identity-bootstrap.md)  
> **Files thực tế**: `handlers.go` (287 dòng), `main.go` (180 dòng), `001_initial_schema.sql` (105 dòng)

---

## Tổng quan

Task này bao gồm 4 thay đổi độc lập cho identity-service:
1. **Tạo** `scripts/init.sh` — bootstrap script (NEW file)
2. **Sửa** `migrations/001_initial_schema.sql` — thêm `CREATE SCHEMA IF NOT EXISTS auth` trước `SET search_path`
3. **Sửa** `cmd/server/main.go` — thêm `IDENTITY_DATABASE_URL` fallback tại dòng 50 và 114, 132
4. **Sửa** `internal/delivery/http/handlers.go` — thêm JWKS + Health handlers + routes vào Router()

---

## Bước 1 — Tạo `services/identity-service/scripts/init.sh`

**Action**: Tạo file mới (executable)

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/scripts/init.sh`

Nội dung đầy đủ từ SOL-INIT-002 (đã có trong giải pháp). Script thực hiện 4 bước:
- Tạo PostgreSQL schema `auth` + extensions
- Apply migrations theo thứ tự
- Generate RSA-4096 key pair nếu chưa có
- Seed admin account với Argon2id hash

**Sau khi tạo**: `chmod +x services/identity-service/scripts/init.sh`

---

## Bước 2 — Sửa `migrations/001_initial_schema.sql`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/migrations/001_initial_schema.sql`

**Vấn đề thực tế**: Dòng 4 gọi `SET search_path TO auth;` nhưng schema `auth` chưa được tạo → fail trên DB mới.

**Thay đổi**: Chèn vào **trước dòng 4** (`SET search_path TO auth;`):

```diff
 -- auth service initial schema
 -- Run: psql $DATABASE_URL -f 001_initial_schema.sql
 
+-- Ensure schema exists before setting search_path
+CREATE SCHEMA IF NOT EXISTS auth;
+
 SET search_path TO auth;
```

**Kết quả**: Dòng 4 trở thành dòng 7, nội dung file từ dòng 4 trở đi giữ nguyên.

---

## Bước 3 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/cmd/server/main.go`

### Thay đổi 3.1 — Dòng 50: DATABASE_URL → IDENTITY_DATABASE_URL fallback

```diff
-	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/authdb?sslmode=disable")
+	dbURL := getEnvFallback("IDENTITY_DATABASE_URL", "DATABASE_URL",
+		"postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")
```

### Thay đổi 3.2 — Dòng 114: GRPC_PORT → IDENTITY_GRPC_PORT fallback

```diff
-	grpcPort := getEnv("GRPC_PORT", "9001")
+	grpcPort := getEnvFallback("IDENTITY_GRPC_PORT", "GRPC_PORT", "9001")
```

### Thay đổi 3.3 — Dòng 132: HTTP_PORT → IDENTITY_HTTP_PORT fallback

```diff
-	httpPort := getEnv("HTTP_PORT", "9101")
+	httpPort := getEnvFallback("IDENTITY_HTTP_PORT", "HTTP_PORT", "9101")
```

### Thay đổi 3.4 — Thêm helper `getEnvFallback` sau `getEnv` (sau dòng 179)

```go
// getEnvFallback returns the value of the first non-empty env var from keys,
// or defaultVal if all are empty.
func getEnvFallback(defaultVal string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return defaultVal
}
```

**Lưu ý**: Hàm `getEnv` hiện tại ở dòng 174 vẫn giữ nguyên (không xóa).

---

## Bước 4 — Sửa `internal/delivery/http/handlers.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service/internal/delivery/http/handlers.go`

### Thay đổi 4.1 — Import: thêm `jwt` package

```diff
 import (
     "encoding/json"
     "net/http"
     "strings"

     "github.com/go-chi/chi/v5"
     "github.com/go-chi/chi/v5/middleware"
     "github.com/rs/zerolog"

     "github.com/osv/identity-service/internal/usecase/apikey"
     "github.com/osv/identity-service/internal/usecase/login"
     "github.com/osv/identity-service/internal/usecase/logout"
     "github.com/osv/identity-service/internal/usecase/oauth2"
     "github.com/osv/identity-service/internal/usecase/refresh"
     "github.com/osv/identity-service/internal/usecase/register"
     "github.com/osv/identity-service/internal/usecase/totp"
+    "github.com/osv/identity-service/internal/infrastructure/jwt"
 )
```

### Thay đổi 4.2 — Handler struct: thêm field `jwtSvc`

```diff
 type Handler struct {
     registerUC *register.UseCase
     loginUC    *login.UseCase
     refreshUC  *refresh.UseCase
     logoutUC   *logout.UseCase
     apikeyUC   *apikey.UseCase
     totpUC     *totp.UseCase
     oauth2UC   *oauth2.UseCase
     logger     zerolog.Logger
+    jwtSvc     *jwt.Service
 }
```

### Thay đổi 4.3 — NewHandler: thêm param `jwtSvc`

```diff
 func NewHandler(
     registerUC *register.UseCase,
     loginUC *login.UseCase,
     refreshUC *refresh.UseCase,
     logoutUC *logout.UseCase,
     apikeyUC *apikey.UseCase,
     totpUC *totp.UseCase,
     oauth2UC *oauth2.UseCase,
     logger zerolog.Logger,
+    jwtSvc *jwt.Service,
 ) *Handler {
     return &Handler{
         registerUC: registerUC,
         loginUC:    loginUC,
         refreshUC:  refreshUC,
         logoutUC:   logoutUC,
         apikeyUC:   apikeyUC,
         totpUC:     totpUC,
         oauth2UC:   oauth2UC,
         logger:     logger,
+        jwtSvc:     jwtSvc,
     }
 }
```

### Thay đổi 4.4 — Router(): thêm 2 routes trước public routes (sau dòng 61)

```diff
 func (h *Handler) Router() http.Handler {
     r := chi.NewRouter()
     r.Use(middleware.RequestID)
     r.Use(middleware.RealIP)
     r.Use(middleware.Recoverer)

+    // Well-known and health endpoints (no auth required)
+    r.Get("/.well-known/jwks.json", h.JWKS)
+    r.Get("/health", h.Health)

     // Public routes
     r.Post("/auth/register", h.Register)
```

### Thay đổi 4.5 — Thêm 2 handler methods (append vào cuối file trước closing brace)

Thêm vào **trước** `func jsonResponse(...)` (dòng 278):

```go
// JWKS handles GET /.well-known/jwks.json
// Returns RSA public key as JWKS for external JWT validators (apps/osv gateway).
// Spec: 01-architecture.md §3.1 — Gateway validates JWT via JWKS
func (h *Handler) JWKS(w http.ResponseWriter, r *http.Request) {
	if h.jwtSvc == nil {
		jsonError(w, "JWT service not configured", http.StatusInternalServerError)
		return
	}
	jwksBytes, err := h.jwtSvc.PublicKeyJWKS()
	if err != nil {
		h.logger.Error().Err(err).Msg("JWKS generation failed")
		jsonError(w, "failed to generate JWKS", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(jwksBytes) //nolint:errcheck
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "identity-service",
	})
}

```

**Lưu ý quan trọng**: File `handlers.go` import package `jwt` sẽ conflict nếu `delivery/http` package có alias. Nếu bị lỗi import cycle, tạo `handler/health.go` riêng.

---

## Acceptance Criteria

- [ ] `scripts/init.sh` tồn tại, executable (`chmod +x`)
- [ ] `migrations/001_initial_schema.sql` có `CREATE SCHEMA IF NOT EXISTS auth;` trước `SET search_path`
- [ ] `main.go` dùng `IDENTITY_DATABASE_URL` trước `DATABASE_URL`
- [ ] `main.go` dùng `IDENTITY_GRPC_PORT` và `IDENTITY_HTTP_PORT`
- [ ] `GET /.well-known/jwks.json` trả về JSON có `{"keys":[...]}`
- [ ] `GET /health` trả về `{"status":"ok","service":"identity-service"}`
- [ ] `go build ./cmd/server` không lỗi

---

## Kiểm tra sau khi thực thi

```bash
# Build
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/identity-service
go build ./cmd/server/...

# Chạy init (cần PostgreSQL running)
./scripts/init.sh

# Test compile + static check
go vet ./...
```
