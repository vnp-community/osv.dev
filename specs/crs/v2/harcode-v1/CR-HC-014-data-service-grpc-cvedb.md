# CR-HC-014: data-service — gRPC CVEDB Client là TODO

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/data-service/internal/adapter/grpcclient/cvedb_client.go:39`

```go
// TODO: wire to real proto once CVEDB service exposes PopulateDB via gRPC.
// For now, this method is intentionally unimplemented.
func (c *CVEDBClient) PopulateDB(ctx context.Context, req *PopulateDBRequest) error {
    return errors.New("not implemented")
}
```

`PopulateDB` via gRPC chưa implement. Chức năng này cần thiết để:
- Đồng bộ CVE data từ external sources vào database
- Trigger manual population từ admin UI
- Pipeline CI/CD nạp data mới

## Phân tích

CVE data hiện tại được sync qua:
- HTTP fetchers (NVD, OSV.dev APIs) trong `data-service/internal/fetcher/`
- Sync usecase trong `data-service/internal/usecase/syncall/`

gRPC endpoint cho `PopulateDB` cần:
1. Định nghĩa proto file
2. Implement server-side handler
3. Expose qua gRPC port (`:9091` hoặc tương tự)

## Giải pháp

### 1. Proto definition
```protobuf
// services/data-service/proto/cvedb/v1/cvedb.proto
syntax = "proto3";
package cvedb.v1;

service CVEDBService {
    rpc PopulateDB(PopulateDBRequest) returns (PopulateDBResponse);
    rpc GetSyncStatus(GetSyncStatusRequest) returns (SyncStatusResponse);
}

message PopulateDBRequest {
    string source = 1;        // "nvd" | "osv" | "all"
    string since   = 2;       // RFC3339 — sync from this date
    bool   dry_run = 3;
}

message PopulateDBResponse {
    int64 processed = 1;
    int64 inserted  = 2;
    int64 updated   = 3;
    int64 errors    = 4;
    string status   = 5;      // "ok" | "partial" | "failed"
}
```

### 2. gRPC Server implementation
```go
type CVEDBServer struct {
    cvedbpb.UnimplementedCVEDBServiceServer
    syncUC SyncAllUseCase
    log    zerolog.Logger
}

func (s *CVEDBServer) PopulateDB(ctx context.Context, req *cvedbpb.PopulateDBRequest) (*cvedbpb.PopulateDBResponse, error) {
    result, err := s.syncUC.Execute(ctx, SyncInput{
        Source:  req.Source,
        Since:   req.Since,
        DryRun:  req.DryRun,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "sync failed: %v", err)
    }
    return &cvedbpb.PopulateDBResponse{
        Processed: result.Processed,
        Inserted:  result.Inserted,
        Updated:   result.Updated,
        Errors:    result.Errors,
        Status:    result.Status,
    }, nil
}
```

### 3. Expose gRPC trong main.go
```go
grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(authInterceptor))
cvedbpb.RegisterCVEDBServiceServer(grpcServer, cvedbServer)
go grpcServer.Serve(lis) // port :9091
```

## Files cần thay đổi
- `services/data-service/proto/cvedb/v1/cvedb.proto` [NEW]
- `services/data-service/internal/delivery/grpc/cvedb_server.go` [NEW]
- `services/data-service/internal/adapter/grpcclient/cvedb_client.go` — implement
- `services/data-service/cmd/server/main.go` — wire gRPC server

## Acceptance Criteria
- [ ] `grpcurl` call `cvedb.v1.CVEDBService/PopulateDB` → 200 OK
- [ ] CVE count trong DB tăng sau khi gọi PopulateDB
- [ ] `GetSyncStatus` trả được trạng thái sync hiện tại
