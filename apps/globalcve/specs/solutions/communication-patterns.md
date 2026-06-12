# GlobalCVE v3.0 — Inter-Service Communication Solutions

## Vấn Đề

Trong monolith với nhiều goroutines service, cần chọn cơ chế giao tiếp phù hợp cho từng loại interaction:

| Interaction | Latency Requirement | Coupling | Solution |
|-------------|-------------------|----------|----------|
| Gateway → CVE Search (hot path) | < 1ms | Tight OK | Direct function call |
| Gateway → KEV / Notification / Sync | < 100ms | Loose preferred | HTTP proxy |
| Sync → Notification (alerts) | async (eventual) | Loose | NATS JetStream |
| KEV sync → CVE table update | async | Loose | NATS JetStream |

---

## Solution 1: Direct Function Call (Zero-latency)

### Use Case
API Gateway gọi trực tiếp CVE Search handler — đây là hot path (mọi search request).

### Implementation
```go
// gateway/service.go
type Service struct {
    cveSearchHandler *cvesearchhttp.Handler  // injected trực tiếp
}

// Trong router — không cần HTTP roundtrip
r.Get("/api/v2/cves", s.cveSearchHandler.SearchCVEs)
r.Get("/api/v2/cves/{id}", s.cveSearchHandler.GetCVE)
```

### DI Wiring (main.go)
```go
cveSearchSvc := cvesearch.New(cfg, pool, redis)
gatewaySvc := gateway.New(cfg, redis, cveSearchSvc.Handler()) // inject handler
```

### Đặc Điểm
- Zero network overhead (function call)
- Type-safe
- Không có failure case (cùng process)
- Khi tách microservice: thay bằng gRPC client

---

## Solution 2: HTTP Internal Proxy (Loose coupling)

### Use Case
Gateway → KEV Service, Notification Service, CVE Sync admin API.

### Implementation
```go
// gateway/service.go
func (s *Service) proxyTo(upstreamBase string) http.HandlerFunc {
    target, _ := url.Parse(upstreamBase)
    proxy := httputil.NewSingleHostReverseProxy(target)
    // ...
    return func(w http.ResponseWriter, r *http.Request) {
        proxy.ServeHTTP(w, r)
    }
}

// Router
r.Get("/api/v2/kev/*", s.proxyTo("http://localhost:8083"))
r.Post("/api/v2/webhooks", s.proxyTo("http://localhost:8084"))
r.Get("/api/v2/sync/status", s.proxyTo("http://localhost:8082"))
```

### Internal Port Map
| Service | Port |
|---------|------|
| CVE Search | 8081 |
| CVE Sync | 8082 |
| KEV Service | 8083 |
| Notification | 8084 |

### Đặc Điểm
- Services có thể scale ra độc lập
- Khi tách microservice: chỉ đổi URL (`localhost:8083` → `kev-service.svc.cluster.local`)
- Error handling: `502 Bad Gateway` nếu service down

---

## Solution 3: NATS JetStream Events (Async)

### Use Case
Background notifications, cache invalidation, cross-service data propagation.

### Stream Configuration
```go
// infra/nats/client.go
streams := []jetstream.StreamConfig{
    {Name: "CVE_EVENTS", Subjects: []string{"cve.>"}, MaxAge: 24h},
    {Name: "KEV_EVENTS", Subjects: []string{"kev.>"}, MaxAge: 24h},
    {Name: "ALERT_EVENTS", Subjects: []string{"alert.>"}, MaxAge: 48h},
}
```

### Event Flow
```
CVE Sync completes:
  → publish "cve.synced" {source, synced, synced_at}
  → Notification service subscribes → dispatch webhooks

KEV Service syncs:
  → publish "kev.updated" {total, inserted, new_kev_ids}
  → CVE Sync subscribes → UPDATE cves SET is_kev=TRUE WHERE id IN (new_kev_ids)

New CRITICAL CVE detected:
  → CVE Sync publish "alert.triggered" {cve_id, severity, cvss3_score}
  → Notification service → dispatch webhooks
```

### Publisher Pattern
```go
data, _ := json.Marshal(KEVUpdatedEvent{
    Total: 1100, Inserted: 5, NewKEVIDs: []string{"CVE-2024-XXXXX"},
})
nc.JS.Publish(ctx, "kev.updated", data)
```

### Subscriber Pattern
```go
consumer, _ := js.CreateOrUpdateConsumer(ctx, "KEV_EVENTS", jetstream.ConsumerConfig{
    Durable:       "cve-sync-kev-subscriber",
    FilterSubject: "kev.updated",
})
msgCtx, _ := consumer.Messages()
for {
    msg, _ := msgCtx.Next()
    // Process event: update is_kev in cves table
    msg.Ack()
}
```

### Fail-Open Design
Nếu NATS không available → app vẫn chạy, chỉ mất async events:
```go
natsClient, err := infraNATS.NewClient(ctx, cfg.NATS)
if err != nil {
    log.Warn().Err(err).Msg("NATS unavailable, events disabled")
    natsClient = nil  // services check natsClient != nil before publishing
}
```

---

## So Sánh Các Giải Pháp

| Tiêu Chí | Direct Call | HTTP Proxy | NATS JetStream |
|----------|-------------|------------|----------------|
| Latency | ~0 µs | ~100-500 µs | async |
| Coupling | Tight | Loose | Very Loose |
| Type Safety | ✅ compile-time | ❌ runtime | ❌ runtime |
| Retry | N/A | Manual | Built-in (JetStream) |
| Persistence | N/A | N/A | ✅ (disk) |
| Scale-out | Thay bằng gRPC | Đổi URL | Không đổi gì |
| Monitoring | Traces | HTTP metrics | NATS metrics |

---

*Solutions v1.0 | Communication Patterns | 2026-06-09*
