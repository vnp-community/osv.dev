# SOL-INIT-000 — Index Các Giải Pháp Thực Thi Init-Account CRs

## Tổng Quan Giải Pháp

Các giải pháp này được thiết kế dựa trên:
- **Kiến trúc**: Clean Architecture 4-layer (domain → usecase → adapter → infra)
- **Schema**: Schema-per-service trong PostgreSQL (osv_identity, osv_cves, v.v.)
- **Auth**: RS256 JWT với RSA-4096 key + Argon2id password hashing
- **Hiện trạng code**: identity-service đã có `crypto/argon2id.go`, `jwt/token.go` với `PublicKeyJWKS()`

## Danh Sách Giải Pháp

| Solution | CR | Nội dung | Files |
|----------|----|----------|-------|
| [SOL-INIT-001](./SOL-INIT-001-env-bootstrap.md) | CR-INIT-001 | Tạo `.env.bootstrap` root + `.env` per-service | 2 files |
| [SOL-INIT-002](./SOL-INIT-002-identity-bootstrap.md) | CR-INIT-002 | identity-service: init script, JWKS handler, admin seeder | 4 files |
| [SOL-INIT-003](./SOL-INIT-003-data-bootstrap.md) | CR-INIT-003 | data-service: init script, fix storage_config envOr | 2 files |
| [SOL-INIT-004](./SOL-INIT-004-search-bootstrap.md) | CR-INIT-004 | search-service: init script, Redis password support | 2 files |
| [SOL-INIT-005](./SOL-INIT-005-ranking-bootstrap.md) | CR-INIT-005 | ranking-service: init script, /health endpoint, health_port env | 2 files |
| [SOL-INIT-006](./SOL-INIT-006-notification-bootstrap.md) | CR-INIT-006 | notification-service: init script, /health handler | 2 files |
| [SOL-INIT-007](./SOL-INIT-007-ai-bootstrap.md) | CR-INIT-007 | ai-service: init script, gRPC health + grpc_health_probe | 2 files |
| [SOL-INIT-008](./SOL-INIT-008-gateway-osv-bootstrap.md) | CR-INIT-008 | gateway + apps/osv: init script, JWT_SECRET guard, JWKS verify | 3 files |
| [SOL-INIT-009](./SOL-INIT-009-master-bootstrap.md) | CR-INIT-009 | `scripts/bootstrap.sh`, `start-all.sh`, `health-check.sh` | 3 files |

## Kiến Trúc DB Schemas

Theo spec `01-architecture.md §4.1`, mỗi service dùng schema riêng trong PostgreSQL:

```
osvdb (single cluster)
├── auth        ← identity-service   (users, sessions, api_keys, oauth_accounts, audit_log)
├── vuln        ← data-service       (cves, kev_entries, capec_patterns, cwe_weaknesses, alias_groups)
├── notif       ← notification-svc   (webhooks, notification_rules, in_app_alerts)
└── public      ← shared extensions  (uuid-ossp, citext, pgvector)
```

## Port Map (theo spec)

| Service | HTTP | gRPC | Metrics |
|---------|------|------|---------|
| identity-service | 9101 | 9001 | — |
| data-service | 8080 | 50053 | 9092 |
| search-service | 8082 | 50056 | 9091 |
| ranking-service | 8088 | — | — |
| notification-service | 8086 | 50063 | 9094 |
| ai-service | — | 50052 | — |
| gateway-service | 8080 | 9090 | 9090 |
| apps/osv | 8080 | — | — |

## Phụ Thuộc Khởi Động

```
PostgreSQL ──► identity-service (schema: auth)
             ► data-service     (schema: vuln)
             ► notification-svc (schema: notif)
Redis       ──► identity-service (token cache/blacklist)
              ► search-service   (CPE browse cache)
              ► apps/osv         (rate limiting)
MongoDB     ──► ranking-service  (cpe_rankings)
NATS        ──► notification-svc (vuln events, optional)
OpenSearch  ──► search-service   (FTS index, optional - fallback to PG)
```
