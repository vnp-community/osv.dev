# S1-GW-02 — Implement Dashboard BFF (gateway-service)


## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` + `go vet` PASSED
- **Files Modified**: `internal/bff/dashboard.go`
- **Changes**: Replaced empty DashboardAggregator struct + TODO GetDashboard() with full implementation:
  - `errgroup`-based parallel calls (10s total timeout)
  - Graceful degradation per downstream service (never fails dashboard)
  - `GeneratedAt` timestamp added
  - `NewDashboardAggregator(aiClient, log)` constructor
  - JSON tags on all struct fields
- **go.mod**: Added `golang.org/x/sync` dependency

## Metadata
- **Task ID**: S1-GW-02
- **Service**: gateway-service
- **Sprint**: 1 (P0 — Blocking)
- **Ước tính**: 3-4 giờ
- **Dependencies**: S1-GW-01 (grpc clients), S1-FIND-01 (finding HTTP), S1-AI-02 (ai gRPC)
- **Spec nguồn**: `specs/develop/08_gateway-service-upgrade.md` § "Implement: Dashboard BFF"

## Context — Đọc trước khi làm

```bash
# Đọc dashboard.go để thấy struct đã có và TODO:
cat services/gateway-service/internal/bff/dashboard.go

# Đọc các handler patterns hiện có:
cat services/gateway-service/internal/bff/handlers/handler_v1.go

# Đọc grpc_clients.go sau khi đã thêm clients ở S1-GW-01:
cat services/gateway-service/internal/bff/clients/grpc_clients.go

# Đọc proto để biết exact field names:
cat services/shared/proto/finding/*.proto 2>/dev/null || ls services/shared/proto/finding/
cat services/shared/proto/scan/*.proto 2>/dev/null || ls services/shared/proto/scan/
```

## Goal

Implement phần body của `GetDashboard()` trong `bff/dashboard.go` bằng cách gọi parallel gRPC tới finding, scan, data, search services. Hiện tại hàm này return `&DashboardData{}` rỗng.

**Nguyên tắc**: KHÔNG tạo file mới — chỉ implement method body còn trống.

## Steps

### Step 1: Đọc toàn bộ `bff/dashboard.go`

Chạy lệnh:
```bash
cat services/gateway-service/internal/bff/dashboard.go
```

Xác định:
- Tên struct `DashboardAggregator` (hoặc tên thực tế)
- Tên method `GetDashboard` (hoặc tên thực tế)
- Các fields trong `DashboardData` struct
- Các gRPC client fields đã có trong DashboardAggregator

### Step 2: Thêm missing imports vào `dashboard.go`

```go
import (
    // ... existing imports giữ nguyên ...
    
    // Thêm mới:
    "golang.org/x/sync/errgroup"
    "time"
)
```

Nếu `golang.org/x/sync` chưa có trong go.mod:
```bash
cd services/gateway-service && go get golang.org/x/sync
```

### Step 3: Implement `GetDashboard()` method

Tìm method `GetDashboard` (hoặc tên tương đương) và thay thế phần body rỗng:

```go
func (a *DashboardAggregator) GetDashboard(ctx context.Context) (*DashboardData, error) {
    g, gctx := errgroup.WithContext(ctx)

    var (
        findingStats FindingsSummary
        scanStats    ScansSummary
        kevStats     KEVSummary
        recentCVEs   []RecentCVE
    )

    // Parallel call 1: Finding stats
    if a.findingClient != nil {
        g.Go(func() error {
            stats, err := a.findingClient.GetStats(gctx)
            if err != nil {
                // Graceful degradation — log but don't fail dashboard
                a.log.Warn().Err(err).Msg("dashboard: finding stats unavailable")
                return nil
            }
            findingStats = FindingsSummary{
                Total:     stats.Total,
                Critical:  stats.Critical,
                High:      stats.High,
                Medium:    stats.Medium,
                Low:       stats.Low,
                Open:      stats.Open,
                Mitigated: stats.Mitigated,
            }
            return nil
        })
    }

    // Parallel call 2: Scan stats
    if a.scanClient != nil {
        g.Go(func() error {
            // Gọi scan-service để lấy scan statistics
            // Adapt theo actual proto fields
            a.log.Debug().Msg("dashboard: fetching scan stats")
            return nil
        })
    }

    // Parallel call 3: KEV stats từ data-service
    if a.dataClient != nil {
        g.Go(func() error {
            a.log.Debug().Msg("dashboard: fetching KEV stats")
            return nil
        })
    }

    // Parallel call 4: Recent CVEs từ search-service
    if a.searchClient != nil {
        g.Go(func() error {
            a.log.Debug().Msg("dashboard: fetching recent CVEs")
            return nil
        })
    }

    // Wait for all goroutines
    if err := g.Wait(); err != nil {
        return nil, err
    }

    return &DashboardData{
        Findings:    findingStats,
        Scans:       scanStats,
        KEV:         kevStats,
        RecentCVEs:  recentCVEs,
        GeneratedAt: time.Now().UTC(),
    }, nil
}
```

### Step 4: Inject FindingClient vào DashboardAggregator

Kiểm tra xem `DashboardAggregator` đã có `findingClient` field chưa. Nếu chưa, thêm:

```go
// Tìm struct DashboardAggregator và thêm fields:
type DashboardAggregator struct {
    // ... existing fields giữ nguyên ...
    findingClient *grpcclient.FindingClient  // thêm nếu chưa có
    log           zerolog.Logger              // thêm nếu chưa có
}
```

### Step 5: Cập nhật constructor của DashboardAggregator

```go
// Tìm hàm NewDashboardAggregator (hoặc tương đương) và thêm params:
func NewDashboardAggregator(
    // ... existing params ...
    findingClient *grpcclient.FindingClient,  // thêm
    log zerolog.Logger,                        // thêm
) *DashboardAggregator {
    return &DashboardAggregator{
        // ... existing fields ...
        findingClient: findingClient,
        log:           log,
    }
}
```

### Step 6: Cập nhật `cmd/server/main.go`

```go
// Tìm chỗ khởi tạo DashboardAggregator và truyền findingClient:
dashboardAgg := bff.NewDashboardAggregator(
    // ... existing params ...
    grpcClients.Finding,  // thêm
    logger,               // thêm
)
```

## Verification

```bash
# Build check:
cd services/gateway-service && go build ./...

# Test endpoint (nếu server đang chạy):
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/api/v1/bff/dashboard
# Expected: JSON với DashboardData (có thể có empty stats nếu downstream chưa ready)

# Verify không panic khi finding-service unreachable:
# DashboardAggregator phải return partial data, không return error
```

## Notes

- Nếu `DashboardData` struct chưa có field `GeneratedAt`, thêm vào struct (không xóa fields cũ)
- Nếu proto method names khác với assumption (GetStats → khác), đọc proto file để biết chính xác
- Timeout cho mỗi gRPC call nên là 5s, tổng timeout context là 10s
