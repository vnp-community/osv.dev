# TASK-INIT-006 — notification-service: NOTIFICATION_ prefix + init.sh

> **Solution**: [SOL-INIT-006](../solutions/SOL-INIT-004-to-006-search-ranking-notification.md)  
> **Files thực tế**: `cmd/server/main.go` (147 dòng)

---

## Phân tích hiện trạng

Notification-service đã xử lý tốt:
- ✅ `NATS_ENABLED` guard (dòng 66-71): nếu NATS fail và `NATS_ENABLED != "true"` → warn + continue
- ✅ gRPC health check đã register (dòng 122-125)
- ✅ `/health` endpoint cần xem trong router của `deliverhttp.SetupRouter`

Cần fix:
- ✗ `DATABASE_URL` (dòng 46) không đọc `NOTIFICATION_DATABASE_URL`
- ✗ `HTTP_PORT` (dòng 102) không đọc `NOTIFICATION_HTTP_PORT`
- ✗ `GRPC_PORT` (dòng 116) không đọc `NOTIFICATION_GRPC_PORT`

---

## Bước 1 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go`

### Thay đổi 1.1 — Dòng 46: DATABASE_URL → NOTIFICATION_DATABASE_URL fallback

```diff
-	dbURL := envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/globalcve?sslmode=disable")
+	dbURL := envOr("NOTIFICATION_DATABASE_URL",
+		envOr("DATABASE_URL",
+			envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")))
```

### Thay đổi 1.2 — Dòng 102: HTTP_PORT → NOTIFICATION_HTTP_PORT

```diff
-	httpPort := envOr("HTTP_PORT", "8086")
+	httpPort := envOr("NOTIFICATION_HTTP_PORT", envOr("HTTP_PORT", "8086"))
```

### Thay đổi 1.3 — Dòng 116: GRPC_PORT → NOTIFICATION_GRPC_PORT

```diff
-	grpcPort := envOr("GRPC_PORT", "50063")
+	grpcPort := envOr("NOTIFICATION_GRPC_PORT", envOr("GRPC_PORT", "50063"))
```

---

## Bước 2 — Kiểm tra /health trong router

Kiểm tra xem `deliverhttp.SetupRouter` đã có `/health` chưa:

```bash
grep -r "health" /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/
```

Nếu chưa có → **thêm** vào `SetupRouter()` function hoặc wrap trong `main.go`:

Trong `main.go` sau dòng 100 (`router := deliverhttp.SetupRouter(...)`):

```go
// Wrap với health check nếu router chưa có
// Kiểm tra: nếu SetupRouter đã có GET /health thì bỏ qua bước này
mainRouter := chi.NewRouter()
mainRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","service":"notification-service"}`)
})
mainRouter.Mount("/", router)
// Thay router → mainRouter trong httpSrv
```

**Lưu ý**: Nếu `SetupRouter` dùng chi router và đã có `/health`, không cần thêm. Kiểm tra code trước khi áp dụng.

---

## Bước 3 — Tạo `scripts/init.sh`

**Action**: Tạo file mới

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/scripts/init.sh`

Script:
1. Tạo PostgreSQL schema `notif` + extension uuid-ossp
2. Apply migrations theo thứ tự (bỏ qua `.down.sql`)
3. Check NATS (graceful nếu NATS_ENABLED=false)

**Sau khi tạo**: `chmod +x services/notification-service/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `NOTIFICATION_DATABASE_URL` được ưu tiên hơn `DATABASE_URL`
- [ ] `NOTIFICATION_HTTP_PORT` và `NOTIFICATION_GRPC_PORT` được đọc từ env
- [ ] `NATS_ENABLED=false` → service start được dù NATS không chạy ✓ (đã có sẵn)
- [ ] `GET /health` trả về 200 JSON
- [ ] `scripts/init.sh` tồn tại và executable
- [ ] `go build ./cmd/server` không lỗi

---

# TASK-INIT-007 — ai-service: AI_GRPC_PORT env var + init.sh

> **Solution**: [SOL-INIT-007](../solutions/SOL-INIT-007-to-008-ai-gateway-osv.md)  
> **Files thực tế**: `cmd/server/main.go` (46 dòng — rất nhỏ), `internal/infra/ai/factory.go` (66 dòng — Validate() đã có!)

---

## Phân tích hiện trạng

Phát hiện quan trọng từ đọc code thực tế:

- ✅ `factory.go` đã có `Validate()` method đúng chuẩn (dòng 42-58) — **KHÔNG CẦN MODIFY**
- ✅ `envOrDefault()` trong `factory.go` đọc OS env đúng cách — **KHÔNG BUG**
- ✗ `cmd/server/main.go` dòng 31: đọc `GRPC_PORT`, không đọc `AI_GRPC_PORT`
- ✗ Không có gRPC server thực sự (chỉ log port, không listen)
- ✗ Không có `scripts/init.sh`

---

## Bước 1 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/cmd/server/main.go`

### Thay đổi 1.1 — Dòng 31: thêm AI_GRPC_PORT

```diff
-	grpcPort := envOrDefault("GRPC_PORT", "50052")
+	grpcPort := envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052"))
```

### Thay đổi 1.2 — Dòng 30-32: thêm gRPC listener thực sự

```diff
-	// gRPC server would be wired here
-	grpcPort := envOrDefault("GRPC_PORT", "50052")
-	log.Info().Str("port", grpcPort).Msg("ai-service gRPC listening")
+	grpcPort := envOrDefault("AI_GRPC_PORT", envOrDefault("GRPC_PORT", "50052"))
+
+	lis, err := net.Listen("tcp", ":"+grpcPort)
+	if err != nil {
+		log.Fatal().Err(err).Str("port", grpcPort).Msg("gRPC listen failed")
+	}
+
+	// gRPC server với health check
+	s := grpc.NewServer()
+	healthSvc := health.NewServer()
+	healthpb.RegisterHealthServer(s, healthSvc)
+	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
+	healthSvc.SetServingStatus("ai.AIService", healthpb.HealthCheckResponse_SERVING)
+	// TODO: Register AIServiceServer after proto generation
+
+	go func() {
+		log.Info().Str("port", grpcPort).Msg("ai-service gRPC started")
+		if err := s.Serve(lis); err != nil {
+			log.Error().Err(err).Msg("gRPC serve error")
+		}
+	}()
```

### Thay đổi 1.3 — Thêm imports

```diff
 import (
+	"net"
 	"os"
 	"os/signal"
 	"syscall"

 	"github.com/rs/zerolog"
 	"github.com/rs/zerolog/log"

+	"google.golang.org/grpc"
+	"google.golang.org/grpc/health"
+	healthpb "google.golang.org/grpc/health/grpc_health_v1"
+
 	aifactory "github.com/osv/ai-service/internal/infra/ai"
 )
```

### Thay đổi 1.4 — Graceful stop trước `<-quit`

```diff
+	s.GracefulStop()
 	log.Info().Msg("ai-service shutting down")
```

---

## Bước 2 — Tạo `scripts/init.sh`

**Action**: Tạo file mới

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/ai-service/scripts/init.sh`

Script:
1. Validate AI backend config (`AI_BACKEND`, required fields)
2. Test Ollama connectivity + model availability (nếu `AI_BACKEND=ollama`)
3. Check Redis cho embedding cache
4. Validate OpenAI/Vertex credentials (nếu applicable)

**Sau khi tạo**: `chmod +x services/ai-service/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `AI_GRPC_PORT` được đọc từ env
- [ ] gRPC server thực sự listen trên port (không chỉ log)
- [ ] `grpc_health_probe -addr=:50052` trả về `SERVING`
- [ ] `scripts/init.sh` tồn tại và executable
- [ ] `go build ./cmd/server` không lỗi
- [ ] `factory.go` — **KHÔNG thay đổi** (Validate() đã đúng)
