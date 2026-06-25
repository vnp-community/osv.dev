# CR-INIT-000 — Init Account: Tổng quan & Danh sách Change Requests

## Mục tiêu

Khởi tạo toàn bộ giá trị cần thiết để các services trong hệ thống OSV.dev có thể **chạy ngay sau khi deploy** mà người dùng có thể sử dụng được luôn (admin account, secrets, DB schema, cấu hình ban đầu). Tất cả thông tin cấu hình được đọc từ file `.env`.

## Phạm vi

| CR | Nội dung | Service liên quan |
|----|----------|-------------------|
| [CR-INIT-001](./CR-INIT-001-env-template.md) | File `.env` mẫu đầy đủ cho toàn hệ thống | Tất cả |
| [CR-INIT-002](./CR-INIT-002-identity-service.md) | Khởi tạo identity-service: JWT keys, DB schema, admin account | identity-service |
| [CR-INIT-003](./CR-INIT-003-data-service.md) | Khởi tạo data-service: DB schema, cấu hình storage backend | data-service |
| [CR-INIT-004](./CR-INIT-004-search-service.md) | Khởi tạo search-service: Redis connection, OpenSearch index | search-service |
| [CR-INIT-005](./CR-INIT-005-ranking-service.md) | Khởi tạo ranking-service: MongoDB indexes | ranking-service |
| [CR-INIT-006](./CR-INIT-006-notification-service.md) | Khởi tạo notification-service: DB schema, NATS subjects | notification-service |
| [CR-INIT-007](./CR-INIT-007-ai-service.md) | Khởi tạo ai-service: cấu hình LLM backend | ai-service |
| [CR-INIT-008](./CR-INIT-008-gateway-osv-app.md) | Khởi tạo gateway-service & OSV app (apps/osv): upstream addresses, JWT config | gateway-service, apps/osv |
| [CR-INIT-009](./CR-INIT-009-bootstrap-script.md) | Script bootstrap tổng hợp chạy toàn bộ init một lần | Tất cả |

## Kiến trúc hạ tầng

```
┌─────────────────────────────────────────────────────────┐
│                    Infrastructure                       │
│  PostgreSQL:5432  Redis:6379  MongoDB:27017  NATS:4222  │
│  OpenSearch:9200  OTLP Collector:4318                   │
└─────────────────────────────────────────────────────────┘
          │            │            │
┌─────────┼────────────┼────────────┼───────────────────┐
│         ▼            ▼            ▼                   │
│  identity-service  data-service  ranking-service      │
│  (port 9101/9001)  (port 8080/50053)  (port 8088)    │
│                                                       │
│  search-service    notification-service  ai-service   │
│  (port 8082/50056) (port 8086/50063)    (port 50052) │
│                                                       │
│  gateway-service   apps/osv (gateway app)            │
│  (port 8080)       (port 8080)                       │
└───────────────────────────────────────────────────────┘
```

## Nguyên tắc

1. **Idempotent** — Mỗi script/migration có thể chạy lại nhiều lần mà không gây lỗi (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`).
2. **Config từ `.env`** — Không hardcode secret trong code, tất cả đọc từ biến môi trường.
3. **Admin first** — Sau khi bootstrap, admin account tồn tại và có thể đăng nhập ngay.
4. **Health check** — Mỗi service expose `/health` endpoint để xác nhận đã sẵn sàng.

## Thứ tự thực thi

```
1. Cài đặt hạ tầng (PostgreSQL, Redis, MongoDB, NATS, OpenSearch)
2. CR-INIT-001: Tạo file .env từ template
3. CR-INIT-002: identity-service (DB + JWT keys + admin account)
4. CR-INIT-003: data-service (DB schema)
5. CR-INIT-004: search-service (Redis + OpenSearch index)
6. CR-INIT-005: ranking-service (MongoDB indexes)
7. CR-INIT-006: notification-service (DB schema + NATS)
8. CR-INIT-007: ai-service (LLM backend config)
9. CR-INIT-008: gateway + OSV app (routing config)
10. CR-INIT-009: Verify toàn hệ thống bằng script bootstrap
```
