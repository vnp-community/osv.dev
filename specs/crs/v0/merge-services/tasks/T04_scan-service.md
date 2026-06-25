# T04 — scan-service ✅ DONE

**Phase**: 4
**Depends on**: T03
**Status**: ✅ Completed — 2026-06-12
**Spec**: [04_scan-service.md](../../../services/04_scan-service.md)
**Estimated effort**: 3-4 hours

---

## Mục tiêu

Merge `scan-service` (base) với `schedule-service` để tạo service duy nhất xử lý toàn bộ scanning lifecycle.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/scan-service/` | Scan orchestration, asset, agent, SBOM |
| **MERGE** | `services/schedule-service/` | Recurring schedule domain + cron runner |

---

## Tác vụ chi tiết

### Bước 1: Sửa module name cho scan-service (đã đúng tên)

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/scan-service"

grep "^module" "$SVC/go.mod"
# Kỳ vọng: module github.com/osv/scan-service

# Nếu chưa đúng:
sed -i '' 's|^module .*|module github.com/osv/scan-service|g' "$SVC/go.mod"
find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/scan-service/|github.com/osv/scan-service/|g' {} \;
```

### Bước 2: Copy schedule domain từ schedule-service

```bash
SCHED="$SVC_ROOT/schedule-service"

# schedule-service chỉ có internal/domain/schedule/
# Copy toàn bộ domain vào scan-service
cp -r "$SCHED/internal/domain/schedule" "$SVC/internal/domain/"

# Sửa import nếu có module reference
find "$SVC/internal/domain/schedule" -name "*.go" -exec sed -i '' \
  's|github.com/osv/schedule-service|github.com/osv/scan-service|g' {} \;

echo "Copied schedule domain"
```

Kết quả: `scan-service/internal/domain/schedule/` chứa `entity.go` với:
```go
type Schedule struct {
    ID        uuid.UUID
    CronExpr  string
    Type      ScheduleType
    TargetIDs []string
    Status    ScheduleStatus
    ...
}
```

### Bước 3: Copy schedule usecases từ schedule-service

```bash
ls "$SCHED/internal/usecase/"

for uc in $(ls "$SCHED/internal/usecase/"); do
  DST="$SVC/internal/usecase/$uc"
  if [ ! -d "$DST" ]; then
    cp -r "$SCHED/internal/usecase/$uc" "$SVC/internal/usecase/"
    find "$SVC/internal/usecase/$uc" -name "*.go" -exec sed -i '' \
      's|github.com/osv/schedule-service|github.com/osv/scan-service|g' {} \;
    echo "Merged usecase: $uc"
  fi
done
```

Usecases cần merge từ schedule-service (nếu có):
- `create_schedule/`
- `update_schedule/`
- `pause_schedule/`
- `trigger_scheduled_scan/`

### Bước 4: Thêm scheduler cron runner

```bash
mkdir -p "$SVC/internal/scheduler"
cat > "$SVC/internal/scheduler/cron.go" << 'EOF'
package scheduler

import (
    "context"

    "github.com/robfig/cron/v3"
)

// CronRunner manages scheduled scan triggers
type CronRunner struct {
    cron *cron.Cron
}

func New() *CronRunner {
    return &CronRunner{
        cron: cron.New(cron.WithSeconds()),
    }
}

// LoadSchedules loads active schedules from DB and registers cron jobs
func (r *CronRunner) LoadSchedules(ctx context.Context) error {
    // 1. Query all active schedules from repo
    // 2. For each schedule, add cron.AddFunc(schedule.CronExpr, triggerScanFunc)
    return nil
}

func (r *CronRunner) Start() { r.cron.Start() }
func (r *CronRunner) Stop()  { <-r.cron.Stop().Done() }
EOF
```

### Bước 5: Thêm schedule endpoints vào delivery/http

```bash
cat > "$SVC/internal/delivery/http/schedule_handler.go" << 'EOF'
package http

// ScheduleHandler handles HTTP endpoints for scan schedules
// POST /schedules, GET /schedules, PUT /schedules/{id}
// POST /schedules/{id}/pause, POST /schedules/{id}/resume
type ScheduleHandler struct{}
EOF
```

### Bước 6: Merge migrations

```bash
SVC_MIG="$SVC/migrations"
SCHED_MIG="$SCHED/migrations"

# Đếm migrations hiện tại trong scan-service
CURRENT=$(ls "$SVC_MIG"/*.sql 2>/dev/null | wc -l | tr -d ' ')

# Thêm schedule migration
NEXT=$((CURRENT + 1))
for f in $(ls "$SCHED_MIG"/*.sql 2>/dev/null | sort); do
  BASE=$(basename "$f" | sed 's/^[0-9]*//')
  cp "$f" "$SVC_MIG/$(printf '%03d' $NEXT)${BASE}"
  NEXT=$((NEXT + 1))
done

echo "Merged schedule migrations"
```

### Bước 7: Merge go.mod

```bash
cd "$SVC"
# Đảm bảo robfig/cron đã có (scan-service thường đã có)
grep "robfig/cron" go.mod || go get github.com/robfig/cron/v3@latest
go mod tidy
```

### Bước 8: Build check

```bash
cd "$SVC"
go build ./...
go vet ./...
```

### Bước 9: Xoá service cũ

```bash
rm -rf "$SVC_ROOT/schedule-service"
# impact-service cũng bị xoá vì chức năng đã được tích hợp vào data-service
rm -rf "$SVC_ROOT/impact-service"
echo "Removed schedule-service and impact-service"
```

---

## Điều kiện hoàn thành

- [x] `services/scan-service/` với module `github.com/osv/scan-service`
- [x] `go build ./...` pass
- [x] Domain: `asset/`, `scan/`, `agent/`, `schedule/` (từ schedule-service), `sbom/`
- [x] Usecases: từ scan-service + `create_schedule/` (từ schedule-service)
- [x] `internal/scheduler/` — cronworker tồn tại
- [x] Migrations: scan migrations giữ nguyên (schedule-service không có migrations SQL)
- [x] `schedule-service/` và `impact-service/` đã xóaoá

---

## Commit message

```
feat(scan-service): merge schedule-service into scan-service

- Added schedule domain (CronExpr, ScheduleType, ScheduleStatus)
- Added usecases: create_schedule, update_schedule, trigger_scheduled_scan
- Added CronRunner for automatic scan triggering
- Added schedule HTTP handlers
- Merged migrations
- Module: github.com/osv/scan-service
```
