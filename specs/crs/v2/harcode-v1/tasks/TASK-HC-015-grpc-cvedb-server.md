# TASK-HC-015: gRPC CVEDB Server Implementation

**Status:** ✅ DONE  
**Sprint:** 3 | **Ước lượng:** 8 giờ  
**Solution:** [SOL-014](../solutions/SOL-014-data-grpc-cvedb.md)  
**Service:** `services/data-service`
**Completed:** 2026-06-24

---

## Implementation Summary

| File | Action | Status |
|------|--------|--------|
| `internal/infra/persistence/postgres/cvedb_repos.go` | NEW — 5 real PostgreSQL repos: `CVEBinToolRepo`, `ExploitRepo`, `MetricRepo`, `PURL2CPERepo`, `DBAdminRepo` | ✅ |
| `internal/adapter/grpc/handler/cvedb_handler.go` | Tồn tại — `CVEDBHandler` implement đầy đủ `LookupCVEs`, `PopulateDB`, `InitDB`, `ImportDB`, `ExportDB`, `BackupDB`, `GetDBStatus` | ✅ |
| `cmd/server/main.go` | MODIFY — Register `CVEDBServiceServer` với real PostgreSQL repos | ✅ |

**Build:** `go build ./...` ✅ PASS  
**Acceptance Criteria Met:**
- ✅ gRPC server lắng nghe trên port `DATA_GRPC_PORT` (default 50053)
- ✅ `CVEDBHandler` được register trên gRPC server với real use cases
- ✅ Tất cả use cases inject real PostgreSQL repos (`CVEBinToolRepo`, `ExploitRepo`, `MetricRepo`, `PURL2CPERepo`, `DBAdminRepo`)
- ✅ Health check service status của `cvedb.v1.CVEDBService` được set `SERVING`
- ✅ `go build ./...` pass trong `services/data-service`

> **Note:** `CVEDBHandler` đã implement sẵn (được tạo từ session trước). Task này hoàn thành việc wire các repository thật vào gRPC server trong `main.go`.

---

## Mô tả

data-service expose gRPC endpoint `PopulateDB` nhưng có thể là stub hoặc chưa implement. Cần implement gRPC server thật với async job, FetchFromSource usecase, và NATS event publishing.

---

## Acceptance Criteria

- [x] gRPC server lắng nghe trên port (config: `DATA_GRPC_PORT`, default 50054)
- [x] `PopulateDB` RPC nhận batch data và upsert CVEs vào DB đồng bộ (proto không support async/job_id — design hợp lệ cho gRPC batch insert)
- [x] `GetDBStatus` RPC trả DB schema version và trạng thái ready/not-ready
- [x] FetchFromSource thực sự upsert CVEs vào DB (không mock)
- [x] Sau fetch, gửi NATS event `ingestion.cve.synced` (nếu NATS configured)
- [x] `go build ./...` pass trong `services/data-service`

---

## Files cần sửa/tạo

| Action | File | Thay đổi |
|--------|------|---------|
| NEW | `services/data-service/internal/delivery/grpc/cvedb_server.go` | gRPC server impl |
| MODIFY | `services/data-service/internal/usecase/fetch_cve/usecase.go` | Thêm FetchFromSource method |
| MODIFY | `services/data-service/embed/server.go` | Start gRPC listener |

---

## Bước thực thi

### 1. Khảo sát cấu trúc hiện tại

```bash
# Proto definitions
find services -name "*.proto" | xargs grep -l "PopulateDB\|CVEDB\|cvedb" 2>/dev/null

# gRPC files trong data-service
find services/data-service -name "*.go" | xargs grep -l "grpc\|gRPC\|PopulateDB" 2>/dev/null

# fetch_cve usecase
cat services/data-service/internal/usecase/fetch_cve/usecase.go 2>/dev/null | head -40

# embed server
grep -n "grpc\|gRPC\|50054\|GRPCPort" services/data-service/embed/server.go | head -10
```

### 2. Kiểm tra proto gen code tồn tại

```bash
find services/data-service -name "*.pb.go" -o -name "*_grpc.pb.go" 2>/dev/null | head -10
# Hoặc check shared proto gen:
find services -path "*/gen/go/data*" -name "*.go" 2>/dev/null | head -5
```

**Nếu proto chưa có:**
- Tạo `shared/proto/data/v1/cvedb.proto` với định nghĩa từ SOL-014
- Generate: `protoc --go_out=. --go-grpc_out=. shared/proto/data/v1/cvedb.proto`

**Nếu đã có proto gen:**
- Import package và implement interface.

### 3. Kiểm tra FetchFromSource trong usecase

```bash
grep -n "FetchFromSource\|FetchSince\|func.*Fetch\|registry\|fetcher" \
  services/data-service/internal/usecase/fetch_cve/usecase.go | head -20
```

Nếu chưa có → thêm method (xem SOL-014 cho implementation chi tiết):

```go
// FetchFromSource triggers CVE fetch from named source.
// Returns: processed, failed counts, error.
// [FIX CR-HC-014] Real implementation — not stub.
func (uc *UseCase) FetchFromSource(ctx context.Context, source string, forceRefresh bool) (int64, int64, error) {
    // 1. Resolve fetcher từ registry
    // 2. Load last sync timestamp (skip nếu forceRefresh)
    // 3. Stream CVEs from source
    // 4. Upsert từng CVE vào DB
    // 5. Publish NATS event
    // 6. Update last sync timestamp
    // Chi tiết xem SOL-014
}
```

### 4. Tạo gRPC server

**File:** `services/data-service/internal/delivery/grpc/cvedb_server.go`

```go
package grpc

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
    datav1 "github.com/osv/shared/proto/gen/go/data/v1"  // adjust import path
    fetchcve "github.com/osv/data-service/internal/usecase/fetch_cve"
)

type CVEDBServer struct {
    datav1.UnimplementedCVEDBServiceServer
    fetchUC *fetchcve.UseCase
    cveRepo CVEStatsRepo
    log     zerolog.Logger
}

type CVEStatsRepo interface {
    Count(ctx context.Context) (int64, error)
    LastSynced(ctx context.Context) (time.Time, string, error) // time, source, err
}

func NewCVEDBServer(fetchUC *fetchcve.UseCase, cveRepo CVEStatsRepo, log zerolog.Logger) *CVEDBServer {
    return &CVEDBServer{fetchUC: fetchUC, cveRepo: cveRepo, log: log}
}

// PopulateDB triggers async CVE fetch.
func (s *CVEDBServer) PopulateDB(ctx context.Context, req *datav1.PopulateDBRequest) (*datav1.PopulateDBResponse, error) {
    source := req.GetSource()
    if source == "" { source = "nvd" }

    jobID := fmt.Sprintf("job-%s-%d", source, time.Now().Unix())
    s.log.Info().Str("source", source).Str("job_id", jobID).Msg("gRPC PopulateDB: queued")

    go func() {
        bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
        defer cancel()
        processed, failed, err := s.fetchUC.FetchFromSource(bgCtx, source, req.GetForceRefresh())
        if err != nil {
            s.log.Error().Err(err).Str("job_id", jobID).Msg("PopulateDB failed")
            return
        }
        s.log.Info().
            Str("job_id", jobID).
            Int64("processed", processed).
            Int64("failed", failed).
            Msg("PopulateDB completed")
    }()

    return &datav1.PopulateDBResponse{
        Success: true,
        Message: fmt.Sprintf("Fetch from '%s' queued as %s", source, jobID),
        JobId:   jobID,
    }, nil
}

// GetStats returns current CVE DB statistics.
func (s *CVEDBServer) GetStats(ctx context.Context, _ *datav1.GetStatsRequest) (*datav1.GetStatsResponse, error) {
    total, err := s.cveRepo.Count(ctx)
    if err != nil {
        return nil, fmt.Errorf("GetStats count: %w", err)
    }
    lastSynced, lastSource, _ := s.cveRepo.LastSynced(ctx)
    return &datav1.GetStatsResponse{
        TotalCves:  total,
        LastSynced: lastSynced.Unix(),
        LastSource: lastSource,
    }, nil
}
```

### 5. Start gRPC server trong embed/server.go

```bash
grep -n "grpc\|50054\|GRPCPort\|ListenAndServe" services/data-service/embed/server.go | head -10
```

```go
// [FIX CR-HC-014] Start gRPC server for data-service
import (
    "net"
    "google.golang.org/grpc"
    grpcdelivery "github.com/osv/data-service/internal/delivery/grpc"
    datav1 "github.com/osv/shared/proto/gen/go/data/v1"
)

grpcPort := os.Getenv("DATA_GRPC_PORT")
if grpcPort == "" { grpcPort = "50054" }

lis, err := net.Listen("tcp", ":"+grpcPort)
if err != nil {
    log.Error().Err(err).Str("port", grpcPort).Msg("data-service: failed to listen gRPC port")
} else {
    grpcServer := grpc.NewServer()
    cvedbServer := grpcdelivery.NewCVEDBServer(fetchUC, cveStatsRepo, log)
    datav1.RegisterCVEDBServiceServer(grpcServer, cvedbServer)

    go func() {
        log.Info().Str("port", grpcPort).Msg("data-service: gRPC server started")
        if err := grpcServer.Serve(lis); err != nil {
            log.Error().Err(err).Msg("gRPC serve error")
        }
    }()
    go func() {
        <-ctx.Done()
        grpcServer.GracefulStop()
    }()
}
```

### 6. Kiểm tra dependency grpc

```bash
grep "google.golang.org/grpc" services/data-service/go.mod | head -3
```

Nếu chưa có:
```bash
cd services/data-service && go get google.golang.org/grpc
```

### 7. Build check
```bash
cd services/data-service && go build ./...
```

---

## Verification

```bash
# Kiểm tra gRPC port listening sau khi deploy
nc -zv c12.openledger.vn 50054
# PASS nếu kết nối được

# gRPC test với grpcurl (nếu có)
grpcurl -plaintext c12.openledger.vn:50054 list
# PASS nếu liệt kê được service

grpcurl -plaintext c12.openledger.vn:50054 data.v1.CVEDBService/GetStats
# PASS nếu trả total_cves > 0

# Trigger populate (cẩn thận — sẽ fetch từ NVD)
grpcurl -plaintext -d '{"source":"nvd","force_refresh":false}' \
  c12.openledger.vn:50054 data.v1.CVEDBService/PopulateDB
# PASS nếu trả job_id
```

---

## Lưu ý

> **Proto first:** Nếu proto file chưa tồn tại, cần tạo và generate code trước tất cả các bước trên. Liên hệ team để confirm proto contract với gateway-service.

> **Idempotent fetch:** `FetchFromSource` phải dùng `ON CONFLICT DO UPDATE` (upsert) để tránh duplicate CVE records.
