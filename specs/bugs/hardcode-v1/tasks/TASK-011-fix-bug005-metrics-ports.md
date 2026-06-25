# TASK-011 — Fix BUG-005: Prometheus Metrics Port Configurable qua Env Var

> **Bug**: BUG-005  
> **Priority**: 🟢 Low — Tech debt; port conflict khi chạy nhiều instances  
> **Depends on**: TASK-000  
> **Solution ref**: [SOL-GROUP-A](../solutions/SOL-GROUP-A-config-externalization.md#bug-005)

## Files Cần Đọc Trước

```
services/gateway-service/cmd/server/main.go       (xem cách gọi StartMetricsServer)
services/search-service/cmd/server/main.go        (xem cách gọi StartMetricsServer)
services/data-service/cmd/server/main.go          (xem cách gọi StartMetricsServer)
services/notification-service/cmd/server/main.go  (xem cách gọi StartMetricsServer)
services/shared/pkg/observability/               (xem signature của StartMetricsServer)
```

## Files Sẽ Bị Sửa

```
services/gateway-service/cmd/server/main.go       [MODIFY]
services/search-service/cmd/server/main.go        [MODIFY]
services/data-service/cmd/server/main.go          [MODIFY]
services/notification-service/cmd/server/main.go  [MODIFY]
```

**Optional**: Sửa `shared/pkg/observability` nếu muốn centralize METRICS_PORT reading.

## Thay Đổi Chi Tiết

### Bước 1: Xem signature thực tế của `StartMetricsServer`

```bash
grep -rn "func StartMetricsServer\|StartMetricsServer(" \
    services/shared/pkg/observability/
```

### Bước 2: Sửa từng `main.go`

Pattern hiện tại (4 files):
```go
observability.StartMetricsServer(9090)  // gateway
observability.StartMetricsServer(9091)  // search
observability.StartMetricsServer(9092)  // data
observability.StartMetricsServer(9094)  // notification
```

**Option A — Đọc env var trong mỗi main.go** (ít thay đổi nhất):

```go
// Thêm helper function vào main.go hoặc import từ shared/pkg/config
metricsPort := envOrInt("METRICS_PORT", 9090)  // default khác nhau cho mỗi service
observability.StartMetricsServer(metricsPort)

// Helper (nếu chưa có trong service):
func envOrInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            return n
        }
    }
    return def
}
```

Áp dụng với port mặc định mỗi service:
- `gateway-service/main.go` → `envOrInt("METRICS_PORT", 9090)`
- `search-service/main.go` → `envOrInt("METRICS_PORT", 9091)`
- `data-service/main.go` → `envOrInt("METRICS_PORT", 9092)`
- `notification-service/main.go` → `envOrInt("METRICS_PORT", 9094)`

**Option B — Dùng `config.EnvInt` từ TASK-000** (sau khi TASK-000 done):

```go
import "github.com/<module>/shared/pkg/config"

metricsPort := config.EnvInt("METRICS_PORT", 9090)
observability.StartMetricsServer(metricsPort)
```

**Nếu `StartMetricsServer` chưa nhận `int`** mà nhận string hoặc có format khác,
điều chỉnh theo signature thực tế.

### Bước 3: Thêm import `strconv` nếu dùng Option A

```go
import (
    // ...
    "strconv"  // [ADD] cho envOrInt
)
```

## Verification

```bash
# Build tất cả 4 services
go build ./services/gateway-service/cmd/server/
go build ./services/search-service/cmd/server/
go build ./services/data-service/cmd/server/
go build ./services/notification-service/cmd/server/

# Test: port có thể override
METRICS_PORT=19090 go run ./services/gateway-service/cmd/server/ &
curl http://localhost:19090/metrics | head -3
# → Prometheus metrics tại port 19090

# Test: default port vẫn hoạt động
go run ./services/gateway-service/cmd/server/ &
curl http://localhost:9090/metrics | head -3
# → Prometheus metrics tại default port 9090
```

## Acceptance Criteria

- [ ] Cả 4 services đọc `METRICS_PORT` từ env var
- [ ] Mỗi service có default port riêng (9090, 9091, 9092, 9094)
- [ ] Override `METRICS_PORT=19090` hoạt động cho mọi service
- [ ] Tất cả 4 services build thành công
- [ ] Không còn integer literal hardcode trong lời gọi `StartMetricsServer`
