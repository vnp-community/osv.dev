# TASK-P3-002 — Wire NATS EventBus cho BulkHandler

**Bug:** MOCK-003  
**Priority:** 🟡 P3 — Silent Data Loss  
**Effort:** ~15 phút  
**Service:** `finding-service`  
**Loại thay đổi:** Sửa 1 dòng trong embedded.go  
**Depends on:** TASK-P3-001 (OutboxPublisher đã được tạo)

---

## Mục tiêu

`NewBulkHandler(..., nil, ...)` — argument `nil` là `eventBus`. Khi `BulkReopen` được gọi, event `finding.status.changed` không được publish vì `eventBus == nil`. Cần pass `outboxPub` thay vì `nil`.

---

## Preconditions

- [ ] TASK-P3-001 đã hoàn thành (OutboxPublisher đã tồn tại)
- [ ] Đọc `services/finding-service/embedded.go` — xác định dòng `NewBulkHandler`
- [ ] Xác định đúng vị trí argument `nil` (là argument thứ mấy):
  ```bash
  grep -n "func NewBulkHandler" services/finding-service/internal/delivery/http/bulk_handler.go
  ```

---

## Steps

### Step 1 — Xác định BulkHandler constructor

```bash
grep -n "func NewBulkHandler\|NewBulkHandler(" services/finding-service/internal/delivery/http/bulk_handler.go
```

Ghi lại signature, ví dụ:
```go
func NewBulkHandler(bulkUC BulkUseCase, repo FindingRepository, eventBus EventBus, log zerolog.Logger) *BulkHandler
```

### Step 2 — Tìm dòng NewBulkHandler trong embedded.go

```bash
grep -n "NewBulkHandler" services/finding-service/embedded.go
```

### Step 3 — Thay nil bằng outboxPub

Trong `services/finding-service/embedded.go`, tìm:
```go
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, nil, logger)
```

Thay bằng:
```go
// FIX MOCK-003: pass outboxPub thay vì nil cho eventBus
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, outboxPub, logger)
```

> **Quan trọng**: `outboxPub` phải được khai báo TRƯỚC dòng này trong code. Nếu TASK-P3-001 chưa done, có thể dùng `loggingNoopPublisher` như giải pháp interim:

```go
// Interim fix nếu outboxPub chưa có: noop với logging (không mất silently)
type loggingEventBus struct{ logger zerolog.Logger }
func (b *loggingEventBus) Publish(ctx context.Context, subject string, data interface{}) error {
    payload, _ := json.Marshal(data)
    b.logger.Warn().Str("subject", subject).RawJSON("payload", payload).
        Msg("EventBus: NATS unavailable, event dropped — MOCK-003 interim")
    return nil
}

// Sau đó:
eventBus := &loggingEventBus{logger: logger}
bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, eventBus, logger)
```

---

## Acceptance Criteria

- [ ] `nil` không còn được pass vào argument `eventBus` của `NewBulkHandler`
- [ ] Sau `BulkReopen`, event `finding.status.changed` được ghi vào outbox (hoặc logged)
- [ ] `go build ./services/finding-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/finding-service/...

# Verify nil removed
grep -n "NewBulkHandler" services/finding-service/embedded.go
# Expected: không có "nil" trong argument list
```
