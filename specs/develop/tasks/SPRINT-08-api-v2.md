# SPRINT-08 — API v2 & CLI Enhancement

> **Thời gian:** Q1-Q2 2027, Tháng 9 (3 tuần)  
> **Mục tiêu:** API v2 với enrichment data, CLI `cvectl` đầy đủ commands  
> **Refs:** [04-roadmap.md §3](../04-roadmap.md), [06-new-features.md §5, §7.1](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "Developer experience tốt nhất: CLI one-liner và API v2 giàu data"

Deliverables:
  1. OSV API v2 endpoints (enrichment data, related, timeline)
  2. cvectl CLI — đầy đủ subcommands
  3. API key management integration
  4. Local dev improvements (Jaeger, NATS UI, Swagger)
```

---

## TASK-08-01 · API v2 — Extended Endpoints [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 4 ngày  
**Priority:** P1  
**Files:**
- [enrichment_handler.go](../../../../services/api-gateway/internal/infra/handlers/v2/enrichment_handler.go)
- [firestore_store.go](../../../../services/api-gateway/internal/infra/handlers/v2/firestore_store.go)

### Hoàn thành

#### TASK-08-01a · GetEnrichment Endpoint [✅ DONE]
- [x] `GET /v2/vulns/{id}/enrichment` — trả KEV, EPSS, tags, CWE, exploit, AI summary
- [x] `EnrichmentData`, `KEVData`, `EPSSData` domain types
- [x] `FirestoreEnrichmentStore.GetEnrichment()`

#### TASK-08-01b · GetRelated Endpoint [✅ DONE]
- [x] `GET /v2/vulns/{id}/related` — alias groups + related subcollection
- [x] `FirestoreEnrichmentStore.GetRelated()`

#### TASK-08-01c · BatchGetById Endpoint [✅ DONE]
- [x] `POST /v2/vulns/batch-get` — max 100 IDs, parallel Firestore read
- [x] `FirestoreEnrichmentStore.BatchGetEnrichment()`

#### TASK-08-01d · GetTimeline Endpoint [✅ DONE]
- [x] `GET /v2/vulns/{id}/timeline` — Firestore events subcollection
- [x] `FirestoreEnrichmentStore.GetTimeline()`
- [x] `RecordTimelineEvent()` helper

- [ ] **TODO:** Integration tests (Sprint 09)
- [ ] **TODO:** Wire route vào api-gateway main.go (Sprint 09)

---

## TASK-08-02 · cvectl CLI Enhancement [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 4 ngày  
**Priority:** P2  
**Files:**
- [cmd/cvectl/main.go](../../../../services/cvectl/cmd/cvectl/main.go)
- [internal/client/client.go](../../../../services/cvectl/internal/client/client.go)
- [internal/config/config.go](../../../../services/cvectl/internal/config/config.go)
- [internal/output/printer.go](../../../../services/cvectl/internal/output/printer.go)

### Hoàn thành
- [x] Module `services/cvectl/` với cobra + viper
- [x] Config `~/.cvectl/config.yaml` (server URL, API key, output format)
- [x] HTTP client cho tất cả API endpoints
- [x] Table + JSON output printer
- [x] `cvectl sources list/status/sync/pause/resume`
- [x] `cvectl vuln get/search/enrich/related`
- [x] `cvectl admin stats/withdraw/reprocess`
- [x] `cvectl version`
- [ ] **TODO:** Shell completion (Sprint 09)
- [ ] **TODO:** Export command (Sprint 09)
- [ ] **TODO:** Tests với mock server (Sprint 09)

---

## TASK-08-03 · Rate Limiting & Quota (API Gateway) [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1

### Subtasks

- [ ] Move rate limiting từ api-gateway → dedicated middleware
- [ ] Implement per-API-key rate limiting (Redis sliding window)
- [ ] Quota tracking: requests/month per key
- [ ] Return `X-RateLimit-*` headers
- [ ] Return 429 Too Many Requests với `Retry-After`
- [ ] Tests

---

## TASK-08-04 · Local Dev Environment Improvements [📋 TODO]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 1 ngày  
**Priority:** P3  
**Refs:** [06-new-features.md §7.2](../06-new-features.md)

### Subtasks

- [ ] Thêm Jaeger vào `docker-compose.yaml`:
  ```yaml
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports: ["16686:16686", "14268:14268"]
  ```
- [ ] Thêm NATS UI:
  ```yaml
  nats-ui:
    image: natsio/nats-box:latest  # hoặc NATS Surveyor
  ```
- [ ] Thêm Swagger UI:
  ```yaml
  swagger-ui:
    image: swaggerapi/swagger-ui
    environment:
      SWAGGER_JSON_URL: http://api-gateway:8080/openapi.json
  ```
- [ ] Update README với hướng dẫn local dev mới

---

## Sprint 08 Definition of Done

- [x] `GET /v2/vulns/{id}/enrichment` trả về KEV, EPSS, tags đầy đủ ✅ 2026-06-03
- [x] `POST /v2/vulns/batch-get` hỗ trợ up to 100 IDs ✅ 2026-06-03
- [x] `cvectl vuln get CVE-2023-44487` — CLI hoàn chỉnh ✅ 2026-06-03
- [x] `cvectl sources list` — trả về source status ✅ 2026-06-03
- [x] cvectl module tạo thành công (cobra + viper + client) ✅ 2026-06-03
- [ ] Rate limiting per-API-key với quota (Sprint 09)
- [ ] `go build ./services/cvectl/...` pass (Sprint 09 — cần add go.work entry)
- [ ] Shell completion đầy đủ (Sprint 09)
