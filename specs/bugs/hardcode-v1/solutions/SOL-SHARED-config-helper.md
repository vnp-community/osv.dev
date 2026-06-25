# SOL-SHARED — Shared Config Helper Package

> **Fixes**: Infrastructure dùng chung cho BUG-001, 003, 004, 005, 006, 007, 012  
> **Priority**: Phải tạo trước khi implement các solutions khác  
> **Path**: `shared/pkg/config/`

---

## Vấn Đề Cần Giải Quyết

Mỗi service hiện tại tự implement env-reading với pattern:
```go
addr := os.Getenv("SOME_SERVICE_ADDR")
if addr == "" {
    addr = "localhost:PORT"  // silent fallback, no log
}
```

Pattern này lặp lại ~20 lần khắp codebase, không có warning log, không nhất quán.

---

## Giải Pháp: `shared/pkg/config` Package

### File: `shared/pkg/config/env.go`

```go
// Package config cung cấp helper functions để load configuration từ environment variables.
// Mọi fallback về default value đều phải được log với level WARN để dễ debug production.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// Str đọc env var, fallback về defaultVal nếu không set.
// Luôn log WARN khi dùng fallback để dễ phát hiện misconfiguration.
func Str(envKey, defaultVal string) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	if defaultVal != "" {
		log.Warn().
			Str("env_key", envKey).
			Str("default", defaultVal).
			Msg("env var not set, using default — configure in production")
	}
	return defaultVal
}

// StrRequired đọc env var bắt buộc — panic nếu không được set.
// Dùng cho credentials và security-critical config.
func StrRequired(envKey string) string {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", envKey))
	}
	return v
}

// Int đọc env var dạng integer với fallback.
func Int(envKey string, defaultVal int) int {
	v := os.Getenv(envKey)
	if v == "" {
		if defaultVal != 0 {
			log.Warn().
				Str("env_key", envKey).
				Int("default", defaultVal).
				Msg("env var not set, using default")
		}
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Warn().
			Str("env_key", envKey).
			Str("value", v).
			Int("fallback", defaultVal).
			Msg("invalid int value for env var, using default")
		return defaultVal
	}
	return n
}

// Duration đọc env var dạng duration string (e.g. "5m", "30s") với fallback.
func Duration(envKey string, defaultVal time.Duration) time.Duration {
	v := os.Getenv(envKey)
	if v == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Warn().
			Str("env_key", envKey).
			Str("value", v).
			Dur("fallback", defaultVal).
			Msg("invalid duration value for env var, using default")
		return defaultVal
	}
	return d
}

// ServiceAddr đọc địa chỉ service từ env var với fallback về localhost.
// Log WARN khi dùng fallback để engineer biết cần configure trong production.
//
// Ví dụ:
//
//	addr := config.ServiceAddr("FINDING_SERVICE_GRPC", "localhost", 50060)
//	// → "localhost:50060" + WARN log nếu env var không set
func ServiceAddr(envKey, defaultHost string, defaultPort int) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	fallback := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
	log.Warn().
		Str("env_key", envKey).
		Str("fallback", fallback).
		Msg("service address env var not set, using localhost — configure in production")
	return fallback
}

// HTTPServiceAddr tương tự ServiceAddr nhưng thêm http:// prefix.
func HTTPServiceAddr(envKey, defaultHost string, defaultPort int) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	fallback := fmt.Sprintf("http://%s:%d", defaultHost, defaultPort)
	log.Warn().
		Str("env_key", envKey).
		Str("fallback", fallback).
		Msg("HTTP service addr not set, using localhost — configure in production")
	return fallback
}

// Coalesce trả về giá trị đầu tiên không rỗng trong danh sách.
// Thứ tự ưu tiên: env var → explicit override → hardcode default.
func Coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

### File: `shared/pkg/config/version.go`

```go
package config

import "os"

// ServiceVersion đọc version từ build-time ldflags hoặc env var.
// Dùng trong mọi service để inject version vào logger, tracer, health endpoint.
//
// Build-time injection (recommended):
//
//	var Version = "dev"  // overridden by: go build -ldflags "-X main.Version=$(git describe --tags)"
//
// Runtime fallback:
//
//	version := config.ServiceVersion(Version)
func ServiceVersion(buildTimeVersion string) string {
	if buildTimeVersion != "" && buildTimeVersion != "dev" {
		return buildTimeVersion
	}
	if v := os.Getenv("SERVICE_VERSION"); v != "" {
		return v
	}
	return "dev"
}
```

### File: `shared/pkg/config/infra.go`

```go
package config

// InfraConfig chứa các địa chỉ infrastructure dùng chung.
// Được load một lần trong main() và passed xuống các components.
type InfraConfig struct {
	PostgresDSN string
	RedisURL    string
	NATSUrl     string
}

// LoadInfraConfig load config từ env vars.
// PostgresDSN là required — panic nếu thiếu để fail fast thay vì connect localhost.
func LoadInfraConfig() InfraConfig {
	// PostgresDSN: không có default để tránh chứa credentials trong source code
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		// Thử build từ các env vars riêng lẻ
		host := Str("POSTGRES_HOST", "localhost")
		port := Int("POSTGRES_PORT", 5432)
		db   := Str("POSTGRES_DB", "osvdb")
		user := os.Getenv("POSTGRES_USER")
		pass := os.Getenv("POSTGRES_PASSWORD")
		if user != "" && pass != "" {
			postgresDSN = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
				user, pass, host, port, db)
		} else {
			log.Fatal().
				Msg("POSTGRES_DSN not set and POSTGRES_USER/POSTGRES_PASSWORD not provided — " +
					"cannot connect to database without credentials")
		}
	}

	return InfraConfig{
		PostgresDSN: postgresDSN,
		RedisURL:    Str("REDIS_URL", "redis://localhost:6379/0"),
		NATSUrl:     Str("NATS_URL", "nats://localhost:4222"),
	}
}
```

---

## Makefile — Build-Time Version Injection

Thêm vào `Makefile` ở root:

```makefile
# Version từ git tags
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y%m%dT%H%M%SZ)
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Build từng service
.PHONY: build-gateway build-ai build-scan build-finding build-data

build-gateway:
	go build $(LDFLAGS) -o bin/gateway-service ./services/gateway-service/cmd/server/

build-ai:
	go build $(LDFLAGS) -o bin/ai-service ./services/ai-service/cmd/server/

build-scan:
	go build $(LDFLAGS) -o bin/scan-service ./services/scan-service/cmd/server/

build-finding:
	go build $(LDFLAGS) -o bin/finding-service ./services/finding-service/cmd/server/

build-data:
	go build $(LDFLAGS) -o bin/data-service ./services/data-service/cmd/server/

build-all: build-gateway build-ai build-scan build-finding build-data
```

Trong mỗi `cmd/server/main.go`:

```go
// Package variable — injected by ldflags at build time
var (
    Version   = "dev"
    BuildTime = "unknown"
)

func main() {
    version := config.ServiceVersion(Version)
    log := observability.InitLogger("gateway-service", version)
    // ...
}
```

---

## Environment Variables Convention

| Pattern | Ví dụ | Mô tả |
|---------|-------|-------|
| `<SVC>_SERVICE_HTTP` | `SEARCH_SERVICE_HTTP` | HTTP address của service |
| `<SVC>_SERVICE_GRPC` | `FINDING_SERVICE_GRPC` | gRPC address của service |
| `<TOOL>_BASE_URL` | `ZAP_BASE_URL`, `OLLAMA_BASE_URL` | External tool URL |
| `<TOOL>_<PARAM>` | `ZAP_SPIDER_TIMEOUT` | Tool-specific parameter |
| `METRICS_PORT` | `METRICS_PORT=9090` | Prometheus metrics port |
| `SERVICE_VERSION` | `SERVICE_VERSION=v2.2.1` | Runtime version override |

---

## Checklist Khi Dùng Package Này

- [ ] Import `shared/pkg/config` vào service
- [ ] Thay tất cả `os.Getenv` + if-else thành `config.Str()` / `config.ServiceAddr()`
- [ ] Thay credentials default bằng `config.StrRequired()` hoặc `config.LoadInfraConfig()`
- [ ] Thêm `var Version = "dev"` vào `main.go` của mỗi service
- [ ] Cập nhật Makefile để inject version qua ldflags
