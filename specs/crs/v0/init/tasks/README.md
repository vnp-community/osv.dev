# Task Index — AI Execution Guide

> **Gốc từ**: `/specs/develop/` upgrade specifications
> **Nguyên tắc**: Chỉ THÊM file mới, KHÔNG XÓA hay MERGE code hiện có
> **Cấu trúc**: Mỗi file task = 1 unit công việc AI thực hiện độc lập

---

## Cách đọc task file

Mỗi task file có cấu trúc:
```
## Metadata         ← ID, service, ước tính thời gian, dependency
## Context          ← File cần đọc trước khi làm
## Goal             ← Mục tiêu cụ thể
## Steps            ← Các bước thực hiện tuần tự
## Files to Create  ← Code đầy đủ cho từng file mới
## Files to Extend  ← Chỉ thêm đoạn code vào file cũ
## Verification     ← Cách kiểm tra đã làm đúng
```

---

## Sprint 1 — P0 (Blocking, làm trước)

| Task ID | Service | Mô tả | File |
|---------|---------|-------|------|
| S1-GW-01 | gateway | Thêm 4 gRPC clients còn thiếu | [sprint1/S1-GW-01_grpc-clients.md](./sprint1/S1-GW-01_grpc-clients.md) |
| S1-GW-02 | gateway | Implement Dashboard BFF | [sprint1/S1-GW-02_dashboard-bff.md](./sprint1/S1-GW-02_dashboard-bff.md) |
| S1-GW-03 | gateway | Thêm Redis token cache | [sprint1/S1-GW-03_token-cache.md](./sprint1/S1-GW-03_token-cache.md) |
| S1-DATA-01 | data | Thêm PostgreSQL AliasGroup repo | [sprint1/S1-DATA-01_alias-group-postgres.md](./sprint1/S1-DATA-01_alias-group-postgres.md) |
| S1-DATA-02 | data | Thêm NATS CVE event publisher | [sprint1/S1-DATA-02_cve-event-publisher.md](./sprint1/S1-DATA-02_cve-event-publisher.md) |
| S1-FIND-01 | finding | Thêm HTTP delivery layer | [sprint1/S1-FIND-01_http-delivery.md](./sprint1/S1-FIND-01_http-delivery.md) |
| S1-FIND-02 | finding | Thêm NATS event publisher | [sprint1/S1-FIND-02_nats-publisher.md](./sprint1/S1-FIND-02_nats-publisher.md) |
| S1-AI-01 | ai | Tạo migrations + EnrichmentResult entity | [sprint1/S1-AI-01_migrations-entity.md](./sprint1/S1-AI-01_migrations-entity.md) |
| S1-AI-02 | ai | Thêm gRPC server | [sprint1/S1-AI-02_grpc-server.md](./sprint1/S1-AI-02_grpc-server.md) |
| S1-AI-03 | ai | Thêm MongoDB enrichment repo | [sprint1/S1-AI-03_mongo-enrichment-repo.md](./sprint1/S1-AI-03_mongo-enrichment-repo.md) |
| S1-NOTIF-01 | notification | Thêm PostgreSQL rule/alert repos | [sprint1/S1-NOTIF-01_postgres-repos.md](./sprint1/S1-NOTIF-01_postgres-repos.md) |
| S1-NOTIF-02 | notification | Thêm NATS finding event consumer | [sprint1/S1-NOTIF-02_finding-consumer.md](./sprint1/S1-NOTIF-02_finding-consumer.md) |

---

## Sprint 2 — P1 (Core Features)

| Task ID | Service | Mô tả | File |
|---------|---------|-------|------|
| S2-ID-01 | identity | Thêm TOTP management UC + Handler | [sprint2/S2-ID-01_totp-management.md](./sprint2/S2-ID-01_totp-management.md) |
| S2-ID-02 | identity | Thêm Password Reset flow | [sprint2/S2-ID-02_password-reset.md](./sprint2/S2-ID-02_password-reset.md) |
| S2-ID-03 | identity | Thêm Email Verification flow | [sprint2/S2-ID-03_email-verification.md](./sprint2/S2-ID-03_email-verification.md) |
| S2-ID-04 | identity | Thêm Admin use cases + handler | [sprint2/S2-ID-04_admin-endpoints.md](./sprint2/S2-ID-04_admin-endpoints.md) |
| S2-ID-05 | identity | Thêm NATS event publisher | [sprint2/S2-ID-05_nats-publisher.md](./sprint2/S2-ID-05_nats-publisher.md) |
| S2-DATA-01 | data | Thêm Git source sync | [sprint2/S2-DATA-01_git-sync.md](./sprint2/S2-DATA-01_git-sync.md) |
| S2-DATA-02 | data | Thêm GCS bucket sync | [sprint2/S2-DATA-02_gcs-sync.md](./sprint2/S2-DATA-02_gcs-sync.md) |
| S2-DATA-03 | data | Thêm OSV ingest pipeline | [sprint2/S2-DATA-03_ingest-pipeline.md](./sprint2/S2-DATA-03_ingest-pipeline.md) |
| S2-DATA-04 | data | Thêm OSV v1 API endpoints | [sprint2/S2-DATA-04_osv-v1-api.md](./sprint2/S2-DATA-04_osv-v1-api.md) |
| S2-SEARCH-01 | search | Thêm OSV v1 Query API | [sprint2/S2-SEARCH-01_osv-query-api.md](./sprint2/S2-SEARCH-01_osv-query-api.md) |
| S2-SEARCH-02 | search | Thêm DetermineVersion endpoint | [sprint2/S2-SEARCH-02_determine-version.md](./sprint2/S2-SEARCH-02_determine-version.md) |
| S2-SEARCH-03 | search | Thêm NATS update/withdrawn consumer | [sprint2/S2-SEARCH-03_nats-consumers.md](./sprint2/S2-SEARCH-03_nats-consumers.md) |
| S2-SCAN-01 | scan | Thêm Agent management UCs | [sprint2/S2-SCAN-01_agent-management.md](./sprint2/S2-SCAN-01_agent-management.md) |
| S2-SCAN-02 | scan | Thêm Scan result processing | [sprint2/S2-SCAN-02_scan-result-processing.md](./sprint2/S2-SCAN-02_scan-result-processing.md) |
| S2-SCAN-03 | scan | Thêm SBOM ingest endpoint | [sprint2/S2-SCAN-03_sbom-endpoint.md](./sprint2/S2-SCAN-03_sbom-endpoint.md) |
| S2-FIND-01 | finding | Thêm SLA breach publisher + cron | [sprint2/S2-FIND-01_sla-publisher.md](./sprint2/S2-FIND-01_sla-publisher.md) |
| S2-FIND-02 | finding | Thêm Risk Acceptance UC | [sprint2/S2-FIND-02_risk-acceptance.md](./sprint2/S2-FIND-02_risk-acceptance.md) |
| S2-FIND-03 | finding | Thêm NATS subscribers | [sprint2/S2-FIND-03_nats-subscribers.md](./sprint2/S2-FIND-03_nats-subscribers.md) |
| S2-GW-01 | gateway | Thêm CVE Detail BFF | [sprint2/S2-GW-01_cve-detail-bff.md](./sprint2/S2-GW-01_cve-detail-bff.md) |
| S2-GW-02 | gateway | Thêm Aggregated Health check | [sprint2/S2-GW-02_health-aggregation.md](./sprint2/S2-GW-02_health-aggregation.md) |
| S2-GW-03 | gateway | Thêm Circuit Breaker | [sprint2/S2-GW-03_circuit-breaker.md](./sprint2/S2-GW-03_circuit-breaker.md) |
| S2-NOTIF-01 | notification | Thêm Rule management HTTP | [sprint2/S2-NOTIF-01_rule-http.md](./sprint2/S2-NOTIF-01_rule-http.md) |
| S2-NOTIF-02 | notification | Thêm Retry delivery logic | [sprint2/S2-NOTIF-02_retry-delivery.md](./sprint2/S2-NOTIF-02_retry-delivery.md) |
| S2-AI-01 | ai | Thêm complete HTTP handlers | [sprint2/S2-AI-01_http-handlers.md](./sprint2/S2-AI-01_http-handlers.md) |
| S2-AI-02 | ai | Thêm NATS consumers/publishers | [sprint2/S2-AI-02_nats-events.md](./sprint2/S2-AI-02_nats-events.md) |

---

## Sprint 3 — P2 (Enhancements)

| Task ID | Service | Mô tả | File |
|---------|---------|-------|------|
| S3-ID-01 | identity | Email verification flow | [sprint3/S3-ID-01_email-verify.md](./sprint3/S3-ID-01_email-verify.md) |
| S3-AI-01 | ai | Anthropic Claude provider | [sprint3/S3-AI-01_anthropic-provider.md](./sprint3/S3-AI-01_anthropic-provider.md) |
| S3-AI-02 | ai | Vector storage (pgvector) | [sprint3/S3-AI-02_vector-storage.md](./sprint3/S3-AI-02_vector-storage.md) |
| S3-SCAN-01 | scan | Trivy scanner adapter | [sprint3/S3-SCAN-01_trivy-adapter.md](./sprint3/S3-SCAN-01_trivy-adapter.md) |
| S3-NOTIF-01 | notification | In-app notifications (SSE) | [sprint3/S3-NOTIF-01_inapp-notifications.md](./sprint3/S3-NOTIF-01_inapp-notifications.md) |
| S3-NOTIF-02 | notification | Digest mode | [sprint3/S3-NOTIF-02_digest-mode.md](./sprint3/S3-NOTIF-02_digest-mode.md) |
| S3-GW-01 | gateway | GraphQL BFF | [sprint3/S3-GW-01_graphql-bff.md](./sprint3/S3-GW-01_graphql-bff.md) |
| S3-SEARCH-01 | search | pgvector semantic search | [sprint3/S3-SEARCH-01_pgvector.md](./sprint3/S3-SEARCH-01_pgvector.md) |

---

## Dependency Graph (Sprint 1)

```
S1-DATA-01 (alias_group postgres) ← độc lập
S1-DATA-02 (cve event publisher)  ← độc lập
S1-AI-01   (migrations + entity)  ← độc lập
S1-AI-03   (mongo enrichment repo) ← cần S1-AI-01
S1-AI-02   (grpc server)          ← cần S1-AI-01 + S1-AI-03
S1-NOTIF-01 (postgres repos)      ← độc lập
S1-NOTIF-02 (finding consumer)    ← cần S1-NOTIF-01
S1-FIND-01  (http delivery)       ← độc lập
S1-FIND-02  (nats publisher)      ← độc lập
S1-GW-01   (grpc clients)         ← độc lập
S1-GW-02   (dashboard bff)        ← cần S1-GW-01 + S1-FIND-01 + S1-AI-02
S1-GW-03   (token cache)          ← cần S1-GW-01
```
