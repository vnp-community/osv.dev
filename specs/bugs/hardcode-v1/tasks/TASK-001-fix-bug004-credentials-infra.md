# TASK-001 — Fix BUG-004: Xóa Dev Credentials & Thêm Warning Log Cho Localhost Fallback

> **Bug**: BUG-004  
> **Priority**: 🔴 High — Security violation + Silent failure trong container  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-A](../solutions/SOL-GROUP-A-config-externalization.md#bug-004)  
> **Trạng thái**: ✅ DONE — 2026-06-22  
> **Ghi chú**: Xóa `postgres://osv:osv_dev@` khỏi source. Thêm `log.Fatal()` khi thiếu DB credentials. Thêm `log.Warn()` cho REDIS_URL và NATS_URL fallback. Build pass.

## Files Cần Đọc Trước

```
services/notification-service/cmd/server/main.go     (lines 45-75)
services/asset-service/embedded.go                   (lines 33-42)
services/finding-service/embedded.go                 (lines 30-50)
services/search-service/cmd/server/main.go           (lines 70-80)
```

## Files Sẽ Bị Sửa

```
services/notification-service/cmd/server/main.go     [MODIFY]
services/asset-service/embedded.go                   [MODIFY]
services/finding-service/embedded.go                 [MODIFY — chỉ phần NATS_URL]
services/search-service/cmd/server/main.go           [MODIFY — chỉ phần REDIS_ADDR]
```

## Thay Đổi Chi Tiết

### 1. `notification-service/cmd/server/main.go`

**Mục tiêu**: Xóa credentials `osv:osv_dev` khỏi default DSN.

Tìm pattern:
```go
envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")
```

Thay bằng — build DSN từ các env vars riêng lẻ, không có credentials trong source:
```go
// POSTGRES_DSN takes priority (full DSN string)
postgresDSN := os.Getenv("POSTGRES_DSN")
if postgresDSN == "" {
    // Build from individual parts — credentials MUST come from env vars
    pgHost := envOr("POSTGRES_HOST", "localhost")
    pgPort := envOr("POSTGRES_PORT", "5432")
    pgDB   := envOr("POSTGRES_DB", "osvdb")
    pgUser := os.Getenv("POSTGRES_USER")
    pgPass := os.Getenv("POSTGRES_PASSWORD")
    if pgUser == "" || pgPass == "" {
        log.Fatal().Msg("database credentials not configured: set POSTGRES_DSN or POSTGRES_USER + POSTGRES_PASSWORD")
    }
    postgresDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
        pgUser, pgPass, pgHost, pgPort, pgDB)
}
```

Tìm pattern (REDIS_URL fallback):
```go
envOr("REDIS_URL", "redis://localhost:6379/0")
```
Thay bằng:
```go
// Giữ fallback nhưng thêm warning log
redisURL := os.Getenv("REDIS_URL")
if redisURL == "" {
    redisURL = "redis://localhost:6379/0"
    log.Warn().Str("fallback", redisURL).Msg("REDIS_URL not set, using localhost fallback — configure in production")
}
```

Tìm pattern (NATS_URL fallback):
```go
envOr("NATS_URL", "nats://localhost:4222")
```
Thay bằng:
```go
natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
    log.Warn().Str("fallback", natsURL).Msg("NATS_URL not set, using localhost fallback — configure in production")
}
```

### 2. `asset-service/embedded.go`

**Mục tiêu**: Thêm warning log khi dùng `localhost:50060` fallback cho gRPC.

Tìm:
```go
grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"
}
```

Thay bằng:
```go
grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"
    log.Warn().Str("fallback", grpcTarget).
        Msg("FINDING_SERVICE_GRPC not set, using localhost — configure for container deployment")
}
```

### 3. `finding-service/embedded.go`

**Mục tiêu**: Thêm warning log cho NATS_URL fallback.

Tìm pattern tương tự:
```go
natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
}
```

Thêm warning log sau dòng hardcode:
```go
natsURL := os.Getenv("NATS_URL")
if natsURL == "" {
    natsURL = "nats://localhost:4222"
    log.Warn().Str("fallback", natsURL).
        Msg("NATS_URL not set, using localhost fallback — configure in production")
}
```

### 4. `search-service/cmd/server/main.go`

**Mục tiêu**: Thêm warning log cho REDIS_ADDR fallback.

Tìm:
```go
redisAddr := os.Getenv("REDIS_ADDR")
if redisAddr == "" {
    redisAddr = "localhost:6379"
}
```

Thay bằng:
```go
redisAddr := os.Getenv("REDIS_ADDR")
if redisAddr == "" {
    redisAddr = "localhost:6379"
    log.Warn().Str("fallback", redisAddr).
        Msg("REDIS_ADDR not set, using localhost fallback — configure in production")
}
```

## Quy Tắc Khi Thực Thi

1. **Đọc file thực tế trước** — đừng assume pattern, tìm chính xác dòng cần sửa
2. **Chỉ sửa các phần liên quan** — không refactor code khác
3. **Giữ nguyên tên variable** — chỉ thay đổi logic, không đổi tên
4. **Kiểm tra import** — thêm `"fmt"` nếu chưa có trong notification-service

## Verification

```bash
# Build tất cả services bị sửa
go build ./services/notification-service/cmd/server/
go build ./services/asset-service/...
go build ./services/finding-service/...
go build ./services/search-service/cmd/server/

# Test: notification-service fail nếu thiếu credentials
cd services/notification-service
POSTGRES_DSN="" POSTGRES_USER="" POSTGRES_PASSWORD="" go run ./cmd/server/ 2>&1 | grep -i "fatal\|credentials"
# → phải thấy log.Fatal về missing credentials

# Test: warning log xuất hiện khi dùng localhost
REDIS_URL="" go run ./cmd/server/ 2>&1 | grep -i "warn\|fallback"
# → phải thấy WARN về REDIS_URL
```

## Acceptance Criteria

- [ ] String `"postgres://osv:osv_dev@"` không còn trong source code
- [ ] `notification-service` fail với log.Fatal khi thiếu DB credentials
- [ ] Mọi localhost fallback đều có `log.Warn()` kèm env key name
- [ ] Tất cả services build thành công
