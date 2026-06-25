# F18 — Observability (Metrics / Tracing / Logs)

> **Spec Folder:** `specs/features/f18-observability/`  
> **Feature Doc:** [`docs/features/F18-observability.md`](../../../docs/features/F18-observability.md)  
> **Status:** ✅ v2.0 Implemented

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Metrics taxonomy, tracing context propagation, log levels, alerting rules |
| [dataflow.md](./dataflow.md) | Metrics collection flow, distributed trace flow, log aggregation |

---

## Three Pillars

| Pillar | Tech | Storage |
|--------|------|---------|
| **Metrics** | Prometheus (pull) | Prometheus TSDB → Grafana |
| **Distributed Tracing** | OpenTelemetry + Jaeger | Jaeger in-memory / Elasticsearch |
| **Structured Logging** | Zerolog (JSON) | stdout → Loki / ELK |

---

## Services Instrumented

| Service | Metrics | Traces | Logs |
|---------|---------|--------|------|
| All services | ✅ | ✅ | ✅ |
| Gateway (apps/osv) | ✅ (HTTP req/sec, latency, error rate) | ✅ | ✅ |
| data-service | ✅ (fetch duration, CVE count) | ✅ | ✅ |
| finding-service | ✅ (finding state changes) | ✅ | ✅ |
| search-service | ✅ (search latency, embedding time) | ✅ | ✅ |

---

## Key Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | Counter | method, path, status_code, service |
| `http_request_duration_seconds` | Histogram | method, path, service |
| `cve_sync_duration_seconds` | Histogram | source (nvd, epss, ...) |
| `cve_synced_total` | Counter | source |
| `findings_created_total` | Counter | severity, product_id |
| `sla_breaches_total` | Counter | severity |
| `nats_messages_published_total` | Counter | subject |
| `nats_messages_consumed_total` | Counter | subject, consumer |
| `embedding_generation_duration_ms` | Histogram | provider |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/metrics` | Prometheus scrape endpoint (all services) |
| GET | `/health` | Health check (all services) |
| GET | `/readyz` | Readiness probe |
| GET | `/livez` | Liveness probe |
