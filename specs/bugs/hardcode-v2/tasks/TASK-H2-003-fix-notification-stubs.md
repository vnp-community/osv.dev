# TASK-H2-003 — Fix BUG-H2-005,006: notification-service stubs

> **Bugs**: BUG-H2-005 (Jira stubs), BUG-H2-006 (AlertsHandler nil)
> **Solution**: [SOL-H2-C](../solutions/SOL-H2-C-notification-service-stubs.md)
> **Status**: ✅ Done — Build verified ✓

## Checklist

- [x] Xóa 6 package-level Jira stubs (L243-251 trong integration_handler.go)
- [x] Kiểm tra embedded.go — xác nhận AlertsHandler được wire đúng (ah != nil tại L86, truyền vào SetupRouter L93)

## Files Modified

- `services/notification-service/internal/delivery/http/integration_handler.go` [MODIFY]
- `services/notification-service/embedded.go` [VERIFY/MODIFY if needed]
