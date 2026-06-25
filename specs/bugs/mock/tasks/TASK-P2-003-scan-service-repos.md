# TASK-P2-003 — Wire ScanRepo và StatsRepo cho scan-service

**Bug:** MOCK-006  
**Priority:** 🟡 P2 — Data không được lưu  
**Effort:** ~2 giờ  
**Service:** `scan-service`  
**Loại thay đổi:** New files + DB migration + Wire embedded.go  
**Depends on:** TASK-P1-001 (AgentRepo đã wire)

---

## Mục tiêu

`scan-service/embedded.go` khởi tạo `ScanAPIHandler` và `StatsHandler` với `nil` repo. Mọi scan được tạo không được lưu vào DB; stats luôn trả 0.

---

## Preconditions

- [ ] Đọc `services/scan-service/embedded.go`
- [ ] Đọc `services/scan-service/internal/delivery/http/scan_api_handler.go`
- [ ] Đọc `services/scan-service/internal/delivery/http/stats_handler.go`
- [ ] Xác định domain entity cho Scan:
  ```bash
  find services/scan-service/internal/domain -name "scan*"
  ```
- [ ] Kiểm tra repos đã có:
  ```bash
  ls services/scan-service/internal/infra/postgres/
  ```

---

## Steps

### Step 1 — Xác định Scan domain entity và ScanRepository interface

```bash
grep -rn "type Scan struct\|ScanRepository\|type.*Repo.*interface" \
  services/scan-service/internal/domain/ \
  services/scan-service/internal/infra/
```

### Step 2 — Tạo DB migrations

**File mới**: `services/scan-service/internal/infra/postgres/migrations/XXX_add_scans.sql`

```sql
CREATE TABLE IF NOT EXISTS scans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID,
    test_id     UUID,
    tool        VARCHAR(50) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','queued','running','completed','failed','cancelled')),
    targets     JSONB DEFAULT '[]',
    scan_type   VARCHAR(50),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scans_status     ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_product_id ON scans(product_id);
CREATE INDEX IF NOT EXISTS idx_scans_created_at ON scans(created_at DESC);
```

### Step 3 — Tạo ScanRepo (nếu chưa có)

```bash
find services/scan-service -name "scan_repo*" -o -name "*scan*repo*"
```

Nếu chưa có:

**File mới**: `services/scan-service/internal/infra/postgres/scan_repo.go`

Implement các methods cần thiết theo ScanAPIHandler yêu cầu:
```bash
grep -n "h\.repo\." services/scan-service/internal/delivery/http/scan_api_handler.go
```

Với mỗi method được gọi, implement tương ứng trong repo.

### Step 4 — Tạo ScanStatsRepo (nếu chưa có)

```bash
grep -n "h\.repo\.\|h\.statsRepo\." services/scan-service/internal/delivery/http/stats_handler.go
```

**File mới**: `services/scan-service/internal/infra/postgres/scan_stats_repo.go`

```go
package postgres

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type ScanStatsRepo struct {
    db *pgxpool.Pool
}

func NewScanStatsRepo(db *pgxpool.Pool) *ScanStatsRepo {
    return &ScanStatsRepo{db: db}
}

// GetStats trả tổng thống kê scans theo status
func (r *ScanStatsRepo) GetStats(ctx context.Context) (map[string]int64, error) {
    rows, err := r.db.Query(ctx, `
        SELECT status, COUNT(*) as count FROM scans GROUP BY status
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    stats := make(map[string]int64)
    for rows.Next() {
        var status string
        var count int64
        if err := rows.Scan(&status, &count); err != nil {
            continue
        }
        stats[status] = count
    }
    return stats, rows.Err()
}

// GetStatsByProduct trả stats scans cho một product cụ thể
func (r *ScanStatsRepo) GetStatsByProduct(ctx context.Context, productID string) (map[string]int64, error) {
    rows, err := r.db.Query(ctx, `
        SELECT status, COUNT(*) as count FROM scans
        WHERE product_id = $1 GROUP BY status
    `, productID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    stats := make(map[string]int64)
    for rows.Next() {
        var status string
        var count int64
        rows.Scan(&status, &count)
        stats[status] = count
    }
    return stats, rows.Err()
}
```

### Step 5 — Wire trong embedded.go

Mở `services/scan-service/embedded.go`.

Tìm:
```go
scanHandler  := httpdelivery.NewScanAPIHandler(nil, logger)
statsHandler := httpdelivery.NewStatsHandler(nil, logger)
```

Thay bằng:
```go
// FIX MOCK-006: Wire real PostgreSQL repositories
scanRepo  := postgres.NewScanRepo(pool)
statsRepo := postgres.NewScanStatsRepo(pool)

scanHandler  := httpdelivery.NewScanAPIHandler(scanRepo, logger)
statsHandler := httpdelivery.NewStatsHandler(statsRepo, logger)
```

---

## Acceptance Criteria

- [ ] `POST /api/v1/scans` → scan được lưu vào bảng `scans`
- [ ] `GET /api/v1/scans` → trả danh sách scans thực từ DB
- [ ] `GET /api/v1/scans/stats` (hoặc endpoint stats tương ứng) → trả counts thực
- [ ] Sau restart service, scans đã tạo vẫn còn trong DB
- [ ] `go build ./services/scan-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/scan-service/...
go vet ./services/scan-service/...

# Verify nil repos removed
grep -n "NewScanAPIHandler(nil\|NewStatsHandler(nil" services/scan-service/embedded.go
# Expected: no output

go test ./services/scan-service/internal/... -v -run Scan
```
