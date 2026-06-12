# Tasks Index — DefectDojo Go Monolith

## Tổng quan

Danh sách đầy đủ các tác vụ thực thi được chia thành **7 Phase** và **~40 task** cụ thể.

## Quick Reference

| Phase | Task File | Mô tả | Ước tính |
|---|---|---|---|
| 0 | [TASK-00-prerequisites.md](./TASK-00-prerequisites.md) | Kiểm tra môi trường, codebase audit | 2h |
| 1 | [TASK-01-project-setup.md](./TASK-01-project-setup.md) | Go module, workspace, cấu trúc thư mục | 3h |
| 2 | [TASK-02-config-infra.md](./TASK-02-config-infra.md) | Config, infra connections, migrations | 4h |
| 3 | [TASK-03-service-registry.md](./TASK-03-service-registry.md) | ServiceRunner interface, Registry, lifecycle | 6h |
| 4 | [TASK-04-service-runners.md](./TASK-04-service-runners.md) | 13 goroutine runners (auth→gateway) | 32h |
| 5 | [TASK-05-api-gateway.md](./TASK-05-api-gateway.md) | HTTP router, handlers, DD v2 API compat | 24h |
| 6 | [TASK-06-events-nats.md](./TASK-06-events-nats.md) | NATS JetStream, pub/sub, event flows | 8h |
| 7 | [TASK-07-testing.md](./TASK-07-testing.md) | Unit tests, integration tests, e2e | 16h |
| 8 | [TASK-08-deployment.md](./TASK-08-deployment.md) | Docker, Compose, K8s, monitoring | 8h |

**Tổng ước tính**: ~103 giờ (~13 ngày làm việc)

## Dependency Order

```
TASK-00 → TASK-01 → TASK-02 → TASK-03
                                  │
                    ┌─────────────┘
                    ↓
                TASK-04 (13 runners, sequential)
                    │
              ┌─────┼─────┐
              ↓     ↓     ↓
          TASK-05 TASK-06 TASK-07
              │
              ↓
          TASK-08
```

## Nguyên tắc thực thi

> ⚠️ **QUAN TRỌNG**: Không được thay đổi bất kỳ file nào trong `services/`.
> Chỉ tạo file mới trong `apps/DefectDojo/`.

## Status Tracking

| Phase | Status | Notes |
|---|---|---|
| TASK-00 | `[ ] pending` | |
| TASK-01 | `[ ] pending` | |
| TASK-02 | `[ ] pending` | |
| TASK-03 | `[ ] pending` | |
| TASK-04 | `[ ] pending` | |
| TASK-05 | `[ ] pending` | |
| TASK-06 | `[ ] pending` | |
| TASK-07 | `[ ] pending` | |
| TASK-08 | `[ ] pending` | |
