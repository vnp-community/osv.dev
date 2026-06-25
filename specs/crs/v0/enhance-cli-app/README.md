# OSV.dev — CLI & App Enhancement Solutions

> **Mục đích**: Cập nhật `apps/cli` và `apps/osv` để sử dụng microservices mới trong `services/`
> **Ngày tạo**: 2026-06-13
> **Phương pháp**: CHỈ THÊM, KHÔNG XÓA — additive integration pattern

---

## Tổng quan kiến trúc mục tiêu

```
apps/cli  ─────────────────────────────────────────► services/
  cmd/importer   ──NATS──► data-service (ingest pipeline)
  cmd/worker     ──gRPC──► data-service + ai-service
  cmd/exporter   ──REST──► data-service (bulk export)
  cmd/relations  ──gRPC──► data-service (alias group)
  cmd/recordchecker ─REST──► data-service + search-service
  [NEW] cmd/scan    ──gRPC──► scan-service
  [NEW] cmd/query   ──REST──► gateway-service

apps/osv  ─────────────────────────────────────────► services/
  cmd/server   ─ tổng hợp tất cả services (goroutines)
    ├─ gateway-service  (port 8080 HTTP, 9090 gRPC)
    ├─ data-service     (port 8082 HTTP, 50053 gRPC)
    ├─ search-service   (port 8083 HTTP, 50056 gRPC)
    ├─ ai-service       (port 8086 HTTP, 50052 gRPC)
    ├─ finding-service  (port 8085 HTTP, 50060 gRPC)
    ├─ identity-service (port 8081 HTTP, 50051 gRPC)
    ├─ notification-service (port 8084)
    └─ scan-service     (port 8087)
```

## Tài liệu giải pháp

| File | Nội dung |
|------|---------|
| [01_architecture.md](./01_architecture.md) | Kiến trúc tổng thể, service map, ports |
| [02_cli-upgrade.md](./02_cli-upgrade.md) | Nâng cấp `apps/cli` — từng command |
| [03_osv-server-upgrade.md](./03_osv-server-upgrade.md) | Nâng cấp `apps/osv` — service orchestration |
| [04_service-client-layer.md](./04_service-client-layer.md) | Client layer shared — gRPC + REST + NATS |
| [05_grpc-integration.md](./05_grpc-integration.md) | gRPC client wiring cho từng service |
| [06_nats-integration.md](./06_nats-integration.md) | NATS event flow giữa apps và services |
| [07_osv-app-feature-matrix.md](./07_osv-app-feature-matrix.md) | Feature matrix PRD → service mapping |
| [08_implementation-tasks.md](./08_implementation-tasks.md) | Tasks chi tiết để implement |

## Nguyên tắc cốt lõi

1. **GIỮ NGUYÊN code cũ** — CLI commands hiện tại (GCP Datastore, Pub/Sub) tiếp tục hoạt động
2. **THÊM backend mới** — Microservices làm backend thứ hai (configurable via env)
3. **Goroutines độc lập** — Mỗi service chạy trong goroutine riêng trong `apps/osv/cmd/server`
4. **Giao tiếp qua gRPC/REST/NATS** — Không gọi function trực tiếp giữa services
5. **Feature parity** — Tất cả chức năng từ PRD/SRS/URD phải được phục vụ
