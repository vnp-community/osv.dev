> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T05 — Scan Service Wiring (Core HTTP Handler)

## Thông tin
| | |
|---|---|
| **Phase** | 2 — Scan Core |
| **Ước tính** | 4–6 giờ |
| **Depends on** | T04 |
| **Blocks** | T06, T07, T08 |

## Mục tiêu
Wire-up `scan-service` HTTP handler nguyên si vào monolith. Scan handler đã có đầy đủ CRUD. Chỉ cần khởi tạo dependencies và mount router.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `scan-service/internal/adapters/handler/http` | `ScanHandler`, `NewRouter()` |
| `scan-service/internal/usecase/create_scan` | `CreateScanUseCase` |
| `scan-service/internal/usecase/execute_scan` | `ExecuteScanUseCase` |
| `scan-service/internal/adapters/repository/postgres` | Scan, Finding, Asset repos |
| `scan-service/internal/adapters/repository/redis` | Redis cache |
| `scan-service/internal/adapters/worker` | `WorkerPool` |
| `scan-service/internal/domain/scan/entity` | Entities |

---

## Các bước thực hiện

### 5.1 Đọc constructor signatures của scan-service

```bash
# Xác minh API
cat osv.dev/services/scan-service/internal/usecase/create_scan/create_scan.go
cat osv.dev/services/scan-service/internal/usecase/execute_scan/execute_scan.go
cat osv.dev/services/scan-service/internal/adapters/repository/postgres/*.go
cat osv.dev/services/scan-service/internal/adapters/handler/http/scan_handler.go
```

Ghi lại:
- `New(...)` params cho CreateScanUseCase
- `New(...)` params cho ExecuteScanUseCase
- `NewScanRepo(db)`, `NewFindingRepo(db)`, `NewAssetRepo(db)` signatures
- `NewScanHandler(...)` params (đã thấy trong spec: createUC, executeUC, scanRepo, findingRepo, pool, log)

### 5.2 Khởi tạo repositories trong app.go

```go
import (
    scanrepo "github.com/osv/scan-service/internal/adapters/repository/postgres"
    scanredis "github.com/osv/scan-service/internal/adapters/repository/redis"
)

// Trong app.New():
scanRepo    := scanrepo.NewScanRepo(a.db)
findingRepo := scanrepo.NewFindingRepo(a.db)
assetRepo   := scanrepo.NewAssetRepo(a.db)
scanCache   := scanredis.NewScanCache(redisClient)  // nếu có
```

### 5.3 Khởi tạo usecases

```go
import (
    scancreate  "github.com/osv/scan-service/internal/usecase/create_scan"
    scanexecute "github.com/osv/scan-service/internal/usecase/execute_scan"
)

// CreateScan usecase (lưu scan vào DB + enqueue)
createScanUC := scancreate.New(scanRepo, a.nc, a.log)

// ExecuteScan usecase (chạy nmap/zap)
executeScanUC := scanexecute.New(scanRepo, findingRepo, assetRepo, a.log)
// Có thể cần thêm: nmap binary path, timeout, etc. từ config
```

### 5.4 Khởi tạo WorkerPool

```go
import (
    scanworker "github.com/osv/scan-service/internal/adapters/worker"
)

pool := scanworker.NewWorkerPool(
    cfg.Scan.WorkerPoolSize,
    func(ctx context.Context, job scanworker.ScanJob) error {
        return executeScanUC.Execute(ctx, job.ScanID, job.UserID)
    },
    a.log,
)
```

> **Lưu ý**: Xem `execute_scan.go` để biết chính xác signature của `Execute()` method.

### 5.5 Khởi tạo ScanHandler

```go
import (
    scanhttp "github.com/osv/scan-service/internal/adapters/handler/http"
)

scanHandler := scanhttp.NewScanHandler(
    createScanUC,
    executeScanUC,
    scanRepo,
    findingRepo,
    pool,
    a.log,
)
```

### 5.6 Mount scan router

Trong `internal/router/router.go`:

```go
import (
    scanhttp "github.com/osv/scan-service/internal/adapters/handler/http"
)

// Bên trong protected group (sau JWT middleware)
r.Group(func(r chi.Router) {
    r.Use(authMW.RequireAuth)

    // Mount scan handler nguyên si
    // scanhttp.NewRouter() đã định nghĩa tất cả routes:
    // POST /scans, GET /scans, GET /scans/{id}, DELETE /scans/{id}
    // GET /scans/{id}/findings
    r.Mount("/", scanhttp.NewRouter(a.ScanHandler, a.log))
})
```

> **Quan trọng**: Kiểm tra `scanhttp.NewRouter()` mount tại prefix nào.  
> Nếu nó tự mount tại `/api/v1`, dùng `r.Mount("/", ...)`.  
> Nếu mount tại `/`, dùng `r.Mount("/api/v1", ...)`.

### 5.7 Cập nhật App struct

```go
// internal/app/app.go
type App struct {
    // Infrastructure
    cfg *Config
    db  *shareddb.DB
    nc  *sharednats.Client
    log zerolog.Logger

    // Repositories (shared giữa các services)
    ScanRepo    ScanRepository    // interface
    FindingRepo FindingRepository // interface
    AssetRepo   AssetRepository   // interface

    // Scan service
    ScanHandler    *scanhttp.ScanHandler
    WorkerPool     *scanworker.WorkerPool
    CreateScanUC   *scancreate.UseCase
    ExecuteScanUC  *scanexecute.UseCase
}
```

### 5.8 Thêm nmap config

```go
// Trong execute_scan usecase init (nếu cần):
executeScanUC := scanexecute.New(
    scanRepo, findingRepo, assetRepo,
    scanexecute.Config{
        NmapBinary: cfg.Scan.NmapBinary,
        Timeout:    time.Duration(cfg.Scan.DefaultTimeout) * time.Second,
    },
    a.log,
)
```

---

## Output

- [x] Scan repositories khởi tạo đúng (Bridge Pattern — pgxpool direct) ✓
- [x] CreateScan, ExecuteScan usecases khởi tạo đúng (scan_bridge.go) ✓
- [x] WorkerPool khởi tạo với pool size từ config (cfg.Scan.WorkerPool) ✓
- [x] ScanHandler mount vào router (/api/v1/scans/*) ✓
- [x] App struct cập nhật: ScanRunner với HTTPHandler ✓

## Acceptance Criteria

```bash
TOKEN=<login_token>

# Tạo scan
curl -X POST http://localhost:8080/api/v1/scans \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"targets":["127.0.0.1"],"scan_type":"discovery"}'
# → {"scan_id":"<uuid>","status":"pending","message":"scan queued"}

# Lấy scan detail
SCAN_ID=<uuid>
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/scans/$SCAN_ID
# → {"id":"...","status":"running"|"completed","targets":["127.0.0.1"]}

# List scans
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/scans
# → {"scans":[...],"total":1}

# Cancel scan
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/scans/$SCAN_ID
# → {"message":"scan cancelled"}
```

## Lưu ý

- `nmap` phải được cài trên host (`brew install nmap` trên macOS, `apt install nmap` trên Linux)
- Scan discovery (`nmap -sn`) không cần root, full scan cần quyền cao hơn
- Kiểm tra scan-service có cần NATS để publish `scan.created` event không
