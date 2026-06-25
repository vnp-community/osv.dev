# F16 — Dashboard: Data Flow

---

## 1. Dashboard Overview Load

```
Client → GET /api/v2/dashboard/overview
    │
    ▼
finding-service:
    Check Redis: osv:dashboard:{user_id}:overview
    [HIT → return cached in ~1ms]
    [MISS →]
        Parallel queries:
            Q1: total_active findings
            Q2: critical_active count
            Q3: SLA compliance rate
            Q4: products_at_risk count
        Merge → SET Redis TTL 5min
    │
    ▼
Client ← 200 {
    total_active: 247,
    critical_active: 3,
    sla_compliance_rate: 87.5,
    products_at_risk: 2,
    trend_7d: {new: 12, closed: 8}
}
```

---

## 2. Product Risk Heatmap

```
Client → GET /api/v2/dashboard/products
    │
    ▼
finding-service:
    For each accessible product (parallel):
        grade = calculateGrade(product_id)  // or cached
        findings_summary = aggregateBySeverity(product_id)
    Sort by grade (worst first)
    │
    ▼
Client ← [{
    product_id, name, grade: "F",
    critical: 2, high: 8, medium: 15,
    sla_breached: 3, last_activity: "..."
}, ...]
```

---

## 3. Cache Invalidation

```
[NATS: finding.state_changed]
    │
    ▼
finding-service (event handler):
    product_id = event.product_id
    
    // Invalidate affected caches
    Redis.DEL("osv:dashboard:*:overview")    // wildcard user caches
    Redis.DEL("osv:dashboard:*:products")
    Redis.DEL("osv:grade:{product_id}")
```

---

## 4. Trend Chart Data Flow

```
Client → GET /api/v2/dashboard/trend?days=30
    │
    ▼
finding-service:
    Check Redis: osv:dashboard:{user_id}:trend:30d
    [MISS →]
        SELECT DATE(created_at), COUNT(*) FROM findings
        WHERE product_id IN accessible AND created_at >= NOW()-30d
        GROUP BY DATE(created_at)
        
        SELECT DATE(updated_at), COUNT(*) FROM findings
        WHERE state='Mitigated' AND updated_at >= NOW()-30d
        GROUP BY DATE(updated_at)
    
    Fill gaps with 0 for days without activity
    SET Redis TTL 15min
    │
    ▼
Client ← {dates[30], created[30], closed[30], cumulative_open[30]}
```

---

## 5. CVE Intelligence Widget Flow

```
Client → GET /api/v2/dashboard/cve-intel
    │
    ▼
Gateway routes to data-service + finding-service:
    
    data-service:
        new_kev = COUNT kev_entries WHERE date_added >= NOW()-7d
        high_epss = SELECT top 10 cves WHERE epss >= 0.7
    
    finding-service:
        trending_cves = SELECT cve_id, COUNT(*) FROM findings
                        WHERE created_at >= NOW()-7d
                        GROUP BY cve_id ORDER BY count DESC LIMIT 5
    
    Merge results
    Cache TTL 1h
    │
    ▼
Client ← {new_kev_count, high_epss_cves[], trending_cves[]}
```
