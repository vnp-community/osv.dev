# DefectDojo Monolithic Go Application — Solution Overview

## Mục tiêu

Xây dựng **một ứng dụng monolithic** bằng Go tái sử dụng toàn bộ code base tại `services/` (không thay đổi source code của các service). Ứng dụng thực hiện đầy đủ chức năng của **Django DefectDojo** — nền tảng quản lý bảo mật ứng dụng (Application Security Management).

## Chiến lược

| Phương pháp | Mô tả |
|---|---|
| **Monolithic Binary** | Một binary duy nhất (`defectdojo-go`) chứa tất cả |
| **Goroutine Isolation** | Mỗi service chạy trên goroutine độc lập với lifecycle riêng |
| **In-Process gRPC** | Các service giao tiếp qua gRPC in-process (bufconn) hoặc TCP gRPC |
| **NATS JetStream** | Event-driven communication cho async workflows |
| **REST Gateway** | HTTP/REST API tương thích với DefectDojo API v2 |
| **Code Reuse** | Import trực tiếp các package Go, không fork/copy code |

## Các Service Được Tích Hợp

```
services/
├── auth-service          → Authentication & JWT (port: 50051)
├── ai-service            → AI Analysis (port: 50052)
├── impact-service        → Impact Assessment (port: 50053)
├── integration-service   → JIRA/GitHub integrations (port: 50054)
├── vulnerability-service → CVE/Vuln Database (port: 50055)
├── search-service        → OpenSearch indexing (port: 50056)
├── query-service         → Vulnerability queries (port: 50057)
├── scan-service          → Scan orchestration (port: 50058)
├── finding-service       → Finding CRUD & SLA (port: 50060)
├── product-service       → Products & Engagements (port: 50061)
├── notification-service  → Alerts & Webhooks (port: 50063)
├── report-service        → Report generation (port: 50065)
├── ingestion-service     → Data ingestion (NATS)
└── unified-gateway       → HTTP API Gateway (port: 8080/9090)
```

## DefectDojo Capabilities được implement

| Chức năng DefectDojo | Service Go | Giao tiếp |
|---|---|---|
| Quản lý Product/ProductType | product-service | gRPC |
| Quản lý Engagement | product-service | gRPC |
| Quản lý Test | scan-service | gRPC |
| Import Scan Results | ingestion-service | NATS |
| Finding Management | finding-service | gRPC |
| Deduplication | finding-service | gRPC (batch) |
| SLA Tracking | finding-service | gRPC |
| Risk Acceptance | finding-service | gRPC |
| Notification/Alerts | notification-service | NATS + gRPC |
| Report Generation | report-service | gRPC stream |
| JIRA Integration | integration-service | NATS |
| Authentication/RBAC | auth-service | gRPC |
| Search | search-service | gRPC |
| CVE Lookup | vulnerability-service | gRPC |
| AI Triage | ai-service | NATS |
| Impact Analysis | impact-service | NATS |

## Cấu trúc Tài liệu

| File | Nội dung |
|---|---|
| [00-overview.md](./00-overview.md) | Overview (file này) |
| [01-architecture.md](./01-architecture.md) | Kiến trúc monolith, goroutine topology |
| [02-service-registry.md](./02-service-registry.md) | Service registry & lifecycle management |
| [03-communication-protocol.md](./03-communication-protocol.md) | gRPC, NATS, REST patterns |
| [04-defectdojo-feature-mapping.md](./04-defectdojo-feature-mapping.md) | Mapping đầy đủ DD features → Go services |
| [05-data-model.md](./05-data-model.md) | Data model & database schema |
| [06-api-gateway.md](./06-api-gateway.md) | REST API compatible với DefectDojo v2 |
| [07-implementation-guide.md](./07-implementation-guide.md) | Hướng dẫn implement step-by-step |
| [08-config-and-deployment.md](./08-config-and-deployment.md) | Cấu hình & triển khai |

## Nguyên tắc Thiết kế

1. **Zero Code Change**: Code trong `services/` được import nguyên vẹn
2. **Goroutine Isolation**: Mỗi service có goroutine chính + goroutines con riêng
3. **Graceful Shutdown**: context.Context propagation toàn bộ hệ thống
4. **Observability**: Structured logging (zerolog), metrics, tracing
5. **Backward Compatibility**: REST API tương thích 100% với Django DefectDojo v2
