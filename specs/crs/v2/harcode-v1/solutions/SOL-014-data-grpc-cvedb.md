# SOL-014: gRPC CVEDB PopulateDB — data-service

**CR:** CR-HC-014 | **Priority:** 🟡 Medium | **Sprint:** 3  
**Service:** `services/data-service`

---

## Implementation Status

**✅ IMPLEMENTED** — 2026-06-24
**Task:** TASK-HC-015
**Note:** CVEDBHandler register trên gRPC server với real PostgreSQL repositories
**Build:** ✅ `go build ./...` passes

---

---

## Context phân tích code

Cần kiểm tra file gRPC server trong data-service:

```bash
find services/data-service -name "*.go" | xargs grep -l "PopulateDB\|grpc\|gRPC" 2>/dev/null | head -10
```

**Kịch bản dự kiến:** data-service expose gRPC endpoint `PopulateDB` cho gateway-service, nhưng implementation trả về hardcoded response hoặc empty.

---

## Solution

### Bước 1: Kiểm tra current state

```bash
# Tìm gRPC server trong data-service
grep -rn "PopulateDB\|RegisterCVEDB\|grpc.Server" services/data-service/ --include="*.go"

# Kiểm tra proto definition
find services -name "*.proto" | xargs grep -l "PopulateDB\|cvedb" 2>/dev/null
```

### Bước 2: Proto contract (nếu chưa có)

**File mới hoặc sửa:** `shared/proto/data/v1/cvedb.proto`

```protobuf
syntax = "proto3";
package data.v1;

option go_package = "github.com/osv/shared/proto/gen/go/data/v1";

service CVEDBService {
    // PopulateDB triggers or queries CVE database population status
    rpc PopulateDB(PopulateDBRequest) returns (PopulateDBResponse);
    rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
}

message PopulateDBRequest {
    string source = 1;      // "nvd", "circl", "jvn", "all"
    bool force_refresh = 2; // bypass last-synced check
    string since = 3;       // ISO date — fetch since this date
}

message PopulateDBResponse {
    bool success    = 1;
    int64 processed = 2;
    int64 failed    = 3;
    string message  = 4;
    string job_id   = 5;
}

message GetStatsRequest {}

message GetStatsResponse {
    int64 total_cves    = 1;
    int64 last_synced   = 2; // unix timestamp
    string last_source  = 3;
}
```

### Bước 3: gRPC Server Implementation

**File mới:** `data-service/internal/delivery/grpc/cvedb_server.go`

```go
package grpc

import (
    "context"
    "fmt"

    "github.com/rs/zerolog"
    datav1 "github.com/osv/shared/proto/gen/go/data/v1"
    "github.com/osv/data-service/internal/usecase/fetch_cve"
)

type CVEDBServer struct {
    datav1.UnimplementedCVEDBServiceServer
    fetchUC *fetch_cve.UseCase
    statsUC StatsUseCase
    log     zerolog.Logger
}

type StatsUseCase interface {
    GetDBStats(ctx context.Context) (*fetch_cve.DBStats, error)
}

func NewCVEDBServer(fetchUC *fetch_cve.UseCase, statsUC StatsUseCase, log zerolog.Logger) *CVEDBServer {
    return &CVEDBServer{fetchUC: fetchUC, statsUC: statsUC, log: log}
}

// PopulateDB triggers CVE fetch from a given source.
// [FIX CR-HC-014] Real implementation — không stub
func (s *CVEDBServer) PopulateDB(ctx context.Context, req *datav1.PopulateDBRequest) (*datav1.PopulateDBResponse, error) {
    source := req.GetSource()
    if source == "" {
        source = "nvd"
    }

    s.log.Info().
        Str("source", source).
        Bool("force_refresh", req.ForceRefresh).
        Msg("gRPC PopulateDB: starting fetch")

    // Run async — gRPC responds immediately
    jobID := fmt.Sprintf("job-%s-%d", source, time.Now().Unix())

    go func() {
        bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
        defer cancel()

        processed, failed, err := s.fetchUC.FetchFromSource(bgCtx, source, req.ForceRefresh)
        if err != nil {
            s.log.Error().Err(err).Str("source", source).Msg("PopulateDB job failed")
            return
        }
        s.log.Info().
            Str("job_id", jobID).
            Int64("processed", processed).
            Int64("failed", failed).
            Msg("PopulateDB job completed")
    }()

    return &datav1.PopulateDBResponse{
        Success:   true,
        Processed: 0,  // async — will be updated via NATS events
        Message:   fmt.Sprintf("Fetch from '%s' queued as job %s", source, jobID),
        JobId:     jobID,
    }, nil
}

// GetStats returns current CVE database statistics.
func (s *CVEDBServer) GetStats(ctx context.Context, _ *datav1.GetStatsRequest) (*datav1.GetStatsResponse, error) {
    stats, err := s.statsUC.GetDBStats(ctx)
    if err != nil {
        return nil, fmt.Errorf("GetStats: %w", err)
    }
    return &datav1.GetStatsResponse{
        TotalCves:  stats.TotalCVEs,
        LastSynced: stats.LastSyncedUnix,
        LastSource: stats.LastSource,
    }, nil
}
```

### Bước 4: FetchFromSource trong fetch_cve UseCase

**File sửa:** `data-service/internal/usecase/fetch_cve/usecase.go` (hoặc tương đương)

```go
// FetchFromSource fetches CVEs from a named source.
// Returns: processed count, failed count, error
func (uc *UseCase) FetchFromSource(ctx context.Context, source string, forceRefresh bool) (int64, int64, error) {
    fetcher, ok := uc.registry[source]
    if !ok {
        // "all" → run all fetchers
        if source == "all" {
            var totalProcessed, totalFailed int64
            for name, f := range uc.registry {
                p, fl, err := uc.runFetcher(ctx, name, f, forceRefresh)
                if err != nil {
                    uc.log.Warn().Err(err).Str("source", name).Msg("fetcher failed")
                }
                totalProcessed += p
                totalFailed += fl
            }
            return totalProcessed, totalFailed, nil
        }
        return 0, 0, fmt.Errorf("unknown source: %s", source)
    }
    return uc.runFetcher(ctx, source, fetcher, forceRefresh)
}

func (uc *UseCase) runFetcher(ctx context.Context, name string, f CVEFetcher, forceRefresh bool) (int64, int64, error) {
    since := uc.lastSyncRepo.GetLastSync(ctx, name)
    if forceRefresh {
        since = time.Time{} // fetch all
    }

    ch, err := f.FetchSince(ctx, since)
    if err != nil {
        return 0, 0, fmt.Errorf("fetch from %s: %w", name, err)
    }

    var processed, failed int64
    for rec := range ch {
        if err := uc.cveRepo.Upsert(ctx, rec); err != nil {
            failed++
            continue
        }
        _ = uc.publisher.Publish(ctx, "ingestion.cve.synced", map[string]string{
            "cve_id": rec.CVEID, "action": "upsert", "source": name,
        })
        processed++
    }

    _ = uc.lastSyncRepo.SetLastSync(ctx, name, time.Now())
    return processed, failed, nil
}
```

### Bước 5: Wire gRPC server trong data-service embed

**File sửa:** `data-service/embed/server.go`

```go
// [FIX CR-HC-014] Start real gRPC server
grpcPort := cfg.GRPCPort
if grpcPort <= 0 { grpcPort = 50054 }

grpcServer := grpcdelivery.NewCVEDBServer(fetchUseCase, statsUseCase, log)
lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
if err == nil {
    s := grpc.NewServer()
    datav1.RegisterCVEDBServiceServer(s, grpcServer)
    go func() {
        log.Info().Int("port", grpcPort).Msg("data-service: gRPC server started")
        if err := s.Serve(lis); err != nil {
            log.Error().Err(err).Msg("gRPC serve error")
        }
    }()
    go func() {
        <-ctx.Done()
        s.GracefulStop()
    }()
}
```

---

## Files cần tạo/sửa

| Action | File |
|--------|------|
| NEW | `shared/proto/data/v1/cvedb.proto` (nếu chưa có) |
| NEW | `data-service/internal/delivery/grpc/cvedb_server.go` |
| MODIFY | `data-service/internal/usecase/fetch_cve/usecase.go` — thêm FetchFromSource |
| MODIFY | `data-service/embed/server.go` — start gRPC listener |

---

## Verification

```bash
# Build và generate proto (nếu cần)
protoc --go_out=. --go-grpc_out=. shared/proto/data/v1/cvedb.proto

cd services/data-service && go build ./...

# Test gRPC (cần grpcurl)
grpcurl -plaintext c12.openledger.vn:50054 data.v1.CVEDBService/GetStats
# Expect: {"totalCves": NNN, "lastSynced": UNIX_TS}

grpcurl -plaintext -d '{"source":"nvd"}' c12.openledger.vn:50054 data.v1.CVEDBService/PopulateDB
# Expect: {"success":true,"message":"Fetch from 'nvd' queued as job nvd-..."}
```
