# BUG-004 — Asset Service: Hardcoded gRPC Target để kết nối finding-service

## Metadata
- **ID**: BUG-004
- **Service**: `asset-service`
- **File**: [`embedded.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/embedded.go)
- **Lines**: 35–38
- **Severity**: Medium
- **Category**: Hardcode / Network
- **Status**: Open

## Mô tả

`asset-service/embedded.go` hardcode địa chỉ gRPC của `finding-service`:

```go
// Lines 35-38
grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"   // HARDCODE fallback
}
```

Port `50060` là gRPC port của finding-service. Trong môi trường container (Docker Compose,
K8s), finding-service chạy tại hostname `finding-service:50060`, không phải `localhost:50060`.

## Pattern lặp lại

Cùng pattern xuất hiện ở nhiều nơi:

| File | Env Var | Hardcode Fallback |
|------|---------|-------------------|
| `asset-service/embedded.go:37` | `FINDING_SERVICE_GRPC` | `localhost:50060` |
| `finding-service/embedded.go:37` | `NATS_URL` | `nats://localhost:4222` |
| `notification-service/cmd/server/main.go:50` | `POSTGRES_DSN` | `postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable` |
| `notification-service/cmd/server/main.go:58` | `REDIS_URL` | `redis://localhost:6379/0` |
| `notification-service/cmd/server/main.go:67` | `NATS_URL` | `nats://localhost:4222` |
| `search-service/cmd/server/main.go:75` | `REDIS_ADDR` | `localhost:6379` |

## Vấn đề nghiêm trọng nhất: DSN có credentials trong code

```go
// notification-service/cmd/server/main.go:50
envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")
```

Default DSN chứa username `osv` và password `osv_dev` trực tiếp trong source code.
Mặc dù đây là dev credentials, việc commit credentials vào source code là vi phạm
security best practice.

## Tác động

1. **Security**: Dev credentials (`osv:osv_dev`) trong source code — có thể bị
   include trong Docker layers, logs, stack traces.
2. **Deployment**: Fallback localhost không hoạt động trong container environments
   dẫn đến risk scoring silently trả về 0 (asset-service log Warn nhưng không fail).
3. **Observability**: Không có warning log khi dùng hardcode fallback cho NATS/Redis.

## Fix Proposal

### 1. Xóa credentials khỏi default DSN

```go
// Thay vì:
envOr("POSTGRES_DSN", "postgres://osv:osv_dev@localhost:5432/osvdb?sslmode=disable")

// Dùng:
envOr("POSTGRES_DSN", "postgres://localhost:5432/osvdb?sslmode=disable")
// Credentials phải được set qua POSTGRES_USER + POSTGRES_PASSWORD env vars riêng
// hoặc dùng POSTGRES_DSN đầy đủ
```

### 2. Log warning khi dùng fallback

```go
grpcTarget := os.Getenv("FINDING_SERVICE_GRPC")
if grpcTarget == "" {
    grpcTarget = "localhost:50060"
    logger.Warn().Str("fallback", grpcTarget).
        Msg("FINDING_SERVICE_GRPC not set, using localhost fallback — configure in production")
}
```

### 3. Tách env-var loading ra shared helper

Tạo `shared/pkg/config/defaults.go`:
```go
func ServiceAddr(envKey, defaultHost string, defaultPort int) string {
    if v := os.Getenv(envKey); v != "" {
        return v
    }
    addr := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
    slog.Warn("config: using default address", "env_key", envKey, "default", addr)
    return addr
}
```

## References

- [asset-service/embedded.go L35-38](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/embedded.go#L35-L38)
- [finding-service/embedded.go L35-38](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/embedded.go#L35-L38)
- [notification-service/main.go L48-67](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/cmd/server/main.go#L48-L67)
