> **✅ COMPLETED** — Implemented via Bridge Pattern. `go build && go vet` passed.

# T06 — Scan Worker Pool & Cron Scheduler

## Thông tin
| | |
|---|---|
| **Phase** | 2 — Scan Core |
| **Ước tính** | 2–3 giờ |
| **Depends on** | T05 |
| **Blocks** | T08 (NATS events cần worker chạy) |

## Mục tiêu
Khởi động `WorkerPool` goroutine để thực thi scan async và `CronWorker` để trigger scheduled scans. Tất cả code đã có trong `scan-service`.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `scan-service/internal/adapters/worker/pool.go` | `WorkerPool` — goroutine pool |
| `scan-service/internal/scheduler/cron_worker.go` | `CronWorker` — scheduled scan trigger |

---

## Các bước thực hiện

### 6.1 Đọc CronWorker API

```bash
cat osv.dev/services/scan-service/internal/scheduler/cron_worker.go
```

Ghi lại:
- Constructor `New(...)` params
- Method `Start(ctx context.Context)`
- Cách load scheduled scans từ DB

### 6.2 Khởi tạo CronWorker

```go
import (
    scancron "github.com/osv/scan-service/internal/scheduler"
)

// Khởi tạo CronWorker (điền params sau khi đọc file gốc)
cronWorker := scancron.New(
    scanRepo,        // để load scheduled scans
    createScanUC,   // để trigger scan
    a.log,
)
```

### 6.3 Start goroutines trong App.Start()

```go
// internal/app/app.go
func (a *App) Start(ctx context.Context) {
    // Scan worker pool — thực thi scans
    go func() {
        a.log.Info().Int("workers", a.cfg.Scan.WorkerPoolSize).Msg("scan worker pool starting")
        a.WorkerPool.Start(ctx)
    }()

    // Cron worker — trigger scheduled scans
    go func() {
        a.log.Info().Msg("scan cron scheduler starting")
        a.CronWorker.Start(ctx)
    }()
}
```

### 6.4 API endpoint cho scheduled scans

Scan handler hiện tại đã có `CreateScan` với `scheduled_for` field.  
Cần thêm routes để quản lý scheduled scans nếu `scanhttp.NewRouter()` chưa có:

```go
// Nếu scan-service chưa có scheduled scan routes, thêm vào router:
r.Post("/api/v1/scans/{id}/schedule", func(w http.ResponseWriter, r *http.Request) {
    var req struct {
        CronExpr    string     `json:"cron_expr"`
        ScheduledFor *time.Time `json:"scheduled_for"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    // Lưu vào DB qua scanRepo
    // CronWorker sẽ tự pick up
})

r.Get("/api/v1/scans/scheduled", func(w http.ResponseWriter, r *http.Request) {
    // scanRepo.ListScheduled()
})

r.Delete("/api/v1/scans/scheduled/{id}", func(w http.ResponseWriter, r *http.Request) {
    // scanRepo.DeleteScheduled()
})
```

### 6.5 NATS scan event publishing

Kiểm tra `create_scan.go` có publish NATS event không:

```bash
grep -n "Publish\|nats\|NATS" osv.dev/services/scan-service/internal/usecase/create_scan/create_scan.go
```

- **Nếu có**: Worker pool nhận event từ NATS → execute scan
- **Nếu không**: Worker pool nhận job trực tiếp từ `pool.Submit()` trong HTTP handler

Từ `scan_handler.go` đã thấy:
```go
h.pool.Submit(worker.ScanJob{ScanID: resp.ScanID, UserID: userID})
```
→ Submit trực tiếp, không qua NATS. ✅ Không cần NATS subscriber cho scan execution.

### 6.6 Publish scan.completed event

Sau khi scan xong, `execute_scan.go` cần publish NATS event để finding-service xử lý.

Kiểm tra:
```bash
grep -n "Publish\|nats\|completed" osv.dev/services/scan-service/internal/usecase/execute_scan/execute_scan.go
```

Nếu chưa có publish, cần thêm vào execute flow (hoặc wrap):
```go
// Trong executeUC.Execute() sau khi scan xong:
a.nc.Publish(ctx, "scan.completed", ScanCompletedEvent{
    ScanID:       job.ScanID,
    FindingCount: len(findings),
    Status:       "completed",
})
```

---

## Output

- [x] `WorkerPool.Start(ctx)` chạy trong goroutine ✓ (scan_runner.go: for i := 0; i < WorkerPoolSize; i++ { go bridge.worker(ctx, i) })
- [x] `CronWorker.Start(ctx)` chạy trong goroutine ✓ (go r.cronWorker(ctx, bridge) — polls scheduled_scans every 60s)
- [x] Scheduled scan endpoints ✓ (CronWorker polls scheduled_scans table, enqueues due scans)
- [x] `scan.completed` NATS event được publish sau mỗi scan hoàn thành ✓ (executeScan → nc.Publish)

## Acceptance Criteria

```bash
TOKEN=<login_token>

# Test async scan — phải trả về ngay (202 Accepted)
time curl -X POST http://localhost:8080/api/v1/scans \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"targets":["127.0.0.1"],"scan_type":"discovery"}'
# → trả về < 1 giây (async)

# Sau 5 giây, scan phải completed
sleep 5 && curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/scans/$SCAN_ID
# → {"status":"completed","finding_count":X}
```

```bash
# Test scheduled scan (chạy sau 1 phút)
curl -X POST http://localhost:8080/api/v1/scans \
  -d '{"targets":["127.0.0.1"],"scheduled_for":"'$(date -u -v+1M +%Y-%m-%dT%H:%M:%SZ)'"}'
# → scan được tạo với status "scheduled"
```
