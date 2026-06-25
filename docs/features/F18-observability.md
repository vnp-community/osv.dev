# F18 — Observability (Metrics / Tracing / Logging)

**Status:** ✅ v2.2 Implemented  
**CR References:** CR-GCV-009  
**Services:** Tất cả microservices  
**Infrastructure:** Prometheus + Grafana, OpenTelemetry → Jaeger, zerolog

---

## 1. Mô tả

Observability stack toàn diện cho tất cả microservices: structured logging (zerolog), Prometheus metrics, và distributed tracing (OpenTelemetry → Jaeger). Mỗi service tự export health/ready endpoints. Đảm bảo khả năng debug, alerting, và performance monitoring.

---

## 2. Structured Logging (zerolog)

### 2.1 Log Format
Tất cả services dùng `zerolog` JSON logger:

```json
{
  "level": "info",
  "ts": "2026-06-18T10:00:00Z",
  "service": "finding-service",
  "trace_id": "abc123def456",
  "span_id": "789xyz",
  "method": "PATCH",
  "path": "/api/v1/findings/f-001",
  "status": 200,
  "latency_ms": 45,
  "user_id": "bob-001",
  "message": "Finding status updated"
}
```

### 2.2 Required Fields (mọi log line)
| Field | Mô tả |
|-------|-------|
| `level` | debug/info/warn/error |
| `ts` | RFC3339 timestamp |
| `service` | Service name |
| `trace_id` | Distributed trace ID |
| `span_id` | Current span ID |

### 2.3 HTTP Request Log Fields
| Field | Mô tả |
|-------|-------|
| `method` | HTTP method |
| `path` | Request path (sanitized) |
| `status` | HTTP status code |
| `latency_ms` | Response time |
| `user_id` | Authenticated user |
| `ip` | Client IP |

### 2.4 Log Levels Guidelines
| Level | Use case |
|-------|----------|
| `error` | Unrecoverable errors, failed operations |
| `warn` | Degraded state, retries, deprecated usage |
| `info` | State changes, important business events |
| `debug` | Detailed traces (disabled in production) |

---

## 3. Prometheus Metrics

### 3.1 Core Metrics (mọi service phải export)

```
# HTTP metrics
http_requests_total{method, path, status}           Counter
http_request_duration_seconds{method, path}         Histogram

# Database metrics
db_query_duration_seconds{query, service}           Histogram
db_pool_connections_total{state}                    Gauge

# Cache metrics
cache_hits_total{cache_name}                        Counter
cache_misses_total{cache_name}                      Counter
cache_hit_ratio{cache_name}                         Gauge

# NATS metrics
nats_messages_published_total{subject}              Counter
nats_messages_consumed_total{subject}               Counter
nats_consumer_lag{consumer_name}                    Gauge

# Business metrics (service-specific)
findings_active_total{severity, product_id}         Gauge
cves_indexed_total{source}                          Gauge
sla_breaches_total{severity}                        Counter
webhooks_delivered_total{status}                    Counter
scans_running_total{scan_type}                      Gauge
```

### 3.2 Metrics Endpoint
```
GET /metrics   → Prometheus text format (scrape endpoint)
```

### 3.3 Grafana Dashboards
- **Gateway Dashboard:** Request rate, error rate, P50/P95/P99 latency
- **Finding Service:** Finding state distribution, dedup rate
- **Data Service:** Fetcher success/failure, sync lag per source
- **SLA Dashboard:** Breach count, compliance rate over time
- **Infrastructure:** PostgreSQL pool, Redis memory, NATS lag

---

## 4. Distributed Tracing (OpenTelemetry → Jaeger)

### 4.1 Setup
- **SDK:** OpenTelemetry Go SDK
- **Exporter:** Jaeger (OTLP gRPC)
- **Propagation:** W3C TraceContext headers (HTTP) + NATS headers

### 4.2 Trace Propagation

**HTTP (inter-service):**
```
Traceparent: 00-abc123...-789xyz...-01
```

**NATS (event-driven):**
```
NATS Header: traceparent=00-abc123...-789xyz...-01
```

### 4.3 Span Types
| Span | Attributes |
|------|-----------|
| HTTP request | `http.method`, `http.url`, `http.status_code` |
| DB query | `db.system`, `db.operation`, `db.table` |
| NATS publish | `messaging.system`, `messaging.destination` |
| NATS consume | `messaging.system`, `messaging.destination`, `messaging.message_id` |
| External API | `http.url`, `net.peer.name` |

### 4.4 Sampling
- **Development:** 100% sampling
- **Production:** 10% sampling (configurable)
- **Error spans:** 100% (always sampled)

---

## 5. Health Endpoints

**Mỗi service phải implement:**

### 5.1 Liveness Check
```
GET /health
```
```json
{
  "status": "ok",
  "service": "finding-service",
  "version": "2.2.0",
  "uptime_seconds": 86400
}
```
HTTP 200 = alive, HTTP 500 = unhealthy

### 5.2 Readiness Check
```
GET /ready
```
```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "nats": "ok",
    "redis": "ok"
  }
}
```
HTTP 200 = ready to receive traffic, HTTP 503 = not ready

---

## 6. Alerting Rules (Prometheus Alertmanager)

| Alert | Condition | Severity |
|-------|-----------|---------|
| `ServiceDown` | Service `/health` returning 5xx for > 1m | Critical |
| `HighErrorRate` | `http_requests_total{status=~"5.."} > 5%` for 5m | Warning |
| `SlowAPI` | P95 latency > 1s for 5m | Warning |
| `DBPoolExhausted` | `db_pool_connections_total{state="busy"} > 90%` | Critical |
| `NATSLagHigh` | Consumer lag > 1000 messages | Warning |
| `SLABreachRate` | `sla_breaches_total` rate > 10/hour | Warning |

---

## 7. Infrastructure Stack

| Component | Port | Mô tả |
|-----------|------|-------|
| Prometheus | 9090 | Metrics collection + alerting rules |
| Grafana | 3000 | Visualization dashboards |
| Jaeger | 16686 | Distributed trace UI |
| Alertmanager | 9093 | Alert routing (Email/Slack) |

---

## 8. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Metrics scrape interval | 15 giây |
| Trace export | Async, no user latency impact |
| Log output | STDOUT (structured JSON) |
| Health check | < 10ms response |
| Metrics endpoint | < 50ms response |
| Jaeger retention | 7 ngày |
| Prometheus retention | 30 ngày |
