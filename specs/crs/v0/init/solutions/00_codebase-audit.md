# Codebase Audit — Trạng thái thực tế của 8 Services

> **Kiểm tra tại**: `/Users/binhnt/Lab/sec/cve/osv.dev/services/`
> **Ngày audit**: 2026-06-13
> **Mục đích**: Xác định chính xác những gì đã implement, những gì còn thiếu, những gì cần refactor

---

## ✅ Implementation Status — 2026-06-13

> **Kết quả**: Tất cả gaps đã được identify và fill trong Sprint 1/2/3.
> **Tổng số files**: 640 Go files + 28 SQL migrations, tất cả `go build` PASSED.

| Service | Gaps ban đầu | Trạng thái |
|---------|-------------|-----------|
| identity-service | TOTP mgmt, pwd reset | ✅ TOTP 3 flows + migration |
| data-service | alias_group postgres, ingest | ✅ AliasGroup + CVEPublisher + IngestUseCase |
| search-service | OSV v1 API, pgvector | ✅ OSV handler + pgvector store |
| scan-service | Trivy adapter, agent mgmt | ✅ Trivy CLI/Server adapter |
| finding-service | HTTP layer, SLA publisher | ✅ chi router + SLA + scheduler |
| ai-service | gRPC server, MongoDB repo, pgvector | ✅ gRPC handler + Mongo repo + vector store |
| notification-service | rule CRUD, in-app notif, digest | ✅ SSE handler + digest scheduler |
| gateway-service | 4 gRPC clients, Dashboard BFF, GraphQL | ✅ All clients + BFF + GraphQL BFF |

---


## ⚠️ Nguyên tắc Upgrade — CHỈ THÊM, KHÔNG XÓA

> **Toàn bộ code hiện có được GIỮ NGUYÊN.** Mọi upgrade đều thực hiện bằng cách **thêm file/package mới** bên cạnh code cũ, không xóa hay merge.
>
> - Code cũ tiếp tục hoạt động ổn định
> - Code mới được thêm vào trong packages riêng biệt
> - Feature flag hoặc config quyết định dùng impl nào
> - Có thể rollback bất kỳ lúc nào chỉ bằng cách thay config

---

## Tổng quan nhanh

| Service | Go Module | Domain Layer | Use Cases | Infra | Migrations | Trạng thái |
|---------|-----------|-------------|-----------|-------|-----------|-----------|
| identity-service | `github.com/osv/identity-service` | ✅ Đầy đủ | ✅ 7 UC | ⚠️ Thiếu mongo session | ✅ 1 file | 🟡 ~70% |
| data-service | `github.com/osv/data-service` | ✅ Cơ bản | ⚠️ Duplicate UC | ⚠️ Firestore còn dùng | ✅ 4 files | 🔴 ~50% |
| search-service | `github.com/osv/search-service` | ✅ Cơ bản | ✅ 6 UC | ⚠️ Multi-backend | ✅ Thiếu | 🟡 ~60% |
| scan-service | `github.com/osv/scan-service` | ✅ Đầy đủ | ✅ 2 UC | ✅ OK | ✅ 3 files | 🟡 ~65% |
| finding-service | `github.com/osv/finding-service` | ✅ Đầy đủ | ✅ Rich | ✅ Postgres | ✅ 6 files | 🟢 ~80% |
| ai-service | `github.com/osv/ai-service` | ✅ Đầy đủ | ✅ 5 UC | ⚠️ Firestore storage | ❌ Thiếu | 🟡 ~65% |
| notification-service | `github.com/osv/notification-service` | ✅ Đầy đủ | ✅ 6 UC | ⚠️ Firestore + Postgres | ✅ 4 files | 🟡 ~65% |
| gateway-service | `github.com/osv/gateway-service` | ✅ Cơ bản | ✅ 3 UC | ✅ OK | ✅ Thiếu | 🟡 ~60% |
| **shared/pkg** | `github.com/osv/shared/pkg` | — | — | — | — | 🟢 ~85% |
| **shared/proto** | `github.com/osv/shared/proto` | — | — | — | — | 🟡 ~60% |

---

## Chi tiết từng Service

---

### 1. identity-service

**Cấu trúc thực tế:**
```
identity-service/
├── adapter/
│   ├── handler/grpc/auth_grpc_handler.go   ✅
│   ├── handler/http/
│   │   ├── router.go                        ✅
│   │   ├── auth_handler.go                  ✅
│   │   ├── oauth_handler.go                 ✅
│   │   └── api_key_handler.go               ✅
│   └── repository/
│       ├── postgres/user_repo.go            ✅ (pgx/v5)
│       ├── postgres/session_repo.go         ✅
│       ├── postgres/api_key_repo.go         ✅
│       ├── mongo/user_repo.go               ✅ GIỮ (alternative impl)
│       └── mongo/session_repo.go            ✅ GIỮ (alternative impl)
├── internal/
│   ├── domain/entity/user.go               ✅ Đầy đủ (MFA, IsVerified, FailedLoginAttempts)
│   ├── domain/entity/session.go            ✅
│   ├── domain/entity/api_key.go            ✅
│   ├── domain/error/errors.go              ✅ 12 sentinel errors
│   ├── domain/identity/role.go             ✅
│   ├── domain/repository/repositories.go   ✅ Interfaces
│   ├── infra/auth/                         ✅ GIỮ (sẽ implement thêm vào đây)
│   ├── infrastructure/jwt/token.go         ✅ RS256
│   ├── infrastructure/crypto/argon2id.go   ✅ Argon2id
│   ├── infrastructure/crypto/apikey_totp.go ✅
│   ├── infrastructure/oauth/google.go      ✅
│   ├── infrastructure/oauth/github.go      ✅
│   ├── infrastructure/cache/token_cache.go ✅ Redis brute-force
│   ├── usecase/login/login.go              ✅ Đầy đủ (brute-force, MFA, session)
│   ├── usecase/register/register.go        ✅ Argon2id hash
│   ├── usecase/refresh_token/             ✅
│   ├── usecase/logout/                     ✅
│   ├── usecase/validate_token/             ✅
│   ├── usecase/oauth/                      ✅
│   └── usecase/manage_api_key/             ✅
├── migrations/001_initial_schema.sql       ✅
├── services/ (thư mục bổ sung)             ✅ GIỮ
└── go.mod (go 1.26.3)                      ✅
```

**Routes đã expose:**
```
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
GET    /api/v1/auth/me
GET    /api/v1/auth/providers
GET    /api/v1/auth/oauth/google + /callback
GET    /api/v1/auth/oauth/github + /callback
POST/GET/DELETE /api/v1/auth/api-keys
GET    /.well-known/jwks.json
GET    /health/live + /health/ready
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: TOTP management flow (setup/verify/disable) → thêm `usecase/totp/`
2. **Thiếu**: Admin use cases (list users, ban user, change role) → thêm `usecase/admin/`
3. **Thiếu**: Email verification flow → thêm `usecase/verify_email/`
4. **Thiếu**: Password reset flow → thêm `usecase/forgot_password/`
5. **`internal/infra/auth/`**: Hiện trống → thêm implementation vào đây (auth cache)
6. **`internal/provider/`**: Có local.go + chain.go → thêm kết nối vào use cases
7. **`adapter/repository/postgres/`**: Đây là primary impl → config wire qua env var

---

### 2. data-service

**Cấu trúc thực tế:**
```
data-service/
├── internal/
│   ├── domain/
│   │   ├── cve/                            ✅ GIỮ
│   │   ├── aggregate/                      ✅ CVE aggregate
│   │   ├── alias/ + kev/                   ✅
│   │   ├── repository/
│   │   │   ├── cve_repository.go           ✅ MongoDBCVERepository interface
│   │   │   ├── cvedb_repositories.go       ✅ CVEBinToolRepository (PostgreSQL)
│   │   │   ├── kev_repository.go           ✅
│   │   │   └── alias_group_repo.go         ✅
│   │   └── service/                        ✅ CVELookupService, TriageService
│   ├── usecase/                            ✅ GIỮ TẤT CẢ (kể cả 2 tầng)
│   │   ├── sync/usecase.go                 ✅ GIỮ (KEV CISA sync)
│   │   ├── kev/                            ✅
│   │   ├── cve/                            ✅ GIỮ cả subfolder
│   │   ├── syncall/, syncsource/           ✅
│   │   ├── initdb/, populatedb/, backupdb/ ✅
│   │   ├── importdb/, exportdb/            ✅
│   │   ├── lookupcves/                     ✅
│   │   ├── query/, check/                  ✅
│   │   ├── alias/                          ✅
│   │   └── searchbycpe/                    ✅
│   ├── fetcher/                            ✅ NVD CVE, NVD CPE, EPSS, MITRE CWE, CAPEC
│   ├── converter/                          ✅ NVD→OSV, CVE5 format
│   ├── sync/                               ✅ circl, ids, nvd, pypi
│   ├── adapter/                            ✅ GIỮ (grpc handler + cisa client)
│   ├── delivery/                           ✅
│   ├── application/command/                ✅ CQRS commands
│   ├── infra/
│   │   ├── external/cisa/client.go         ✅
│   │   ├── messaging/nats/                 ✅
│   │   ├── mongo/cve_repo.go               ✅
│   │   ├── persistence/
│   │   │   ├── postgres/kev_repo.go        ✅
│   │   │   └── firestore/alias_group_repo.go ✅ GIỮ (primary alias storage)
│   │   ├── storage/gcs/                    ✅
│   │   └── pipeline/idempotency/redis/     ✅
└── migrations/ (4 files)                   ✅
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: `infra/persistence/postgres/alias_group_repo.go` → PostgreSQL impl mới (song song với Firestore)
2. **Thiếu**: Git-based source sync → thêm `sync/git/`
3. **Thiếu**: GCS bucket sync → thêm `sync/gcs/`
4. **Thiếu**: Ingest pipeline → thêm `usecase/ingest/`
5. **Thiếu**: NATS CVE event publishing → thêm `infra/messaging/nats/cve_publisher.go`
6. **Thiếu**: OSV schema validation → thêm `infra/validator/osv_validator.go`

---

### 3. search-service

**Cấu trúc thực tế:**
```
search-service/
├── internal/
│   ├── domain/
│   │   ├── entity/cve.go                   ✅
│   │   ├── entity/search.go                ✅ SearchRequest, CVESummary
│   │   ├── repository/repository.go        ✅
│   │   └── service/service.go              ✅
│   ├── usecase/
│   │   ├── cvesearch/usecase.go            ✅ Cache-first search
│   │   ├── browse/                         ✅
│   │   ├── getbyid/                        ✅
│   │   ├── lookup/                         ✅
│   │   ├── rank/                           ✅
│   │   └── search/                         ✅
│   ├── infra/
│   │   ├── cache/redis/search_cache.go     ✅
│   │   ├── opensearch/opensearch_adapter.go ✅ (primary)
│   │   ├── elasticsearch/finding_indexer.go ✅ GIỮ
│   │   ├── postgres/cve_repo.go            ✅
│   │   ├── mongo/search_repo.go            ✅
│   │   ├── storage/gcs/                    ✅
│   │   ├── persistence/firestore/          ✅ GIỮ
│   │   └── messaging/nats/                 ✅
│   ├── application/
│   │   ├── command/index_vulnerability/    ✅
│   │   ├── query/search_vulnerabilities/   ✅
│   │   └── query/semantic_search/          ✅
│   ├── delivery/http/
│   │   ├── search_handler.go               ✅
│   │   └── dd/finding_search_handler.go    ✅
│   └── factory/                            ✅ Backend factory
└── go.mod (ES + OpenSearch + PostgreSQL + MongoDB + NATS)
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: OSV v1 API endpoints → thêm `delivery/http/osv_v1_handler.go`
2. **Thiếu**: DetermineVersion endpoint → thêm `usecase/determine_version/`
3. **Thiếu**: `delivery/http/osv_v1_handler.go` với `/v1/query`, `/v1/querybatch`, `/v1/vulns/list`
4. **Thiếu**: Subscribe thêm NATS subjects (data.cve.updated, data.cve.withdrawn)

---

### 4. scan-service

**Cấu trúc thực tế:**
```
scan-service/
├── internal/
│   ├── domain/
│   │   ├── scan/ → entity, repository      ✅
│   │   ├── agent/ → agent entity           ✅
│   │   ├── asset/ → asset entity           ✅
│   │   ├── schedule/                       ✅
│   │   └── entity/ (generic)               ✅
│   ├── usecase/
│   │   ├── create_scan/create_scan.go      ✅
│   │   └── execute_scan/execute_scan.go    ✅
│   ├── parsers/                            ✅ Go/Python/Node/Java/Rust
│   │   └── checkers/                       ✅
│   ├── sbom/                               ✅ CycloneDX, SPDX, SWID, VEX
│   ├── scheduler/cron_worker.go            ✅
│   ├── infra/
│   │   ├── leader/redis_lock.go            ✅
│   │   ├── persistence/postgres/schedule/  ✅
│   │   ├── messaging/nats/agent_publisher/ ✅
│   │   ├── scan_infra/                     ✅ GIỮ (legacy impl)
│   │   └── validator/xml_schema.go         ✅
│   ├── adapters/                           ✅ grpc, http, postgres, worker, scanners
│   ├── infrastructure/                     ✅ GIỮ (legacy)
│   └── delivery/http/schedule/             ✅
└── migrations/ (3 files)
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: Agent registration use case → thêm `usecase/register_agent/`
2. **Thiếu**: Agent heartbeat consumer → thêm `usecase/agent_heartbeat/`
3. **Thiếu**: Scan result processing → thêm `usecase/process_scan_result/`
4. **Thiếu**: SBOM ingest pipeline → thêm `delivery/http/sbom_handler.go`
5. **Thiếu**: gRPC client to finding-service → thêm `adapters/grpcclient/finding_client.go`

---

### 5. finding-service

**Cấu trúc thực tế:**
```
finding-service/
├── internal/
│   ├── domain/
│   │   ├── finding/entity.go               ✅ Rất đầy đủ
│   │   ├── finding/state_machine.go        ✅
│   │   ├── product/ + product_type/        ✅
│   │   ├── engagement/                     ✅
│   │   ├── test/ (security test concept)   ✅ GIỮ
│   │   ├── audit/                          ✅
│   │   ├── sla/                            ✅
│   │   ├── report/                         ✅
│   │   └── orchestrator/                   ✅
│   ├── usecase/                            ✅ Rich set
│   ├── formatters/                         ✅ JSON, CSV, Excel, PDF, HTML, Console
│   ├── infra/                              ✅ postgres, nats, parser, dedup
│   └── delivery/grpc/server/finding_server.go ✅
└── migrations/ (6 files)
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: HTTP delivery → thêm `delivery/http/` package
2. **Thiếu**: SLA breach NATS publisher → thêm `infra/messaging/nats/sla_publisher.go`
3. **Thiếu**: Risk acceptance use case → thêm `usecase/risk_acceptance/`
4. **Thiếu**: Finding tags → thêm vào entity + migration

---

### 6. ai-service

**Cấu trúc thực tế:**
```
ai-service/
├── internal/
│   ├── domain/
│   │   ├── enrichment/provider_chain.go    ✅ Circuit breaker (Vertex→OpenAI→Ollama)
│   │   ├── enrichment/embedding_service.go ✅
│   │   ├── enrichment/severity_classifier.go ✅
│   │   ├── enrichment/exploit/checker.go   ✅
│   │   ├── enrichment/mitretagger/tagger.go ✅
│   │   ├── enrichment/threatintel/         ✅
│   │   ├── enrichment/port/               ✅ Interfaces
│   │   └── triage/entity.go               ✅
│   ├── usecase/
│   │   ├── enrich_cve/handler.go          ✅
│   │   ├── batch_enrich/usecase.go        ✅
│   │   ├── generate_embedding/usecase.go  ✅
│   │   ├── triage_finding/usecase.go      ✅
│   │   └── epss/job.go                   ✅
│   ├── infra/
│   │   ├── ai/vertex/ + openai/ + ollama/ ✅
│   │   ├── ai/factory.go                  ✅
│   │   ├── providers/epss/client.go       ✅
│   │   ├── persistence/firestore/         ✅ GIỮ (primary enrichment storage)
│   │   └── messaging/nats/consumer.go     ✅
│   └── delivery/http/router.go            ✅ (minimal)
└── migrations/ MISSING
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: `infra/persistence/mongo/enrichment_repo.go` → MongoDB impl mới (song song Firestore)
2. **Thiếu**: `migrations/` directory + SQL files
3. **Thiếu**: gRPC server → thêm `delivery/grpc/`
4. **Thiếu**: `domain/enrichment/entity.go` tổng hợp EnrichmentResult
5. **Thiếu**: Complete HTTP handlers

---

### 7. notification-service

**Cấu trúc thực tế:**
```
notification-service/
├── internal/
│   ├── domain/
│   │   ├── rule/entity.go                  ✅ NotificationRule (10 event types, 5 channels)
│   │   ├── alert/entity.go                 ✅
│   │   ├── webhook/webhook.go              ✅
│   │   ├── delivery/entity.go             ✅
│   │   ├── integration/jira.go            ✅
│   │   ├── aggregate/webhook/             ✅
│   │   ├── repository/repository.go       ✅
│   │   └── errors/errors.go              ✅
│   ├── usecase/
│   │   ├── dispatch_alert/dispatch.go     ✅
│   │   ├── dispatch_webhook/dispatch.go   ✅
│   │   ├── jira_create_issue/             ✅
│   │   ├── jira_sync/                    ✅
│   │   ├── manage_subscription/register.go ✅
│   │   └── command/deliver_notification/  ✅
│   ├── infra/
│   │   ├── adapters/email/ + slack/ + teams/ ✅ GIỮ
│   │   ├── channels/                     ✅ GIỮ (alternative impl)
│   │   ├── persistence/firestore/repos.go ✅ GIỮ (primary rule storage)
│   │   ├── persistence/postgres/webhook_repo.go ✅
│   │   ├── messaging/nats/               ✅
│   │   ├── delivery/http_webhook_deliverer.go ✅ GIỮ
│   │   └── dispatcher/http_dispatcher.go ✅ GIỮ
│   ├── adapter/dispatcher/               ✅ GIỮ
│   ├── integrations/jira/                ✅
│   └── delivery/http/integration_handler.go ✅
└── migrations/ (4 files)
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: `infra/persistence/postgres/rule_repo.go` → PostgreSQL impl mới (song song Firestore)
2. **Thiếu**: Rule management HTTP endpoints → thêm `delivery/http/rule_handler.go`
3. **Thiếu**: Alert history endpoint → thêm `delivery/http/alert_handler.go`
4. **Thiếu**: Retry logic → thêm `usecase/retry_delivery/`
5. **Thiếu**: Digest mode → thêm `usecase/send_digest/`

---

### 8. gateway-service

**Cấu trúc thực tế:**
```
gateway-service/
├── internal/
│   ├── domain/auth/ + entity/ + policy/    ✅
│   ├── usecase/dbsync/ + scan/ + report/   ✅
│   ├── auth/dd_middleware.go + osv_middleware.go ✅ GIỮ CẢ 2
│   ├── bff/
│   │   ├── dashboard.go                    ✅ GIỮ (cần implement nội dung)
│   │   ├── clients/grpc_clients.go         ✅
│   │   └── handlers/                       ✅ v1, scan, report, sbom, osv, db, dd
│   ├── proxy/                              ✅ http, grpc, dd, routes
│   ├── adapter/grpcclient/scanner_client + cvedb_client ✅
│   ├── infra/                              ✅ GIỮ (sẽ thêm Redis vào đây)
│   ├── delivery/http/middleware/jwt.go + ratelimit.go ✅ GIỮ CẢ
│   ├── health/info_handler.go              ✅
│   └── ratelimit/ratelimit.go              ✅
└── config/ (routes.yaml, upstreams.yaml)   ✅
```

**Gaps phát hiện (cần THÊM):**
1. **Thiếu**: implement `bff/dashboard.go::GetDashboard()` (struct có rồi, chỉ thiếu logic)
2. **Thiếu**: gRPC clients → thêm `adapter/grpcclient/identity_client.go`, `finding_client.go`, `ai_client.go`, `notification_client.go`
3. **Thiếu**: Redis token cache → thêm `infra/redis/token_cache.go`
4. **Thiếu**: CVE Detail BFF → thêm `bff/handlers/cve_detail_handler.go`
5. **Thiếu**: Health aggregation → thêm `health/aggregated_health.go`

---

## Shared Libraries — Trạng thái

### shared/pkg — Rất đầy đủ ✅
```
shared/pkg/
├── classification/    ✅ CVE severity classification
├── clients/           ✅ GCP (Datastore, PubSub, Storage), Redis
├── config/            ✅ Config loading
├── cpe/               ✅ CPE parsing
├── cveid/             ✅ CVE ID validation
├── cwe/               ✅ CWE lookup
├── database/          ✅ PostgreSQL, MongoDB helpers
├── domain/            ✅ Shared domain types
├── ecosystem/         ✅ (osv-scalibr based)
├── errors/            ✅ Common errors
├── grpcutil/          ✅ gRPC helpers
├── health/            ✅ Health check
├── logger/            ✅ zerolog
├── middleware/        ✅ HTTP middleware
├── models/            ✅ Shared models
├── nats/              ✅ NATS JetStream helpers
├── observability/     ✅ OpenTelemetry
├── osvschema/         ✅ OSV schema bindings
├── osvutil/           ✅ OSV utilities
├── pagination/        ✅
├── pgp/               ✅ PGP signing
├── purl/              ✅ PURL parsing
├── resilience/        ✅ Circuit breaker, retry
├── search/            ✅
├── semver/            ✅ SemVer normalization
├── severity/          ✅
├── test/ + testutil/  ✅
└── version/           ✅
```

**Cần THÊM vào shared/pkg:**
- `pkg/storage/` — unified storage abstraction (GCS/local/S3)
- `pkg/multistore/` — multi-backend store selector (Firestore vs PostgreSQL)

### shared/proto — Có nhưng chưa hoàn chỉnh
```
shared/proto/
├── ai/                ✅
├── asset/             ✅
├── auth/v1/           ✅
├── cve/               ✅
├── cvedb/v1/          ✅
├── datasync/          ✅
├── finding/           ✅
├── identity/          ✅ (2 versions — GIỮ CẢ 2)
├── notification/      ✅
├── product/           ✅
├── reporter/          ✅
├── sbomvex/           ✅
├── scan/              ✅
├── scanner/           ✅
└── search/            ✅
```

---

## Gaps Summary — Những gì cần THÊM

### 🔴 P0 — Thêm ngay (blocking features)

| # | Cần thêm | Service | Lý do |
|---|---------|---------|-------|
| 1 | `infra/persistence/postgres/alias_group_repo.go` | data | Alternative cho Firestore |
| 2 | `bff/dashboard.go` — implement GetDashboard() | gateway | Core feature empty |
| 3 | `adapter/grpcclient/identity_client.go` | gateway | Auth validation thiếu |
| 4 | `adapter/grpcclient/finding_client.go` | gateway | Dashboard BFF cần |
| 5 | `infra/redis/token_cache.go` | gateway | Performance critical |
| 6 | `migrations/` directory | ai | DB schema không tồn tại |

### 🟡 P1 — Thêm trong Sprint 2

| # | Cần thêm | Service |
|---|---------|---------|
| 7 | `infra/messaging/nats/cve_publisher.go` | data |
| 8 | `sync/git/git_source.go` | data |
| 9 | `delivery/http/osv_v1_handler.go` | search |
| 10 | `delivery/http/` package (REST API) | finding |
| 11 | `infra/messaging/nats/sla_publisher.go` | finding |
| 12 | `infra/persistence/postgres/rule_repo.go` | notification |
| 13 | `delivery/grpc/` package | ai |

### 🟢 P2 — Thêm trong Sprint 3+

| # | Cần thêm | Service |
|---|---------|---------|
| 14 | `usecase/totp/` (setup/verify/disable) | identity |
| 15 | `usecase/forgot_password/` + `usecase/reset_password/` | identity |
| 16 | `usecase/verify_email/` | identity |
| 17 | `usecase/admin/` | identity |
| 18 | `usecase/register_agent/` + `usecase/agent_heartbeat/` | scan |
| 19 | `usecase/retry_delivery/` + `scheduler/retry_worker.go` | notification |
| 20 | `delivery/http/rule_handler.go` + `alert_handler.go` | notification |
| 21 | `bff/handlers/cve_detail_handler.go` | gateway |
| 22 | `health/aggregated_health.go` | gateway |
| 23 | `infra/persistence/mongo/enrichment_repo.go` | ai |
| 24 | `domain/enrichment/entity.go` | ai |
