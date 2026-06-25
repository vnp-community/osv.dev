# TASK-INIT-003 — data-service: Fix envOr bug + Tạo init.sh + DATA_ prefix env vars

> **Solution**: [SOL-INIT-003](../solutions/SOL-INIT-003-data-bootstrap.md)  
> **Files thực tế**: `storage_config.go` (44 dòng — BUG nghiêm trọng), `main.go` (110 dòng)

---

## Tổng quan

Task này bao gồm 3 thay đổi:
1. **BUG FIX** `internal/config/storage_config.go` — `envOr()` là placeholder trả về `defaultVal` hardcoded, không bao giờ đọc OS env
2. **Sửa** `cmd/server/main.go` — đọc `DATA_GRPC_PORT`, `DATA_HTTP_PORT` và cải thiện `/health` response
3. **Tạo** `scripts/init.sh` — bootstrap script (NEW file)

---

## Bước 1 — BUG FIX `internal/config/storage_config.go` (CRITICAL)

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/config/storage_config.go`

**Bug thực tế** (dòng 38-43):
```go
func envOr(key, defaultVal string) string {
	// Import "os" in the calling package to avoid adding an import here.
	// This function is a placeholder — replace with your config loader.
	// Example: return os.Getenv(key) if empty { return defaultVal }
	return defaultVal // replaced in main.go with actual env read  ← BUG: luôn trả về default!
}
```

**Toàn bộ file sau khi fix** (thay thế toàn bộ nội dung file):

```go
// Package config provides storage backend selection configuration.
// Controls which storage implementation is used for each repository.
// Pattern: env-driven selector — existing Firestore implementation is default.
package config

import "os"

// AliasGroupBackend selects the persistence backend for AliasGroupRepository.
type AliasGroupBackend string

const (
	// AliasGroupBackendFirestore uses Google Cloud Firestore (legacy default).
	AliasGroupBackendFirestore AliasGroupBackend = "firestore"

	// AliasGroupBackendPostgres uses PostgreSQL (recommended for local dev).
	// Requires migration 005_create_alias_groups.up.sql to be applied first.
	AliasGroupBackendPostgres AliasGroupBackend = "postgres"
)

// StorageConfig holds backend selection for data-service storage domains.
type StorageConfig struct {
	// AliasGroupBackend selects where alias groups are persisted.
	// Env: ALIAS_GROUP_BACKEND
	// Default: "postgres" (changed from "firestore" to avoid GCP dependency in dev)
	AliasGroupBackend AliasGroupBackend
}

// LoadStorageConfig loads StorageConfig from environment variables.
func LoadStorageConfig() StorageConfig {
	backend := AliasGroupBackend(envOr("ALIAS_GROUP_BACKEND", string(AliasGroupBackendPostgres)))
	return StorageConfig{
		AliasGroupBackend: backend,
	}
}

// envOr returns the value of the OS environment variable named key,
// or defaultVal if the variable is not set or empty.
// FIXED: previous implementation was a placeholder that always returned defaultVal.
func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
```

**Thay đổi quan trọng**:
- Thêm `import "os"` 
- Sửa `envOr()` để thực sự gọi `os.Getenv(key)`
- Đổi default từ `"firestore"` → `"postgres"` (để local dev không cần GCP)

---

## Bước 2 — Sửa `cmd/server/main.go`

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/cmd/server/main.go`

### Thay đổi 2.1 — Dòng 45-46: thêm DATA_ prefix ports

```diff
-	grpcPort := envOr("GRPC_PORT", "50053")
-	httpPort := envOr("HTTP_PORT", "8080")
+	grpcPort := envOr("DATA_GRPC_PORT", envOr("GRPC_PORT", "50053"))
+	httpPort := envOr("DATA_HTTP_PORT", envOr("HTTP_PORT", "8082"))
```

### Thay đổi 2.2 — Dòng 77-80: cải thiện `/health` response

```diff
 	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
-		w.WriteHeader(http.StatusOK)
-		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
+		w.Header().Set("Content-Type", "application/json")
+		w.WriteHeader(http.StatusOK)
+		fmt.Fprintf(w, `{"status":"ok","service":"data-service","grpc_port":"%s","http_port":"%s","alias_backend":"%s"}`,
+			grpcPort, httpPort, envOr("ALIAS_GROUP_BACKEND", "postgres")) //nolint:errcheck
 	})
```

### Thay đổi 2.3 — Thêm `"fmt"` vào imports (dòng 16-28)

```diff
 import (
 	"context"
+	"fmt"
 	"net"
 	"net/http"
 	"os"
 	"os/signal"
 	"syscall"
 	...
 )
```

---

## Bước 3 — Tạo `scripts/init.sh`

**Action**: Tạo file mới

**File**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/scripts/init.sh`

Nội dung đầy đủ từ SOL-INIT-003. Script thực hiện 3 bước:
- Tạo PostgreSQL extensions (vector, uuid-ossp, citext) và schema `vuln`
- Apply migrations theo thứ tự (bỏ qua `.down.sql`)
- Validate storage backend config

**Sau khi tạo**: `chmod +x services/data-service/scripts/init.sh`

---

## Acceptance Criteria

- [ ] `envOr()` trong `storage_config.go` gọi `os.Getenv()` thực sự
- [ ] `LoadStorageConfig()` trả về `postgres` khi `ALIAS_GROUP_BACKEND` env không set
- [ ] `LoadStorageConfig()` trả về `firestore` khi `ALIAS_GROUP_BACKEND=firestore`
- [ ] `DATA_GRPC_PORT` và `DATA_HTTP_PORT` được đọc từ env
- [ ] `GET /health` trả về JSON với `alias_backend` field
- [ ] `scripts/init.sh` tồn tại và executable
- [ ] `go build ./cmd/server` không lỗi

---

## Kiểm tra sau khi thực thi

```bash
# Build
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service
go build ./cmd/server/...

# Test envOr fix
ALIAS_GROUP_BACKEND=postgres go run ./cmd/server &
curl http://localhost:8082/health
# Expected: {"status":"ok","service":"data-service","alias_backend":"postgres"}
```
