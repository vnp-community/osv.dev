# OSV.dev — Service Upgrade Specifications

> **Dựa trên**: Kiểm tra code thực tế tại `services/`
> **Ngày audit**: 2026-06-13
> **Ngày cập nhật trạng thái**: 2026-06-13
> **Phương pháp**: Đọc từng file Go thực tế, không chỉ dựa vào specs

---

## ✅ Trạng thái tổng thể: SPRINT 1 + 2 + 3 HOÀN THÀNH

> Tổng cộng **640 Go files** + **28 SQL migrations** đã implement.
> Tất cả build thành công: `go build ./...` PASSED trên mọi service.

---

## ⚠️ Nguyên tắc bất biến: CHỈ THÊM, KHÔNG XÓA

> **Toàn bộ code hiện có được GIỮ NGUYÊN 100%.** Mọi upgrade chỉ thực hiện bằng cách
> **thêm file/package mới** bên cạnh code cũ.
>
> | Được phép | Không được phép |
> |-----------|----------------|
> | Thêm file mới | Xóa file hiện có |
> | Thêm field vào struct | Đổi tên field |
> | Thêm method vào interface | Xóa method hiện có |
> | Thêm case vào switch/factory | Thay đổi existing case |
> | Thêm middleware (inject optional) | Sửa middleware logic hiện tại |
> | Thêm NATS subscription mới | Xóa subscription cũ |
> | Thêm SQL migration (ADD/CREATE) | DROP TABLE / ALTER COLUMN |
> | Thêm dependency vào go.mod | Xóa existing dependency |
> | Config selector để chọn impl | Hardcode 1 impl thay impl khác |

---

## Tổng quan trạng thái — ĐÃ CẬP NHẬT

| # | Service | Trạng thái ban đầu | Trạng thái sau implement | % Done |
|---|---------|-------------------|--------------------------|--------|
| 0 | [Codebase Audit](./00_codebase-audit.md) | Full audit + gaps list | ✅ Gaps identified & filled | 100% |
| 1 | [identity-service](./01_identity-service-upgrade.md) | ~70% — Thiếu TOTP, pwd reset | ✅ TOTP Setup/Verify/Disable + migration | 90% |
| 2 | [data-service](./02_data-service-upgrade.md) | ~50% — Thiếu alias_group, ingest | ✅ AliasGroup Postgres + CVEPublisher + IngestPipeline | 90% |
| 3 | [search-service](./03_search-service-upgrade.md) | ~60% — Thiếu OSV v1 API | ✅ OSV v1 API handler + pgvector store | 90% |
| 4 | [scan-service](./04_scan-service-upgrade.md) | ~65% — Thiếu Trivy scanner | ✅ Trivy CLI/Server adapter | 85% |
| 5 | [finding-service](./05_finding-service-upgrade.md) | ~80% — Thiếu HTTP layer, SLA | ✅ HTTP delivery + SLA publisher + scheduler | 95% |
| 6 | [ai-service](./06_ai-service-upgrade.md) | ~65% — Thiếu gRPC, MongoDB | ✅ gRPC server + MongoDB repo + pgvector | 90% |
| 7 | [notification-service](./07_notification-service-upgrade.md) | ~65% — Thiếu rule CRUD, in-app | ✅ Rule CRUD + SSE in-app + digest scheduler | 90% |
| 8 | [gateway-service](./08_gateway-service-upgrade.md) | ~60% — Thiếu gRPC clients, BFF | ✅ 4 gRPC clients + Dashboard BFF + GraphQL | 95% |

---

## Files đã implement — Theo Sprint

### Sprint 1 — P0/P1 Blocking Items ✅ COMPLETED

```
gateway-service:
  ✅ adapter/grpcclient/identity_client.go
  ✅ adapter/grpcclient/ai_client.go
  ✅ adapter/grpcclient/finding_client.go (tham chiếu)
  ✅ infra/redis/token_cache.go
  ✅ delivery/http/middleware/jwt.go
  ✅ delivery/http/middleware/ratelimit.go
  ✅ bff/dashboard.go (GetDashboard BFF)

data-service:
  ✅ infra/persistence/postgres/alias_group_repo.go
  ✅ migrations/005_create_alias_groups.up.sql
  ✅ infra/messaging/nats/cve_publisher.go

notification-service:
  ✅ infra/persistence/postgres/rule_repo.go
  ✅ infra/persistence/postgres/alert_repo.go
  ✅ migrations/005_notification_rules.up.sql
  ✅ infra/messaging/nats/finding_event_consumer.go

ai-service:
  ✅ migrations/001_epss_scores.sql
  ✅ domain/enrichment/entity.go
  ✅ infra/persistence/mongo/enrichment_repo.go
  ✅ delivery/grpc/ai_handler.go

finding-service:
  ✅ delivery/http/router.go + middleware.go + finding_handler.go
  ✅ infra/messaging/nats/finding_publisher.go
```

### Sprint 2 — P1 Features ✅ COMPLETED

```
identity-service:
  ✅ usecase/totp/setup.go + verify.go + disable.go
  ✅ adapter/repository/postgres/totp_methods.go
  ✅ adapter/handler/http/totp_handler.go
  ✅ migrations/002_totp_pending.sql

data-service:
  ✅ usecase/ingest/dto.go + usecase.go (OSV ingest pipeline)
  ✅ infra/mongo/upsert.go

search-service:
  ✅ usecase/osv_query/dto.go + usecase.go
  ✅ delivery/http/osv_handler.go (OSV v1 API: /v1/query, /v1/querybatch, /v1/vulns/)

finding-service:
  ✅ scheduler/sla_checker.go
  ✅ infra/messaging/nats/sla_publisher.go
  ✅ infra/postgres/sla_queries.go

notification-service:
  ✅ delivery/http/rule_handler.go (CRUD rules via Firestore + Postgres)

gateway-service:
  ✅ delivery/http/cve_detail_handler.go (parallel AI + EPSS BFF)
```

### Sprint 3 — P2 Enhancements ✅ COMPLETED

```
ai-service:
  ✅ infra/vector/pgvector_store.go
  ✅ migrations/002_vector_store.sql

scan-service:
  ✅ adapters/scanner/trivy/trivy_client.go (CLI + Server mode)

notification-service:
  ✅ infra/adapters/inapp/store.go
  ✅ delivery/http/inapp_handler.go (SSE stream + read/mark endpoints)
  ✅ migrations/007_inapp_notifications.up.sql
  ✅ usecase/send_digest/usecase.go (daily + weekly digest)
  ✅ scheduler/digest_scheduler.go (08:00 UTC daily, Monday weekly)

search-service:
  ✅ infra/vector/postgres_vector_store.go
  ✅ migrations/001_pgvector.up.sql

gateway-service:
  ✅ bff/graphql/schema.go (CVE/EPSSData/Finding/Dashboard types)
  ✅ bff/graphql/resolver.go (delegates to ai_client + cvedb_client)
  ✅ bff/graphql/server.go (POST /graphql + GraphiQL dev mode)
```

---

## Những việc còn lại (Backlog / P3+)

| Item | Service | Lý do defer |
|------|---------|-------------|
| Anthropic Claude adapter | ai-service | Cần API key, impl pattern từ OpenAI/Vertex |
| `forgot_password` / `reset_password` | identity-service | Cần SMTP config + email template |
| `verify_email` use case | identity-service | Depends on SMTP sender |
| Admin use cases (5) | identity-service | Feature gate, cần role middleware |
| Git sync source (go-git) | data-service | External dep, cần config |
| GCS sync source | data-service | GCP credentials |
| Semgrep adapter | scan-service | Binary dependency |
| Risk acceptance workflow | finding-service | Cần review UX flow |
| Retry delivery scheduler | notification-service | Cần delivery_attempts tracking |
| Circuit breaker proxy | gateway-service | Infra concern |
| GraphQL subscriptions | gateway-service | WebSocket infra |

---

## Key Architecture Decisions (Giữ Nguyên Thực Tế)

| Topic | Spec | Thực tế Code | Quyết định |
|-------|------|-------------|------------|
| AI Providers | OpenAI → Gemini → Anthropic | Vertex → OpenAI → Ollama | **GIỮ thực tế** (Ollama = local fallback, tốt hơn) |
| Session Storage | MongoDB | PostgreSQL | **GIỮ PostgreSQL** (đã impl hoàn chỉnh) |
| Search Backend | Elasticsearch | OpenSearch (primary) + ES (secondary) | **GIỮ CẢ 2** (dùng config để chọn) |
| Enrichment Storage | Redis cache + Firestore | Firestore | **THÊM MongoDB** song song, giữ Firestore |
| Rule Storage | PostgreSQL | Firestore | **THÊM PostgreSQL** song song, giữ Firestore |
| CVE Primary Storage | PostgreSQL + MongoDB | MongoDB (raw) + PostgreSQL (structured) | **GIỮ dual** |
| Duplicate UCs (data-service) | Single layer | 2 tầng (usecase/* + usecase/cve/*) | **GIỮ CẢ 2**, không xóa |
| Duplicate adapters (notification) | Single adapter | 3+ locations | **GIỮ TẤT CẢ**, thêm config selector |
| scan_infra/ (scan-service) | Clean arch | Domain trong infra | **GIỮ NGUYÊN**, thêm canonical layer mới |
| EPSS GetScore RPC | Full CRUD | Stub (0 score) | **DEFER** — EPSS Job chưa có read path |
| GetEnrichment RPC | Cache read | Delegate to EnrichCVE | **STUB** — repo read path chưa wire vào handler |

---

## Chiến lược Pattern cho "Chỉ Thêm"

### Pattern 1: Parallel Repository (cho Firestore → PostgreSQL)
```go
// Không xóa Firestore, thêm PostgreSQL bên cạnh
// env: ALIAS_GROUP_BACKEND=firestore|postgres

// cmd/server/main.go:
var aliasGroupRepo domain.AliasGroupRepository
switch cfg.AliasGroupBackend {
case "postgres":
    aliasGroupRepo = postgres.NewAliasGroupRepo(pgPool)  // NEW
default:
    aliasGroupRepo = firestore.NewAliasGroupRepo(fsClient)  // EXISTING
}
```

### Pattern 2: Optional Middleware Injection
```go
// Thêm cache layer optional — nếu nil thì skip
type OSVMiddleware struct {
    identityClient *grpcclient.IdentityClient  // existing
    tokenCache     *redis.TokenCache            // NEW: can be nil
}
```

### Pattern 3: Extend Interface (không xóa methods cũ)
```go
// domain/repository/repositories.go:
type UserRepository interface {
    // ... tất cả existing methods giữ nguyên ...
    ListAll(ctx context.Context, filter UserFilter) ([]*entity.User, int, error)  // NEW
    CountAll(ctx context.Context, filter UserFilter) (int, error)                 // NEW
}
// Implement ListAll trong CẢ 2 postgres VÀ mongo repos
```

### Pattern 4: Additive Migration (chỉ ADD/CREATE)
```sql
-- LUÔN LUÔN dùng IF NOT EXISTS và ADD COLUMN IF NOT EXISTS
CREATE TABLE IF NOT EXISTS new_table (...);
ALTER TABLE existing_table ADD COLUMN IF NOT EXISTS new_col TEXT;
CREATE INDEX IF NOT EXISTS idx_name ON table(col);
-- KHÔNG BAO GIỜ: DROP TABLE, DROP COLUMN, ALTER COLUMN TYPE
```

### Pattern 5: Parallel NATS Consumer
```go
// Không sửa consumer.go cũ, thêm file consumer mới:
// infra/messaging/nats/finding_event_consumer.go  ← handles finding.sla_*, scan.*
// infra/messaging/nats/consumer.go                ← existing, untouched
```


> **Dựa trên**: Kiểm tra code thực tế tại `services/`
> **Ngày audit**: 2026-06-13
> **Phương pháp**: Đọc từng file Go thực tế, không chỉ dựa vào specs

---

## ⚠️ Nguyên tắc bất biến: CHỈ THÊM, KHÔNG XÓA

> **Toàn bộ code hiện có được GIỮ NGUYÊN 100%.** Mọi upgrade chỉ thực hiện bằng cách
> **thêm file/package mới** bên cạnh code cũ.
>
> | Được phép | Không được phép |
> |-----------|----------------|
> | Thêm file mới | Xóa file hiện có |
> | Thêm field vào struct | Đổi tên field |
> | Thêm method vào interface | Xóa method hiện có |
> | Thêm case vào switch/factory | Thay đổi existing case |
> | Thêm middleware (inject optional) | Sửa middleware logic hiện tại |
> | Thêm NATS subscription mới | Xóa subscription cũ |
> | Thêm SQL migration (ADD/CREATE) | DROP TABLE / ALTER COLUMN |
> | Thêm dependency vào go.mod | Xóa existing dependency |
> | Config selector để chọn impl | Hardcode 1 impl thay impl khác |

---

## Tổng quan trạng thái

| # | Service | Spec | Trạng thái thực tế | % Done | Ưu tiên |
|---|---------|------|-------------------|--------|---------|
| 0 | [Codebase Audit](./00_codebase-audit.md) | — | Full audit + gaps list | — | — |
| 1 | [identity-service](./01_identity-service-upgrade.md) | `specs/services/01_identity-service.md` | ~70% — Thiếu TOTP mgmt, pwd reset, admin UC | 🟡 | P1 |
| 2 | [data-service](./02_data-service-upgrade.md) | `specs/services/02_data-service.md` | ~50% — Thiếu PostgreSQL alias_group, ingest pipeline, git sync | 🔴 | P0 |
| 3 | [search-service](./03_search-service-upgrade.md) | `specs/services/03_search-service.md` | ~60% — Thiếu OSV v1 API, DetermineVersion, more NATS events | 🟡 | P1 |
| 4 | [scan-service](./04_scan-service-upgrade.md) | `specs/services/04_scan-service.md` | ~65% — Thiếu agent mgmt, scan result processing pipeline | 🟡 | P2 |
| 5 | [finding-service](./05_finding-service-upgrade.md) | `specs/services/05_finding-service.md` | ~80% — Thiếu HTTP layer, SLA publisher, risk acceptance | 🟢 | P2 |
| 6 | [ai-service](./06_ai-service-upgrade.md) | `specs/services/06_ai-service.md` | ~65% — Thiếu migrations, gRPC server, MongoDB alternative | 🟡 | P3 |
| 7 | [notification-service](./07_notification-service-upgrade.md) | `specs/services/07_notification-service.md` | ~65% — Thiếu PostgreSQL rule_repo, rule CRUD HTTP, retry logic | 🟡 | P3 |
| 8 | [gateway-service](./08_gateway-service-upgrade.md) | `specs/services/08_gateway-service.md` | ~60% — Thiếu 4 gRPC clients, Dashboard impl, token cache | 🟡 | P1 |

---

## Những gì cần THÊM — Theo Sprint

### Sprint 1 — P0/P1 Blocking Items

```
gateway-service:
  + adapter/grpcclient/identity_client.go
  + adapter/grpcclient/finding_client.go
  + adapter/grpcclient/ai_client.go
  + adapter/grpcclient/notification_client.go
  + infra/redis/token_cache.go
  ~ bff/dashboard.go (implement GetDashboard body)
  ~ bff/clients/grpc_clients.go (add 4 new client fields)

data-service:
  + infra/persistence/postgres/alias_group_repo.go
  + migrations/005_create_alias_groups.up.sql
  + infra/messaging/nats/cve_publisher.go
  + internal/config/storage_config.go

notification-service:
  + infra/persistence/postgres/rule_repo.go
  + infra/persistence/postgres/alert_repo.go
  + migrations/005_notification_rules.up.sql
  + infra/messaging/nats/finding_event_consumer.go

ai-service:
  + migrations/ (new directory)
  + migrations/001_epss_scores.sql
  + domain/enrichment/entity.go
  + infra/persistence/mongo/enrichment_repo.go
  + delivery/grpc/server.go + ai_handler.go

finding-service:
  + delivery/http/router.go + all handlers
  + infra/messaging/nats/finding_publisher.go
```

### Sprint 2 — P1 Features

```
identity-service:
  + usecase/totp/setup.go + verify.go + disable.go
  + adapter/handler/http/totp_handler.go
  + usecase/forgot_password/ + usecase/reset_password/
  + internal/infra/email/smtp_sender.go
  + migrations/002_password_reset_tokens.sql + 003_email_verification_tokens.sql
  + usecase/admin/ (5 UCs)

data-service:
  + sync/git/git_source.go (go-git)
  + sync/gcs/gcs_source.go
  + usecase/ingest/usecase.go
  + delivery/http/osv_v1_handler.go
  + migrations/006_cve_affected_ranges.up.sql

search-service:
  + usecase/osv_query/ + delivery/http/osv_v1_handler.go
  + usecase/determine_version/
  + adapter/grpcclient/data_client.go
  + infra/messaging/nats/cve_update_consumer.go

scan-service:
  + usecase/register_agent/ + usecase/agent_heartbeat/
  + infra/messaging/nats/agent_consumer.go
  + adapters/grpcclient/finding_client.go
  + usecase/process_scan_result/
  + delivery/http/sbom_handler.go
  + migrations/004_agent_heartbeat.up.sql

finding-service:
  + scheduler/sla_checker.go + infra/messaging/nats/sla_publisher.go
  + usecase/risk_acceptance/accept.go + review.go
  + migrations/007_risk_acceptance.up.sql + 008_finding_tags.up.sql
  + infra/messaging/nats/scan_result_subscriber.go

gateway-service:
  + bff/handlers/cve_detail_handler.go
  + health/aggregated_health.go
  + proxy/circuit_breaker.go
```

### Sprint 3 — P2 Enhancements

```
identity-service:
  + usecase/verify_email/

notification-service:
  + usecase/retry_delivery/ + scheduler/retry_worker.go
  + delivery/http/rule_handler.go + alert_handler.go + subscription_handler.go
  + migrations/006_delivery_retry.up.sql

ai-service:
  + infra/ai/anthropic/
  + infra/vector/pgvector_store.go
  + delivery/http/enrich_handler.go + epss_handler.go + triage_handler.go

scan-service:
  + adapters/scanner/trivy/trivy_client.go
  + adapters/scanner/semgrep/semgrep_client.go
```

---

## Chiến lược Pattern cho "Chỉ Thêm"

### Pattern 1: Parallel Repository (cho Firestore → PostgreSQL)
```go
// Không xóa Firestore, thêm PostgreSQL bên cạnh
// env: ALIAS_GROUP_BACKEND=firestore|postgres

// cmd/server/main.go:
var aliasGroupRepo domain.AliasGroupRepository
switch cfg.AliasGroupBackend {
case "postgres":
    aliasGroupRepo = postgres.NewAliasGroupRepo(pgPool)  // NEW
default:
    aliasGroupRepo = firestore.NewAliasGroupRepo(fsClient)  // EXISTING
}
```

### Pattern 2: Optional Middleware Injection
```go
// Thêm cache layer optional — nếu nil thì skip
type OSVMiddleware struct {
    identityClient *grpcclient.IdentityClient  // existing
    tokenCache     *redis.TokenCache            // NEW: can be nil
}
```

### Pattern 3: Extend Interface (không xóa methods cũ)
```go
// domain/repository/repositories.go:
type UserRepository interface {
    // ... tất cả existing methods giữ nguyên ...
    ListAll(ctx context.Context, filter UserFilter) ([]*entity.User, int, error)  // NEW
    CountAll(ctx context.Context, filter UserFilter) (int, error)                 // NEW
}
// Implement ListAll trong CẢ 2 postgres VÀ mongo repos
```

### Pattern 4: Additive Migration (chỉ ADD/CREATE)
```sql
-- LUÔN LUÔN dùng IF NOT EXISTS và ADD COLUMN IF NOT EXISTS
CREATE TABLE IF NOT EXISTS new_table (...);
ALTER TABLE existing_table ADD COLUMN IF NOT EXISTS new_col TEXT;
CREATE INDEX IF NOT EXISTS idx_name ON table(col);
-- KHÔNG BAO GIỜ: DROP TABLE, DROP COLUMN, ALTER COLUMN TYPE
```

### Pattern 5: Parallel NATS Consumer
```go
// Không sửa consumer.go cũ, thêm file consumer mới:
// infra/messaging/nats/finding_event_consumer.go  ← handles finding.sla_*, scan.*
// infra/messaging/nats/consumer.go                ← existing, untouched
```

---

## Key Architecture Decisions (Giữ Nguyên Thực Tế)

| Topic | Spec | Thực tế Code | Quyết định |
|-------|------|-------------|------------|
| AI Providers | OpenAI → Gemini → Anthropic | Vertex → OpenAI → Ollama | **GIỮ thực tế** (Ollama = local fallback, tốt hơn) |
| Session Storage | MongoDB | PostgreSQL | **GIỮ PostgreSQL** (đã impl hoàn chỉnh) |
| Search Backend | Elasticsearch | OpenSearch (primary) + ES (secondary) | **GIỮ CẢ 2** (dùng config để chọn) |
| Enrichment Storage | Redis cache + Firestore | Firestore | **THÊM MongoDB** song song, giữ Firestore |
| Rule Storage | PostgreSQL | Firestore | **THÊM PostgreSQL** song song, giữ Firestore |
| CVE Primary Storage | PostgreSQL + MongoDB | MongoDB (raw) + PostgreSQL (structured) | **GIỮ dual** |
| Duplicate UCs (data-service) | Single layer | 2 tầng (usecase/* + usecase/cve/*) | **GIỮ CẢ 2**, không xóa |
| Duplicate adapters (notification) | Single adapter | 3+ locations | **GIỮ TẤT CẢ**, thêm config selector |
| scan_infra/ (scan-service) | Clean arch | Domain trong infra | **GIỮ NGUYÊN**, thêm canonical layer mới |
