# OSV.dev Microservices — Task Index

> Mỗi task file là self-contained: AI đọc 1 file → thực thi ngay, không cần đọc thêm.

## Quy Tắc Chung (Áp Dụng Mọi Task)

**Stack:** Go 1.22, Clean Architecture, gRPC + Protobuf, NATS JetStream, Redis, Firestore, GCS  
**Layout chuẩn mọi service:**
```
services/{name}/
  cmd/server/main.go
  internal/domain/{entity,valueobject,aggregate,service,repository}/
  internal/application/{command,query,port}/
  internal/infra/{persistence,messaging,cache,storage}/
  interface/{grpc,http}/{handler,middleware,proto}/
  config/config.go  |  Dockerfile  |  go.mod
```
**Mọi service PHẢI có:**
- Structured logging (zerolog), Prometheus metrics, OpenTelemetry tracing
- `/health/live` + `/health/ready` HTTP endpoints
- gRPC: `grpc.health.v1.Health`
- Circuit breaker + retry w/ exponential backoff cho external calls

## Danh Sách Tasks (theo thứ tự ưu tiên)

| File | Mô tả | Priority | Phase |
|------|-------|----------|-------|
| [T00-shared-libs.md](./T00-shared-libs.md) | Shared libraries (`pkg/`) | P0 | Pre |
| [T01-api-gateway.md](./T01-api-gateway.md) | API Gateway service | P0 | 2 |
| [T02-vulnerability-query.md](./T02-vulnerability-query.md) | Vulnerability Query Service | P0 | 2 |
| [T03-ingestion.md](./T03-ingestion.md) | Ingestion Service | P0 | 1 |
| [T04-source-sync.md](./T04-source-sync.md) | Source Sync Service | P0 | 1 |
| [T05-impact-analysis.md](./T05-impact-analysis.md) | Impact Analysis Service | P1 | 3 |
| [T06-version-index.md](./T06-version-index.md) | Version Index Service | P1 | 3 |
| [T07-search.md](./T07-search.md) | Search Service | P1 | 3 |
| [T08-web-bff.md](./T08-web-bff.md) | Web BFF | P1 | 3 |
| [T09-alias-relations.md](./T09-alias-relations.md) | Alias & Relations Service | P1 | 3 |
| [T10-notification.md](./T10-notification.md) | Notification Service | P2 | 3 |
| [T11-ai-enrichment.md](./T11-ai-enrichment.md) | AI Enrichment Service | P2 | 3 |
| [T12-infrastructure.md](./T12-infrastructure.md) | Infrastructure (K8s, NATS, Redis, OpenSearch) | P0 | 0 |
| [T13-migration.md](./T13-migration.md) | Migration Strategy Execution | P0 | All |

## Thứ Tự Thực Thi Khuyến Nghị

```
Phase 0: T12-infrastructure (deploy infra)
Phase Pre: T00-shared-libs (build pkg/)
Phase 1: T03-ingestion + T04-source-sync (data pipeline)
Phase 2: T01-api-gateway + T02-vulnerability-query (serve traffic)
Phase 3: T05..T11 (enrichment + features)
Migration: T13 (chạy song song với mọi phase)
```
