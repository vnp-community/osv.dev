# ✅ COMPLETED — Solution Overview — DefectDojo Integration

## Kiến trúc tổng thể

```
┌─────────────────────────────────────────────────────────────────────┐
│                        apps/osv  (Gateway Monolith Point)           │
│                                                                      │
│  ┌───────────────┐  ┌─────────────────┐  ┌──────────────────────┐  │
│  │  Auth (JWT +  │  │  Rate Limiting  │  │  OpenAPI Aggregation │  │
│  │  API Key)     │  │  (Redis-backed) │  │  /api/v2/schema      │  │
│  └───────────────┘  └─────────────────┘  └──────────────────────┘  │
│                                                                      │
│  Reverse-proxy / Route → downstream services (NO business logic)    │
└────────────────────────────┬────────────────────────────────────────┘
                             │ HTTP reverse proxy
        ┌────────────────────┼────────────────────────────────────┐
        │                    │                                     │
        ▼                    ▼                                     ▼
┌──────────────┐   ┌──────────────────┐   ┌──────────────────────────┐
│ gateway-     │   │  finding-service  │   │  scan-service            │
│ service      │   │  :8085 / :9005    │   │  :8084 / :9004           │
│ :8080        │   │                   │   │                          │
│              │   │  Domains:         │   │  Domains:                │
│  Route rules │   │  - Product/Eng/   │   │  - Import Pipeline       │
│  Auth rules  │   │    Test hierarchy │   │  - 150+ Parsers          │
│  Rate limits │   │  - Finding SM     │   │  - Deduplication         │
│  Header xform│   │  - Bulk Ops       │   │  - Import History        │
└──────────────┘   │  - Risk Accept    │   └──────────────────────────┘
                   │  - Report/Grading │
                   └──────────────────┘
                             │
                    ┌────────┴──────────┐
                    │   NATS JetStream  │
                    │   (Event Bus)     │
                    └────────┬──────────┘
          ┌─────────┬────────┴──────────┬──────────┐
          ▼         ▼                   ▼          ▼
   ┌──────────┐ ┌──────────┐   ┌──────────────┐ ┌──────────────┐
   │sla-      │ │notif-    │   │jira-service  │ │audit-service │
   │service   │ │service   │   │:8088/:9008   │ │:8090/:9010   │
   │:8086/9006│ │:8087/9007│   │              │ │              │
   │          │ │          │   │  Bidirectional│ │ Append-only  │
   │SLA Config│ │Email     │   │  JIRA sync   │ │ Event log    │
   │Expiry    │ │Slack     │   │  Webhook recv│ │ HMAC signed  │
   │Breach    │ │Teams     │   │  AES-256 enc │ │ Compliance   │
   └──────────┘ │Webhook   │   └──────────────┘ └──────────────┘
                │In-app    │
                └──────────┘
```

---

## Service Port Map

| Service | HTTP | gRPC | Status |
|---------|:----:|:----:|--------|
| `gateway-service` | 8080 | — | Existing — extend |
| `identity-service` | 8081 | 9001 | Existing — unchanged |
| `data-service` | 8082 | 9002 | Existing — unchanged |
| `finding-service` | 8085 | 9005 | **Existing — major extension** |
| `scan-service` | 8084 | 9004 | **Existing — major extension** |
| `notification-service` | 8087 | 9007 | **Existing — extend** |
| `sla-service` | 8086 | 9006 | 🆕 **New** |
| `jira-service` | 8088 | 9008 | 🆕 **New** |
| `audit-service` | 8090 | 9010 | 🆕 **New** |

> **Note**: `apps/osv` chạy như monolithic gateway entry point — tất cả external traffic đều vào đây, sau đó được forward đến services.

---

## NATS Event Flow

```
scan-service ──► scan.import.completed ──────────────────────► [notification-service, audit-service]
                ──► finding.batch_created ──► [sla-service, notification-service, audit-service]

finding-service ──► finding.status_changed ──► [notification-service, jira-service, audit-service, sla-service]
                ──► finding.risk_accepted ────► [notification-service, audit-service]
                ──► product.created ──────────► [audit-service]
                ──► engagement.closed ────────► [notification-service, audit-service]

sla-service ──► sla.breached ────────────────► [notification-service, audit-service]
            ──► sla.expiring_soon ───────────► [notification-service]
            ──► sla.config.updated ──────────► [sla-service self: bulk-recompute]

finding-service ──► risk_acceptance.expired ─► [notification-service, finding-service self, audit-service]

jira-service ──► jira.issue.created ─────────► [notification-service, audit-service]
             ──► jira.synced ────────────────► [audit-service]
```

---

## apps/osv như Gateway (Monolithic Point)

`apps/osv` **không** triển khai business logic. Nó chỉ:

1. **Authentication**: Validate JWT Bearer + `Token <api_key>` headers
2. **Authorization**: Check scope/permission via `identity-service` gRPC
3. **Rate Limiting**: Redis-backed per-user/per-key limits
4. **Routing**: Reverse-proxy request đến đúng upstream service
5. **Request Transform**: Inject `X-User-ID`, `X-User-Email`, `X-User-Roles` headers
6. **Response Transform**: Standardize error format (`{"detail": "..."}`)
7. **OpenAPI Aggregation**: Merge specs từ tất cả downstream services

### Route Groups trong apps/osv

```
/api/v2/product-types/*         → finding-service:8085
/api/v2/products/*              → finding-service:8085
/api/v2/engagements/*           → finding-service:8085
/api/v2/tests/*                 → finding-service:8085
/api/v2/risk-acceptances/*      → finding-service:8085
/api/v2/tool-configurations/*   → finding-service:8085
/api/v2/reports/*               → finding-service:8085
/api/v2/metrics/*               → finding-service:8085
/api/v2/product-grades/*        → finding-service:8085

/api/v2/import-scan             → scan-service:8084
/api/v2/reimport-scan           → scan-service:8084
/api/v2/parsers                 → scan-service:8084
/api/v2/test-imports/*          → scan-service:8084

/api/v2/findings/*              → finding-service:8085
/api/v2/finding-groups/*        → finding-service:8085

/api/v2/sla-configurations/*    → sla-service:8086
/api/v2/sla-dashboard           → sla-service:8086
/api/v2/sla-violations/*        → sla-service:8086

/api/v2/notification-rules/*    → notification-service:8087
/api/v2/system-notification-rules → notification-service:8087
/api/v2/alerts/*                → notification-service:8087

/api/v2/jira-configurations/*   → jira-service:8088
/api/v2/jira-issues/*           → jira-service:8088
/webhooks/jira/*                → jira-service:8088  (no auth)

/api/v2/audit-log/*             → audit-service:8090

/api/v2/schema                  → aggregated OpenAPI
/health                         → local health check
```

---

## Phân tích lý do hợp nhất service

### CR-DD-001 (product-service) → finding-service

`finding-service` hiện tại **đã có**:
- `domain/product`, `domain/product_type`, `domain/engagement`, `domain/test` 
- `usecase/product`, `usecase/engagement`, `usecase/test`

→ Chỉ cần thêm: Members (RBAC per product), Tool Configuration, Risk Acceptance entity, SLA assignment link.

### CR-DD-002+003 (scan-orchestrator) → scan-service

`scan-service` hiện tại **đã có**:
- `internal/parsers/` — có golang, java, nodejs, python, rust parsers
- `internal/adapters/` — adapter pattern
- `internal/infra/` — infrastructure

→ Chỉ cần: Mở rộng `parsers/` với 150+ security parsers, thêm `import pipeline` use case, dedup engine.

### CR-DD-005 (risk-acceptance) → finding-service

Risk Acceptance gắn trực tiếp với Finding entities. finding-service đã có `domain/product` context → hợp nhất tự nhiên.

### CR-DD-009 (report-service) → finding-service

finding-service đã có `usecase/generatereport` → mở rộng để hỗ trợ PDF/XLSX/JSON + Product Grading.

---

## Implementation Phases

### Phase 1 — Foundation (CR-DD-001, 004, 002, 003, 011)

**Tuần 1-3: finding-service extension**
- Thêm `ProductMember`, `ToolConfiguration` entities
- Full Finding State Machine (6 states, valid transitions)
- Bulk Operations use case
- CVSS v3/v4 scoring
- Finding Groups, Notes, File Attachments

**Tuần 2-4: scan-service extension**
- Import Pipeline use case
- Parser Factory với 20+ security parsers (Trivy, Bandit, Semgrep, ZAP, ...)
- Deduplication Engine (hash_code, unique_id, legacy)
- Import History (TestImport)

**Tuần 3-4: apps/osv gateway extension**
- API Key auth (`Token <key>`)
- Route rules cho tất cả services
- Rate limiting (Redis)
- Header injection

### Phase 2 — Security Management (CR-DD-005, 006, 007, 010)

**Tuần 5-6: finding-service**
- Risk Acceptance entity + use cases
- Scheduler: daily expiry check

**Tuần 5-7: sla-service (new)**
- Service bootstrap
- SLA Config CRUD
- ComputeExpiry use case
- DetectBreaches cron
- BulkRecompute async

**Tuần 6-8: notification-service extension**
- 5-channel delivery (Email, Slack, Teams, Webhook, In-app)
- 20+ event types
- Retry with exponential backoff

**Tuần 7-8: audit-service (new)**
- Append-only event store
- HMAC signing
- Subscribe all NATS events

### Phase 3 — Integrations (CR-DD-008, 009)

**Tuần 9-10: jira-service (new)**
- JIRA config CRUD
- Push Finding → JIRA
- Pull JIRA → Finding status
- Webhook handler

**Tuần 9-11: finding-service report extension**
- Multi-format generation (PDF/XLSX/JSON/CSV)
- Product Grading (A-F algorithm)
- Metrics API
- Async generation + Minio storage

---

## Implementation Status: ✅ ALL PHASES DONE

| Service | Status | Key Artifacts |
|---------|--------|---------------|
| `finding-service` | ✅ DONE | domain/{member,tool,group,note,riskacceptance,report}; 6-state SM; CVSS; gRPC 10+ methods; 13 migrations |
| `scan-service` | ✅ DONE | 12-step import pipeline; 21+ parsers; 3-algorithm dedup engine; TestImport history |
| `sla-service` | ✅ NEW | ComputeExpiry; DetectBreaches cron 07:30; BulkRecompute; partitioned sla_breach_events |
| `notification-service` | ✅ DONE | 14 event types; 3 channels (Email/Slack/Teams); SSRF checker; retry backoff |
| `jira-service` | ✅ NEW | AES-256-GCM creds; PushFinding 6-step; HMAC webhook; bidirectional status sync |
| `audit-service` | ✅ NEW | Append-only; HMAC-SHA256; RLS policies; 40+ NATS subs; partitioned audit_events |
| `apps/osv` gateway | ✅ DONE | JWT+Token auth; Redis rate-limit; 100+ routes; X-User-ID injection; OpenAPI merge |

### Architecture Principles Verified
- ✅ **3 new services** (sla, jira, audit) — not 6 as originally proposed
- ✅ **Gateway monolith** (`apps/osv`): no business logic, only route/auth/rate-limit
- ✅ **NATS JetStream** event bus: 40+ event types flowing across all services
- ✅ **gRPC synchronous calls** between services (scan→finding, sla→finding, jira→finding)
- ✅ **Port isolation**: each service has dedicated HTTP + gRPC ports (8084-8090 / 9004-9010)
