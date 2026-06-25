# Solutions — GlobalCVE Change Requests

## Nguyên tắc kiến trúc

### 1. `apps/osv` = Gateway / Monolithic Entry Point

`apps/osv` **không chứa business logic**. Vai trò duy nhất:
- Nhận request từ client
- Forward tới upstream service tương ứng (HTTP reverse proxy hoặc gRPC)
- Tổng hợp response khi cần (BFF pattern)
- Health check aggregation

### 2. Business Logic = Services

Toàn bộ xử lý nghiệp vụ nằm trong `services/`:

| Service | Port | Vai trò |
|---------|------|---------|
| `data-service` | :50053 (gRPC), :8082 (HTTP) | CVE sync pipeline, KEV management, EPSS/CAPEC/CWE fetchers |
| `search-service` | :8081 | Full-text search, semantic search, vendor/product filter |
| `notification-service` | :8084 | Webhook registration, CVE alert delivery |
| `gateway-service` | :8080 | API key auth, rate limit, response caching, routing |

### 3. Tối thiểu hóa Service mới

CR-GCV-006 yêu cầu `notification-service` mới — **đây là service DUY NHẤT cần tạo mới**.
Tất cả CR còn lại là **extension của services hiện có**.

---

## Mapping CR → Service

| CR ID | Mô tả | Service chính | Loại thay đổi |
|-------|-------|---------------|---------------|
| [CR-GCV-001](../CR-GCV-001-multi-source-fetcher-pipeline.md) | Multi-Source Fetcher Pipeline | `data-service` | Extend fetchers |
| [CR-GCV-002](../CR-GCV-002-epss-integration.md) | EPSS Integration | `data-service` + `search-service` | New fields + filter |
| [CR-GCV-003](../CR-GCV-003-mitre-capec-cwe-enrichment.md) | MITRE CAPEC + CWE | `data-service` + `search-service` | New fetchers + endpoints |
| [CR-GCV-004](../CR-GCV-004-opensearch-semantic-search.md) | OpenSearch + pgvector | `search-service` | Dual backend |
| [CR-GCV-005](../CR-GCV-005-nvd-cpe-dictionary-vendor-filter.md) | NVD CPE + Vendor Filter | `data-service` + `search-service` | New fetcher + endpoints |
| [CR-GCV-006](../CR-GCV-006-notification-webhook-service.md) | Notification & Webhook | `notification-service` (**NEW**) | New service |
| [CR-GCV-007](../CR-GCV-007-kev-service-enhancement.md) | KEV Enhancement | `data-service` | Extend KEV domain |
| [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) | API Gateway Enhancement | `gateway-service` | Extend gateway |
| [CR-GCV-009](../CR-GCV-009-observability-logging-metrics-tracing.md) | Observability | All services (shared pkg) | Cross-cutting |
| [CR-GCV-010](../CR-GCV-010-export-source-attribution-ui-api.md) | Export + Source Attribution | `search-service` | New endpoints |

---

## Solution Documents

| File | Nội dung |
|------|---------|
| [SOL-GCV-001.md](./SOL-GCV-001-multi-source-fetcher.md) | Multi-source fetcher registry trong data-service |
| [SOL-GCV-002.md](./SOL-GCV-002-epss-integration.md) | EPSS sync + search filter |
| [SOL-GCV-003.md](./SOL-GCV-003-capec-cwe-enrichment.md) | MITRE CAPEC/CWE sync + search endpoints |
| [SOL-GCV-004.md](./SOL-GCV-004-opensearch-semantic-search.md) | OpenSearch + pgvector search backend |
| [SOL-GCV-005.md](./SOL-GCV-005-nvd-cpe-vendor-filter.md) | CPE dictionary + vendor/product filter |
| [SOL-GCV-006.md](./SOL-GCV-006-notification-webhook.md) | notification-service (new) |
| [SOL-GCV-007.md](./SOL-GCV-007-kev-enhancement.md) | KEV KnownRansomware + stats + NATS |
| [SOL-GCV-008.md](./SOL-GCV-008-api-gateway-enhancement.md) | API Key auth + health aggregation + routes |
| [SOL-GCV-009.md](./SOL-GCV-009-observability.md) | Observability shared package |
| [SOL-GCV-010.md](./SOL-GCV-010-export-source-attribution.md) | Export + Source Attribution |

---

## Kiến trúc tổng quan sau khi thực thi

```
┌───────────────────────────────────────────────────────────────────┐
│                         EXTERNAL CLIENTS                           │
│            Browser | CLI | CI/CD | Third-party Webhooks           │
└───────────────────────────────────────────────────────────────────┘
                                  │ HTTPS
                                  ▼
┌───────────────────────────────────────────────────────────────────┐
│                    apps/osv  (GATEWAY)  :8080                     │
│   ┌─────────────────────────────────────────────────────────┐     │
│   │              gateway-service (embedded)                  │     │
│   │  • API Key + JWT auth (CR-GCV-008)                      │     │
│   │  • Rate limiting per tier                               │     │
│   │  • Response caching (Redis)                             │     │
│   │  • Health aggregation                                   │     │
│   │  • Reverse proxy to upstream services                   │     │
│   └─────────────────────────────────────────────────────────┘     │
└───────────────────────────────────────────────────────────────────┘
          │                   │                    │
          ▼                   ▼                    ▼
┌──────────────────┐ ┌────────────────┐ ┌───────────────────────┐
│  data-service    │ │ search-service │ │ notification-service  │
│  :8082 / :50053  │ │  :8081         │ │  :8084                │
│                  │ │                │ │                       │
│ • CVE sync       │ │ • Keyword FTS  │ │ • Webhook registration│
│ • NVD/CIRCL/JVN  │ │ • OpenSearch   │ │ • HMAC delivery       │
│ • ExploitDB      │ │ • pgvector sem │ │ • Retry/backoff       │
│ • CVE.org/CNNVD  │ │ • EPSS filter  │ │ • Subscriptions       │
│ • EPSS sync      │ │ • CWE/CAPEC    │ │ • Alert dedup         │
│ • CAPEC/CWE sync │ │ • Vendor filter│ │ • SSRF protection     │
│ • CPE dict sync  │ │ • Aggregations │ └───────────────────────┘
│ • KEV service    │ │ • Export JSON/ │
│ • NATS publish   │ │   CSV          │
└──────────────────┘ └────────────────┘
          │
          ▼
┌──────────────────────────────────────────────────────┐
│                  Data Layer                           │
│  PostgreSQL 16          OpenSearch       Redis        │
│  + pgvector             (BM25 index)     (cache/RL)   │
│  cves, kev,             cves index       gw:cache:    │
│  cpe_dict,                               apikey:      │
│  cwe, capec,                             rl:tier:     │
│  webhooks, api_keys                                   │
└──────────────────────────────────────────────────────┘
```

---

## Implementation Priority

### Phase 1 — Core Pipeline & Gateway (High)
1. **SOL-GCV-001**: Multi-source fetcher registry ✅
2. **SOL-GCV-002**: EPSS integration ✅
3. **SOL-GCV-008**: API Gateway enhancements ✅

### Phase 2 — Enrichment & Observability (Medium)
4. **SOL-GCV-007**: KEV enhancements ✅
5. **SOL-GCV-003**: MITRE CAPEC/CWE ✅
6. **SOL-GCV-005**: NVD CPE + Vendor filter ✅
7. **SOL-GCV-009**: Observability ✅

### Phase 3 — Advanced Search & Notifications (Medium)
8. **SOL-GCV-004**: OpenSearch + pgvector ✅
9. **SOL-GCV-006**: Notification/Webhook service ✅

### Phase 4 — Export & UI Support (Low)
10. **SOL-GCV-010**: CVE Export + Source Attribution ✅
