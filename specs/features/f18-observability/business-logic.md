# F18 — Observability: Business Logic

---

## 1. Metrics Collection

### 1.1 Prometheus instrumentation

Mỗi service expose `/metrics` endpoint để Prometheus scrape mỗi 15 giây:

```
Metrics được collect tự động (middleware):
    For every HTTP request:
        http_requests_total.Inc(method, path, status_code)
        http_request_duration_seconds.Observe(duration)
    
    For database operations:
        db_query_duration_seconds.Observe(duration, query_type)
    
    For NATS operations:
        nats_messages_published_total.Inc(subject)
        nats_messages_consumed_total.Inc(subject, consumer)

Custom business metrics (manually instrumented):
    Sau mỗi fetcher sync:
        cve_synced_total.Add(count, source=fetcherName)
        cve_sync_duration_seconds.Observe(duration, source=fetcherName)
    
    Sau mỗi finding state change:
        findings_created_total.Inc(severity, product_id)  // khi created
        findings_closed_total.Inc(severity)               // khi Mitigated
        sla_breaches_total.Inc(severity)                  // khi breach detected
```

### 1.2 Metric Types Usage

| Type | Use case |
|------|---------|
| Counter | Tổng số requests, CVEs synced, errors |
| Histogram | Latency, duration (HTTP, DB, embedding) |
| Gauge | Current active goroutines, queue depth, cache hit rate |

---

## 2. Distributed Tracing

### 2.1 OpenTelemetry Context Propagation

```
Mỗi HTTP request vào gateway:
    1. Extract trace context từ headers:
       W3C Traceparent: 00-{trace_id}-{parent_span_id}-01
    2. Nếu không có → tạo root span mới
    3. Inject trace context vào mọi downstream call:
       - Header khi gọi upstream services
       - NATS message headers
    
Ví dụ trace chain:
    [gateway span]
        └── [identity-service span: validateToken]
        └── [finding-service span: getFinding]
                └── [PostgreSQL span: SELECT]
                └── [finding-service span: getRelatedCVE]
                        └── [data-service span: getCVE]
                                └── [MongoDB span: findOne]
```

### 2.2 Span Attributes (KEY)

```
Mỗi span phải có:
    http.method:        GET/POST/...
    http.url:           endpoint path
    http.status_code:   200/404/500
    db.system:          postgresql | mongodb | redis
    db.statement:       (sanitized query — no PII)
    service.name:       identity-service | finding-service | ...
    service.version:    v2.2.0
    error:              true/false
    error.message:      (nếu error)
```

---

## 3. Structured Logging

### 3.1 Log Format (JSON)

```
Mỗi log entry là JSON với fields:
{
    "level": "info",
    "service": "finding-service",
    "time": "2026-06-18T03:00:00Z",
    "trace_id": "abc123",
    "span_id": "def456",
    "request_id": "req-789",
    "msg": "finding state changed",
    "finding_id": "fnd-001",
    "from_state": "Active",
    "to_state": "Mitigated",
    "user_id": "usr-123",
    "duration_ms": 12
}
```

### 3.2 Log Levels

| Level | Sử dụng |
|-------|---------|
| `debug` | Chi tiết internal, chỉ enable trong development |
| `info` | Business events bình thường (state changes, sync completed) |
| `warn` | Recoverable errors (retry, fallback, timeout nhỏ) |
| `error` | Lỗi nghiêm trọng cần attention ngay |
| `fatal` | Service không thể start, dừng ngay |

### 3.3 Fields KHÔNG được log

```
Tuyệt đối không log:
    - Passwords (kể cả hashed)
    - JWT tokens
    - API keys
    - LDAP bind passwords
    - HMAC secrets
    - Thông tin cá nhân (PII) không cần thiết
```

---

## 4. Health Check Logic

### 4.1 Health endpoint

```
GET /health

Mỗi service kiểm tra:
    1. Database connection: ping DB
    2. NATS connection: connection.Status() == CONNECTED
    3. Redis connection (nếu dùng): PING
    4. Dependencies: GET internal dependencies /health

Response:
{
    "status": "ok" | "degraded" | "error",
    "version": "v2.2.0",
    "checks": {
        "database": "ok",
        "nats": "ok",
        "redis": "ok"
    }
}
```

### 4.2 Readiness vs Liveness

```
GET /readyz (Kubernetes readiness):
    Trả về 200 chỉ khi service SẴN SÀNG nhận traffic:
        - DB connected
        - NATS connected
        - Cache warmed (nếu cần)
    Trả về 503 nếu chưa sẵn sàng → Kubernetes không route traffic

GET /livez (Kubernetes liveness):
    Trả về 200 nếu service ĐANG SỐNG (không bị deadlock):
        - Process responsive
        - Goroutine count < threshold
    Trả về 503 → Kubernetes restart pod
```

---

## 5. Alerting Rules (Prometheus)

```
Alerts được define trong prometheus/alerts.yaml:

Alert: HighErrorRate
    condition: rate(http_requests_total{status_code=~"5.."}[5m]) / 
               rate(http_requests_total[5m]) > 0.05  // >5% error rate
    severity: critical
    action: PagerDuty + Slack alert

Alert: CVESyncLag
    condition: time() - cve_last_sync_timestamp{source="nvd"} > 14400  // >4h lag
    severity: warning
    action: Slack alert

Alert: SLABreachSpike
    condition: increase(sla_breaches_total[1h]) > 10  // >10 breaches in 1h
    severity: warning
    action: Slack alert

Alert: ServiceDown
    condition: up{job="osv-services"} == 0  // service không respond /metrics
    severity: critical
    action: PagerDuty
```
