# Task Index — Merge Services

> Tổng số tasks: **25 tasks** chia thành **8 phases**
> Thứ tự thực hiện: Phase 0 → Phase 7 (sequential, không skip)
> Archive sẽ bị xoá sau khi toàn bộ merge hoàn tất (Phase 7)

---

## Tổng quan Phases

| Phase | Tên | Tasks | Mô tả |
|-------|-----|-------|-------|
| 0 | Setup | [T00](./T00_setup_workspace.md) | Chuẩn bị workspace, tạo thư mục services mới |
| 1 | identity-service | [T01](./T01_identity-service.md) | Merge auth-service + archive/identity + archive/admin |
| 2 | data-service | [T02](./T02_data-service.md) | Merge vulnerability-service + ingestion-service |
| 3 | search-service | [T03](./T03_search-service.md) | Merge search-service + query-service + dd-search |
| 4 | scan-service | [T04](./T04_scan-service.md) | Merge scan-service + schedule-service |
| 5 | finding-service | [T05](./T05_finding-service.md) | Merge finding-service + product-service + report-service |
| 6 | ai-service | [T06](./T06_ai-service.md) | Refactor ai-service (đã gần đúng cấu trúc) |
| 7 | notification-service | [T07](./T07_notification-service.md) | Merge notification-service + integration-service |
| 8 | gateway-service | [T08](./T08_gateway-service.md) | Rename unified-gateway → gateway-service |
| 9 | shared | [T09](./T09_shared-layer.md) | Update shared/pkg module name |
| 10 | proto | [T10](./T10_proto-update.md) | Cập nhật tất cả proto definitions |
| 11 | go.mod | [T11](./T11_gomod-update.md) | Cập nhật go.mod cho tất cả services mới |
| 12 | migrations | [T12](./T12_migrations.md) | Hợp nhất và đánh số lại migrations |
| 13 | docker | [T13](./T13_dockerfile.md) | Tạo/cập nhật Dockerfile cho từng service |
| 14 | cleanup | [T14](./T14_cleanup_archive.md) | Xoá archive/, xoá services cũ |

---

## Nguyên tắc thực hiện

1. **Mỗi task là atomic**: Có thể chạy độc lập sau khi task trước hoàn thành
2. **Không modify shared/**: Chỉ update module name, không thay đổi code
3. **Giữ backward compat**: Proto interfaces phải backward compatible
4. **Test sau mỗi phase**: Build `go build ./...` sau mỗi task
5. **Chỉ xoá archive sau Phase cuối**: Xác nhận build pass trước khi `rm -rf archive/`

---

## Cấu trúc thư mục sau merge

```
services/
├── identity-service/      ← NEW (từ auth-service)
├── data-service/          ← NEW (từ vulnerability-service + ingestion-service)
├── search-service/        ← RENAMED + MERGED
├── scan-service/          ← MERGED (+ schedule-service)
├── finding-service/       ← MERGED (+ product-service + report-service)
├── ai-service/            ← REFACTORED
├── notification-service/  ← MERGED (+ integration-service)
├── gateway-service/       ← RENAMED (từ unified-gateway)
└── shared/                ← UNCHANGED (pkg + proto)
```
