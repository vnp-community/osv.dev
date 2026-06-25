# TASK-012 — Fix BUG-006: Inject Service Version qua ldflags & Xóa Hardcode "1.0.0"

> **Bug**: BUG-006  
> **Priority**: 🟢 Low — Logs và traces luôn hiển thị version 1.0.0; không thể phân biệt deployment  
> **Depends on**: không có dependency  
> **Solution ref**: [SOL-GROUP-C](../solutions/SOL-GROUP-C-ai-service-config.md#bug-006)

## Files Cần Đọc Trước

```
services/gateway-service/cmd/server/main.go
services/gateway-service/internal/health/info_handler.go
services/notification-service/cmd/server/main.go
services/product-service/internal/delivery/http/handlers.go       (line 237)
Makefile (ở root project, nếu có)
```

## Files Sẽ Bị Sửa / Tạo

```
services/gateway-service/cmd/server/main.go                 [MODIFY]
services/gateway-service/internal/health/info_handler.go    [MODIFY]
services/notification-service/cmd/server/main.go            [MODIFY]
services/product-service/internal/delivery/http/handlers.go [MODIFY]
Makefile                                                     [MODIFY or CREATE]
```

## Thay Đổi Chi Tiết

### Bước 1: Đọc tất cả files để tìm chính xác các chỗ hardcode

```bash
grep -rn '"1\.0\.0"' \
    services/gateway-service/ \
    services/notification-service/ \
    services/product-service/
```

### Bước 2: Sửa `gateway-service/cmd/server/main.go`

**Thêm** package-level variables (ngay sau `package main`):

```go
package main

// Version và BuildTime được inject lúc build qua -ldflags.
// Giá trị mặc định "dev" được dùng khi build local không có ldflags.
var (
    Version   = "dev"     // injected: go build -ldflags "-X main.Version=v2.2.1"
    BuildTime = "unknown" // injected: go build -ldflags "-X main.BuildTime=20260622T100000Z"
)
```

**Thêm** helper function `resolveVersion`:
```go
// resolveVersion ưu tiên: ldflags build value → SERVICE_VERSION env → "dev"
func resolveVersion(buildVersion string) string {
    if buildVersion != "" && buildVersion != "dev" {
        return buildVersion
    }
    if v := os.Getenv("SERVICE_VERSION"); v != "" {
        return v
    }
    return "dev"
}
```

**Thay** tất cả `"1.0.0"` bằng `resolveVersion(Version)` trong `main()`:
```go
func main() {
    version := resolveVersion(Version)

    // Tìm và thay:
    // log := observability.InitLogger("gateway-service", "1.0.0")
    // → log := observability.InitLogger("gateway-service", version)

    // shutdown, err := observability.InitTracer(ctx, "gateway-service", "1.0.0")
    // → shutdown, err := observability.InitTracer(ctx, "gateway-service", version)

    // healthUseCase := health.NewAggregateUseCase(upstreams, "1.0.0")
    // → healthUseCase := health.NewAggregateUseCase(upstreams, version)
}
```

### Bước 3: Sửa `gateway-service/internal/health/info_handler.go`

Tìm:
```go
Version: "1.0.0",
```

`InfoHandler` cần nhận version từ ngoài thay vì hardcode:

```go
type InfoHandler struct {
    version   string  // injected từ main.go
    // ... other fields
}

func NewInfoHandler(version string, ...) *InfoHandler {
    return &InfoHandler{version: version, ...}
}

func (h *InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "version": h.version,  // [FIX] không còn hardcode "1.0.0"
        // ...
    })
}
```

Cập nhật call site trong `main.go` để pass version:
```go
infoHandler := health.NewInfoHandler(version, ...)
```

### Bước 4: Sửa `notification-service/cmd/server/main.go`

Tương tự gateway-service:
```go
var (
    Version   = "dev"
    BuildTime = "unknown"
)

func main() {
    version := resolveVersion(Version)
    log := observability.InitLogger("notification-service", version)
    // ...
}
```

### Bước 5: Sửa `product-service/internal/delivery/http/handlers.go`

Tìm (khoảng line 237):
```go
if req.Version == "" { req.Version = "1.0.0" }
```

**Xóa** hardcode này. Engagement version là optional:
```go
// [FIX] Version là optional field — không set default "1.0.0"
// Nếu là CI/CD engagement, caller nên set version từ build context
// Nếu là interactive engagement, version có thể để trống
```

Hoặc nếu cần default có nghĩa hơn:
```go
if req.Version == "" && req.BuildID != "" {
    req.Version = req.BuildID  // dùng build ID làm version cho CI/CD engagements
}
```

### Bước 6: Tạo / cập nhật Makefile ở root

```bash
# Kiểm tra Makefile có ở root không
ls Makefile 2>/dev/null || ls services/Makefile 2>/dev/null
```

Thêm hoặc cập nhật với LDFLAGS:

```makefile
# Version injection via git
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u +%Y%m%dT%H%M%SZ)
LDFLAGS     := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: build-gateway build-notification build-product

build-gateway:
	go build $(LDFLAGS) -o bin/gateway-service ./services/gateway-service/cmd/server/

build-notification:
	go build $(LDFLAGS) -o bin/notification-service ./services/notification-service/cmd/server/

build-product:
	go build $(LDFLAGS) -o bin/product-service ./services/product-service/cmd/server/
```

Nếu đã có Makefile, thêm LDFLAGS vào build targets hiện có.

## Verification

```bash
# Build với version injection
make build-gateway 2>/dev/null || \
    go build -ldflags "-X main.Version=v2.2.1-test" \
    -o /tmp/gateway-service ./services/gateway-service/cmd/server/

# Test: health/info endpoint trả về version đúng
/tmp/gateway-service &
curl http://localhost:8080/api/v1/admin/health 2>/dev/null | jq .version
# → "v2.2.1-test" (không phải "1.0.0")

# Test: SERVICE_VERSION env override
SERVICE_VERSION=v2.2.1-env go run ./services/gateway-service/cmd/server/ &
curl http://localhost:8080/api/v1/admin/health | jq .version
# → "v2.2.1-env"

# Kiểm tra không còn "1.0.0" hardcode
grep -rn '"1\.0\.0"' services/gateway-service/ services/notification-service/ services/product-service/
# → phải rỗng
```

## Acceptance Criteria

- [ ] `var Version = "dev"` khai báo trong `gateway-service/main.go` và `notification-service/main.go`
- [ ] `resolveVersion()` function có mặt (hoặc inline logic tương đương)
- [ ] `InfoHandler` nhận version qua constructor, không hardcode
- [ ] `"1.0.0"` không còn xuất hiện trong bất kỳ files nào đã liệt kê
- [ ] Makefile có LDFLAGS với `-X main.Version=$(VERSION)`
- [ ] `go build ./services/gateway-service/...` thành công
- [ ] `go build ./services/notification-service/...` thành công
- [ ] `go build ./services/product-service/...` thành công
