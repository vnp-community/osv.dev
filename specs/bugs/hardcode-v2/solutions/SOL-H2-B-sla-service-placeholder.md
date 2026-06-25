# SOL-H2-B — sla-service: Wire SLAConfigHandler cho /api/v2/sla-configurations

> Bugs: BUG-H2-004
> Service: `sla-service`

---

## Tổng quan

Xóa 5 placeholder functions trong `main.go` và wire `httpdelivery.SLAConfigHandler` cho tất cả routes `/api/v2/sla-configurations/*`.

---

## Phân tích hiện trạng

`httpdelivery.SLAConfigHandler` đã được khởi tạo tại main.go:
```go
slaCfgHandler := httpdelivery.NewSLAConfigHandler(
    ucconfig.NewCreate(slaCfgRepoImpl, nil),
    ucconfig.NewUpdate(slaCfgRepoImpl, nil),
    ucconfig.NewDelete(slaCfgRepoImpl, nil),
    ucconfig.NewAssignProduct(slaCfgRepoImpl, nil, nil),
    slaCfgRepoImpl,
)
```

Nhưng chỉ được mount cho `/api/v1/sla/config` (GET, PUT). Cần extend SLAConfigHandler để cover `/api/v2/sla-configurations/*`.

---

## Fix

### Bước 1: Kiểm tra `SLAConfigHandler` có đủ methods

Cần có: `List`, `Create`, `Get`, `Update`, `Delete`, `BulkCreate`, `BulkAssign`.

Nếu thiếu, thêm vào `httpdelivery.SLAConfigHandler`.

### Bước 2: Thay placeholder functions bằng SLAConfigHandler

```go
// main.go — TRƯỚC (BUG)
r.Route("/api/v2/sla-configurations", func(r chi.Router) {
    r.Post("/bulk", bulkCreateSLAConfigsHandler(pool))   // ← placeholder
    r.Post("/assign-bulk", bulkAssignSLAConfigsHandler(pool)) // ← placeholder
    r.Get("/", listSLAConfigsHandler(pool))              // ← placeholder
    r.Post("/", createSLAConfigHandler(pool))            // ← 501
    r.Get("/{id}", getSLAConfigHandler(pool))            // ← 200 with error body
    r.Put("/{id}", updateSLAConfigHandler(pool))         // ← 501
    r.Delete("/{id}", deleteSLAConfigHandler(pool))      // ← 204 no-op
})

// main.go — SAU (FIX)
r.Route("/api/v2/sla-configurations", func(r chi.Router) {
    r.Post("/bulk", slaCfgHandler.BulkCreate)
    r.Post("/assign-bulk", slaCfgHandler.BulkAssign)
    r.Get("/", slaCfgHandler.List)
    r.Post("/", slaCfgHandler.Create)
    r.Get("/{id}", slaCfgHandler.Get)
    r.Put("/{id}", slaCfgHandler.Update)
    r.Delete("/{id}", slaCfgHandler.Delete)
})
```

### Bước 3: Xóa 5 placeholder functions khỏi main.go

Xóa: `listSLAConfigsHandler`, `createSLAConfigHandler`, `getSLAConfigHandler`, `updateSLAConfigHandler`, `deleteSLAConfigHandler`, `bulkCreateSLAConfigsHandler`, `bulkAssignSLAConfigsHandler`.

---

## Files cần modify

| File | Thay đổi |
|------|----------|
| `sla-service/cmd/server/main.go` | Replace placeholder mounts, xóa 7 functions |
| `sla-service/internal/delivery/http/sla_config_handler.go` | Thêm `BulkCreate`, `BulkAssign`, `List`, `Get`, `Delete` nếu thiếu |
