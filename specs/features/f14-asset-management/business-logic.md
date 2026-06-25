# F14 — Asset Management: Business Logic

> 🔵 Planned (v3.0). Mô tả theo thiết kế đã có.

---

## 1. Auto Asset Registration

Sau mỗi scan hoàn thành, hệ thống tự động upsert asset:

```
Khi scan.completed event nhận được:
    for each target in scan.targets:
        asset = find asset WHERE identifier = target AND product_id = product_id
        
        if asset not found:
            INSERT assets {
                product_id, type: detectAssetType(target),
                identifier: target,
                last_scanned_at: NOW(),
                risk_score: 0  // sẽ được tính sau
            }
        else:
            UPDATE assets SET last_scanned_at = NOW()
        
        link_findings(asset.id, scan.findings)
```

### 1.1 Asset Type Detection

```
detectAssetType(identifier):
    if isIPAddress(identifier):     return "host"
    if isURL(identifier):           return "url"
    if isHostname(identifier):      return "domain"
    if isContainerImage(identifier): return "container"
    return "unknown"
```

---

## 2. Risk Scoring Algorithm

Risk score (0–100) tổng hợp từ findings của asset:

```
calculateRiskScore(asset_id):
    // Lấy active findings của asset
    findings = SELECT f.severity, f.epss_score, f.is_kev
               FROM findings f JOIN asset_findings af ON af.finding_id = f.id
               WHERE af.asset_id = $1 AND f.state = 'Active'
    
    score = 0
    for finding in findings:
        base = severityBase(finding.severity):
            Critical: 40
            High:     20
            Medium:    8
            Low:       2
        
        // Boost nếu EPSS cao
        epss_boost = finding.epss_score * 10  // max +10
        
        // Boost nếu KEV
        kev_boost = finding.is_kev ? 15 : 0
        
        score += min(base + epss_boost + kev_boost, 40)  // cap per finding
    
    return min(score, 100)  // cap total at 100
```

### 2.1 Risk Level Interpretation

| Score | Level | Ý nghĩa |
|-------|-------|---------|
| 0–20 | Low | Ít rủi ro |
| 21–50 | Medium | Cần theo dõi |
| 51–80 | High | Cần xử lý sớm |
| 81–100 | Critical | Cần xử lý ngay |

---

## 3. Tagging

### 3.1 Mục đích

Tags giúp nhóm assets và lọc để scan hoặc report:

```
Ví dụ tags: ["production", "internet-facing", "PCI-DSS", "payment-service"]
```

### 3.2 Tag operations

```
PATCH /api/v1/assets/{id}/tags
{
    add: ["new-tag"],
    remove: ["old-tag"]
}

Logic:
    new_tags = (current_tags + add_tags) - remove_tags
    UPDATE assets SET tags = new_tags
```

---

## 4. Scheduled Re-scan

Asset có thể được cấu hình tự động re-scan định kỳ:

```
Asset config: {
    auto_scan: true,
    scan_schedule: "0 3 * * 0"  // Every Sunday 3am
    scan_type: "nmap"
}

Scheduler check:
    for each asset WHERE auto_scan=true:
        if nextCronTime(scan_schedule) <= NOW():
            trigger scan-service-ovs.CreateScan({
                targets: [asset.identifier],
                type: asset.scan_type,
                product_id: asset.product_id,
                source: "asset_scheduler"
            })
```

---

## 5. Business Rules

| Rule | Chi tiết |
|------|---------|
| Auto-register on scan | Mỗi scan target → auto-upsert asset |
| Risk score real-time | Tính lại mỗi khi finding state thay đổi |
| Tag-based scan trigger | Có thể trigger scan cho tất cả assets với tag X |
| Last seen tracking | `last_scanned_at` cập nhật sau mỗi scan |
| De-register | Asset bị xóa nếu không có scan nào trong 90 ngày (configurable) |
