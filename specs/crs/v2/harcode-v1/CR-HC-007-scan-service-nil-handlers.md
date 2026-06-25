# CR-HC-007: scan-service — Nil Handlers trong Embedded Mode

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/scan-service/embedded.go`

```go
router := httpdelivery.NewRouterFull(
    nil,          // importHandler — not wired in embedded mode
    nil,          // parserHandler — not wired in embedded mode
    agentHandler,
    scanHandler,
    nil,          // scheduleHandler — not wired yet
    statsHandler,
    logger,
)
```

Ba handlers đang là `nil`:
1. `importHandler` — không thể import scan results từ external tools
2. `parserHandler` — không thể parse scan files
3. `scheduleHandler` — không thể schedule scans

Khi nil handler được gọi, router có thể panic hoặc trả 500.

## Phân tích từng handler

### 1. importHandler
**File:** `services/scan-service/internal/usecase/orchestrator/import/import_scan.go`
- Import scan kết quả từ DAST/SAST tools (Burp, ZAP, Nessus...)
- Cần: `ScanRepo`, `FindingRepo` (gRPC → finding-service)
- **Production requirement**: bắt buộc cho CI/CD integration

### 2. parserHandler  
**File:** `services/scan-service/internal/parsers/`
- Parse XML/JSON từ scan tools
- Cần: Parser Factory, Converter interfaces
- **Production requirement**: bắt buộc khi nhận scan results từ agent

### 3. scheduleHandler
**File:** `services/scan-service/internal/scheduler/`
- Scheduled scan management (cron-based)
- Cần: `ScheduledScanRepo`, NATS publisher
- **Production requirement**: enterprise feature

## Giải pháp

### 1. Wire importHandler
```go
// In embedded.go (when pool != nil):
importUC := importscan.NewUseCase(scanRepo, findingGRPCClient, logger)
importHandler = httpdelivery.NewImportHandler(importUC, logger)
logger.Info().Msg("scan-service: ImportHandler wired")
```

### 2. Wire parserHandler
```go
parserFactory := parser.NewFactory()
parserHandler = httpdelivery.NewParserHandler(parserFactory, logger)
```

### 3. Wire scheduleHandler (cần migration)
```go
scheduleRepo := pgadapter.NewScheduledScanRepo(pool)
scheduleUC := scheduleuc.NewUseCase(scheduleRepo, natsPublisher, logger)
scheduleHandler = httpdelivery.NewScheduleHandler(scheduleUC, logger)
```

#### Migration cần thêm:
```sql
-- scan.scheduled_scans đã được tạo (CR-V5-001)
-- Cần thêm: user_id, next_run, last_run columns
ALTER TABLE scan.scheduled_scans
    ADD COLUMN IF NOT EXISTS user_id UUID,
    ADD COLUMN IF NOT EXISTS next_run TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_run TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS description TEXT;
```

### 4. Graceful 501 khi chưa implement
Thay vì `nil` → panic, đăng ký handler trả `501 Not Implemented`:
```go
// Utility: not-yet-implemented handler (no panic)
func notImplemented(feature string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusNotImplemented)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "not implemented",
            "feature": feature,
        })
    }
}
```

## Files cần thay đổi
- `services/scan-service/embedded.go` — wire importHandler, parserHandler, scheduleHandler
- `services/scan-service/internal/delivery/http/import_handler.go` [NEW hoặc implement thật]
- `services/scan-service/internal/adapters/repository/postgres/schedule_repo.go` [NEW]
- `services/scan-service/migrations/005_scheduled_scans_columns.sql` [NEW]

## Acceptance Criteria
- [ ] `POST /api/v1/scans/import` → 200 hoặc 501 (không panic)
- [ ] `GET /api/v1/scans/scheduled` → 200 từ DB (đã fix)
- [ ] `POST /api/v1/scans/scheduled` → tạo scheduled scan trong DB
- [ ] Không có `nil` handler trong production router
