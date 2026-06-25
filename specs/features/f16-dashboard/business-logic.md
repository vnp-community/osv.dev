# F16 — Dashboard: Business Logic

---

## 1. Overview KPI Calculations

### 1.1 Global KPIs

```
getOverviewKPIs(user_id):
    accessible_products = getAccessibleProducts(user_id)
    
    // Total active findings
    total_active = COUNT findings WHERE product_id IN accessible_products
                   AND state='Active'
    
    // Critical findings
    critical_active = COUNT findings WHERE state='Active' AND severity='Critical'
                      AND product_id IN accessible_products
    
    // SLA compliance rate
    total_with_sla = COUNT findings WHERE state='Active' AND sla_expiration_date IS NOT NULL
    breached = COUNT findings WHERE state='Active' AND sla_breached=true
    compliance_rate = ((total_with_sla - breached) / total_with_sla) * 100
    
    // Products at risk (grade D or F)
    at_risk_count = COUNT products WHERE grade IN ['D','F']
                    AND id IN accessible_products
    
    return {total_active, critical_active, compliance_rate, at_risk_count}
```

---

## 2. Per-Product Risk Summary

```
getProductRiskSummary(user_id):
    products = getAccessibleProducts(user_id)
    
    result = []
    for product in products:
        grade = calculateGrade(product.id)
        findings_by_severity = aggregateFindingsBySeverity(product.id)
        sla_breaches = COUNT WHERE product_id=product.id AND sla_breached=true
        
        result.append({
            product_id, product_name,
            grade,
            findings: findings_by_severity,
            sla_breaches,
            last_scan_at
        })
    
    return sorted(result, by=grade, order=[F, D, C, B, A])  // worst first
```

---

## 3. SLA Compliance Breakdown

```
getSLAStatus(user_id):
    For each severity in [Critical, High, Medium, Low]:
        total = COUNT active findings with this severity and sla_expiration_date IS NOT NULL
        breached = COUNT WHERE sla_breached=true
        approaching = COUNT WHERE sla_expiration_date BETWEEN NOW() AND NOW()+7d
        on_track = total - breached - approaching
        
        compliance_pct = (on_track / total) * 100 if total > 0 else 100
```

---

## 4. Finding Trend

```
getFindingTrend(user_id, days=30):
    end = TODAY
    start = TODAY - days
    
    // Created per day
    created_by_day = SELECT DATE(created_at), COUNT(*)
                     FROM findings
                     WHERE product_id IN accessible_products
                       AND created_at BETWEEN start AND end
                     GROUP BY DATE(created_at)
    
    // Closed per day
    closed_by_day = SELECT DATE(updated_at), COUNT(*)
                    FROM findings
                    WHERE state IN ['Mitigated', 'FalsePositive']
                      AND updated_at BETWEEN start AND end
                    GROUP BY DATE(updated_at)
    
    return {
        period_days: days,
        created: created_by_day,
        closed: closed_by_day,
        net_change: sum(created) - sum(closed)
    }
```

---

## 5. CVE Intelligence Widget

```
getCVEIntelligence():
    // New KEV this week
    new_kev_count = COUNT kev_entries WHERE date_added >= NOW()-7d
    
    // High EPSS (≥0.7) active CVEs affecting products
    high_epss_cves = SELECT cve_id, epss, severity FROM cves
                     WHERE epss_score >= 0.7 AND is_kev=false
                     AND cve_id IN (product findings' cve_ids)
                     ORDER BY epss DESC LIMIT 10
    
    // Trending: Most new findings this week
    trending = SELECT cve_id, COUNT(*) as finding_count
               FROM findings WHERE created_at >= NOW()-7d
               GROUP BY cve_id ORDER BY finding_count DESC LIMIT 5
    
    return {new_kev_count, high_epss_cves, trending_cves}
```

---

## 6. Caching Strategy

```
Mỗi dashboard endpoint check Redis trước:

key = "osv:dashboard:{user_id}:{widget_name}"

getWidget(widget_name, user_id):
    cached = Redis.GET(key)
    if cached: return deserialize(cached)
    
    data = computeWidget(user_id)  // expensive DB query
    Redis.SET(key, serialize(data), TTL=widget_ttls[widget_name])
    return data

widget_ttls = {
    "overview":   300,   // 5 phút
    "products":   300,   // 5 phút
    "sla":        300,   // 5 phút
    "trend":      900,   // 15 phút
    "cve-intel":  3600,  // 1 giờ
    "top-vulns":  1800   // 30 phút
}
```

**Cache invalidation:** Khi `finding.state_changed` NATS event → DEL relevant cache keys.
