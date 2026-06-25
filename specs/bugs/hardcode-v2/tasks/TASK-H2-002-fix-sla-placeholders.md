# TASK-H2-002 — Fix BUG-H2-004: sla-service placeholder handlers

> **Bugs**: BUG-H2-004
> **Solution**: [SOL-H2-B](../solutions/SOL-H2-B-sla-service-placeholder.md)
> **Status**: ✅ Done — Build verified ✓

## Checklist

- [x] Kiểm tra `SLAConfigHandler` có đủ methods: `List`, `Create`, `Get`, `Update`, `Delete`, `BulkCreate`, `BulkAssign`
- [x] Thêm methods còn thiếu vào `SLAConfigHandler` nếu cần
- [x] Thay 5 placeholder mounts bằng `slaCfgHandler.*` trong `main.go`
- [x] Xóa 7 placeholder functions khỏi `main.go`

## Files Modified

- `services/sla-service/cmd/server/main.go` [MODIFY]
- `services/sla-service/internal/delivery/http/sla_config_handler.go` [MODIFY if needed]
