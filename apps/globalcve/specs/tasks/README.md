# GlobalCVE v3.0 — Task Index

Danh sách các task để thực thi kiến trúc monolithic Go app.

## Trạng Thái Tổng Quan

| Task | Tên | Trạng Thái | Phụ Thuộc |
|------|-----|------------|-----------|
| [TASK-01](TASK-01-project-scaffold.md) | Project Scaffold | ✅ Completed | — |
| [TASK-02](TASK-02-shared-infra.md) | Shared Infrastructure | ✅ Completed | TASK-01 |
| [TASK-03](TASK-03-db-migrations.md) | Database Migrations | ✅ Completed | TASK-02 |
| [TASK-04](TASK-04-cvesearch-service.md) | CVE Search Service | ✅ Completed | TASK-03 |
| [TASK-05](TASK-05-cvesync-service.md) | CVE Sync Service | ✅ Completed | TASK-03 |
| [TASK-06](TASK-06-kev-service.md) | KEV Service | ✅ Completed | TASK-03 |
| [TASK-07](TASK-07-notification-service.md) | Notification Service | ✅ Completed | TASK-02 |
| [TASK-08](TASK-08-api-gateway.md) | API Gateway | ✅ Completed | TASK-04,05,06,07 |
| [TASK-09](TASK-09-app-lifecycle.md) | App Lifecycle & main.go | ✅ Completed | TASK-08 |
| [TASK-10](TASK-10-docker-devenv.md) | Docker & Dev Environment | ✅ Completed | TASK-09 |

**10/10 tasks completed** — `go build ./...` passes ✅

## Thứ Tự Thực Thi

```
TASK-01 → TASK-02 → TASK-03 ─┬─→ TASK-04 ─┐
                               ├─→ TASK-05 ─┤
                               ├─→ TASK-06 ─┼─→ TASK-08 → TASK-09 → TASK-10
                               └─→ TASK-07 ─┘
```

## Files Đã Tạo / Cập Nhật

### Core
- `cmd/main.go` — Entry point với flag `--config`
- `config/config.yaml` — Full config với defaults
- `internal/app/app.go` — errgroup lifecycle, fail-fast/fail-open DI wiring
- `internal/config/config.go` — Viper config loader

### Shared Infrastructure
- `internal/infra/postgres/pool.go` — pgx connection pool
- `internal/infra/redis/client.go` — go-redis client
- `internal/infra/nats/client.go` — NATS JetStream + stream provisioning
- `internal/infra/opensearch/client.go` — OpenSearch client
- `internal/events/events.go` — NATS event type definitions

### Database
- `migrations/001_create_cves.sql` — cves table + GIN + ivfflat + pgvector
- `migrations/002_create_sync_jobs.sql` — sync_jobs tracking
- `migrations/003_create_kev_entries.sql` — CISA KEV entries
- `migrations/004_create_support_tables.sql` — webhooks, cpe_entries, cwe_entries

### Services
- `internal/cvesearch/` — Domain, repo, adapter, usecase, HTTP, service
- `internal/cvesync/` — Fetchers (NVD/CIRCL/JVN/ExploitDB/CVE.org/EPSS), scheduler, orchestrator
- `internal/kevservice/` — CISA client, domain, usecase, service
- `internal/notification/` — Webhook CRUD, NATS subscriber, dispatch
- `internal/gateway/` — Chi router, direct call, reverse proxy, middlewares

### Dev Environment
- `docker-compose.yml` — PostgreSQL+pgvector, Redis, NATS JetStream, OpenSearch
- `Makefile` — build, run, dev, test, lint, migrate-*, infra-*, setup
- `.env.example` — All required variables documented
- `.air.toml` — Hot reload config
- `.golangci.yml` — Linter config

---
*Tasks v1.1 | 2026-06-10 | GlobalCVE Monolithic Go App — ALL TASKS COMPLETE*
