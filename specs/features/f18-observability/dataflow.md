# F18 — Observability: Data Flow

---

## 1. Metrics Collection Flow

```
[Every HTTP Request]
    │
    ▼
HTTP Middleware (all services):
    timer.Start()
    next.ServeHTTP()
    duration = timer.Stop()
    
    http_requests_total.Inc(method, path, status)
    http_request_duration_seconds.Observe(duration)
    │
    ▼
Prometheus scrape (mỗi 15s):
    GET {service}:PORT/metrics
    → Store in Prometheus TSDB
    │
    ▼
Grafana Dashboard:
    Query Prometheus → render charts:
        - Request rate per service
        - P50/P95/P99 latency heatmap
        - Error rate by endpoint
        - CVE sync lag gauge
```

---

## 2. Distributed Trace Flow

```
Client → GET /api/v2/cves/{id}
    │
    ▼ gateway (apps/osv)
    [Span: gateway.request]
    Extract Traceparent header OR generate new trace_id
    
    Auth middleware:
        → data-service (validate token)
        [Child Span: identity.validateToken] {duration: 2ms}
    
    Route to data-service:
        [Child Span: data.getCVE] {duration: 15ms}
            → MongoDB.findOne
            [Child Span: mongo.findOne] {duration: 8ms}
    
    Collect results
    [Root Span completed: total_duration: 20ms]
    │
    ▼
Jaeger exporter (OTLP gRPC) → Jaeger backend
Trace visible in Jaeger UI:
    Timeline: gateway → identity → data → mongo
    With durations and attributes at each span
```

---

## 3. Log Aggregation Flow

```
All services write JSON logs to stdout
    │
    ▼
Docker/Kubernetes collects stdout
    │
    ▼ (logging pipeline, one of:)
    Option A: Promtail → Loki → Grafana
        Query: {service="finding-service"} |= "ERROR"
        
    Option B: Filebeat → Elasticsearch → Kibana
        Index: osv-logs-2026.06.18
        Dashboard: Error rate, slow queries, state changes
```

---

## 4. Business Metric Flow Example (CVE Sync)

```
[data-service: NVD fetcher completes]
    │
    ▼
PublishingFetcher records:
    cve_sync_started (Gauge: set to 1)
    
    sync_duration = timer.Stop()
    cve_sync_duration_seconds.Observe(sync_duration, source="nvd")
    cve_synced_total.Add(count, source="nvd")
    cve_last_sync_timestamp.Set(now.Unix(), source="nvd")
    
    cve_sync_started (Gauge: set to 0)
    │
    ▼
Prometheus scrapes → Grafana dashboard updates:
    "NVD Last Sync: 2 minutes ago" ✅
    "CVEs Synced Today: 1,247"
```

---

## 5. Alert Flow

```
Prometheus evaluates alert rules (mỗi 1 phút):
    │
    ▼
Alert fires: "CVESyncLag: NVD last sync >4h ago"
    │
    ▼
Alertmanager nhận:
    Route by severity:
        Critical → PagerDuty + Slack #alerts-critical
        Warning  → Slack #alerts-warning
    │
    ▼
On-call engineer nhận alert → investigate
```

---

## 6. Kubernetes Health Probe Flow

```
Kubernetes → GET {pod}:PORT/readyz  (every 10s)
    │
    ├── finding-service:
    │   DB ping OK → NATS connected → return 200
    │   [All OK] → Kubernetes routes traffic to this pod
    │
    └── [DB timeout] → return 503
        Kubernetes: remove pod from load balancer
        "Pod not ready" — restart if persists

Kubernetes → GET {pod}:PORT/livez  (every 30s)
    │
    ├── Responsive → goroutine count < 10,000 → return 200
    └── Deadlocked/no response → Kubernetes kills and restarts pod
```
