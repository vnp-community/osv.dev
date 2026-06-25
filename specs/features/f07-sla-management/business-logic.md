# F07 — SLA Management: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. SLA Configuration Model

### 1.1 Hai loại SLA config

**Global default:** Áp dụng cho tất cả findings khi không có product-specific override.

**Product override:** Config riêng cho từng product, override global theo severity cụ thể.

```
getSLADays(product_id, severity):
    1. Tìm product-specific config: WHERE product_id = $product_id AND severity = $severity
    2. Nếu tìm thấy → return config.days
    3. Nếu không → return global default theo bảng:
        Critical: 7 days
        High:     30 days
        Medium:   90 days
        Low:      180 days
        Info:     null (no SLA)
```

### 1.2 Tạo product SLA config

```
POST /api/v2/sla-configs {product_id, severity, days}

Validation:
    - days > 0
    - severity in [Critical, High, Medium, Low]
    - product_id tồn tại và user có quyền admin/owner trong product
    - Nếu đã có config cho (product_id, severity) → update thay vì insert
```

---

## 2. SLA Deadline Assignment

### 2.1 Khi tạo finding

```
Ngay sau khi finding được tạo (state=Active):
    sla_days = getSLADays(finding.product_id, finding.severity)
    
    if sla_days is null:
        finding.sla_expiration_date = null  // Info findings không có SLA
    else:
        finding.sla_expiration_date = finding.created_at + sla_days (days)
    
    UPDATE finding SET sla_expiration_date
```

### 2.2 Khi finding bị reopen

```
Khi finding chuyển từ bất kỳ trạng thái nào về Active:
    Recalculate SLA:
        sla_expiration_date = NOW() + getSLADays(product_id, severity)
    
    (Không giữ SLA deadline cũ — bắt đầu đếm lại từ thời điểm reopen)
    
    UPDATE findings SET sla_expiration_date, sla_breached=false
```

---

## 3. Daily Breach Detection

### 3.1 Cron job

```
[Daily Cron — thường chạy 1am UTC]

Bước 1: Tìm findings sắp vi phạm trong 7 ngày tới (warning):
    SELECT * FROM findings
    WHERE state='Active'
      AND sla_expiration_date BETWEEN NOW() AND NOW() + 7 days
      AND sla_approaching_notified = false
    
    For each approaching finding:
        Publish NATS: finding.sla.approaching
        UPDATE sla_approaching_notified = true

Bước 2: Tìm findings đã vi phạm:
    SELECT * FROM findings
    WHERE state='Active'
      AND sla_expiration_date < NOW()
      AND sla_breached = false
    
    For each breached finding:
        UPDATE findings SET sla_breached = true
        INSERT sla_breaches (finding_id, breached_at = NOW())
        Publish NATS: finding.sla.breached
```

### 3.2 Business rules trong breach detection

| Rule | Chi tiết |
|------|---------|
| Chỉ check `state='Active'` | Mitigated/FalsePositive findings không bị check |
| `sla_breached = false` filter | Tránh duplicate events cho cùng một finding |
| Info severity | Không bao giờ breach (sla_expiration_date IS NULL) |
| Reopen resets | Khi reopen, `sla_breached` reset về `false` |

---

## 4. SLA Status Query

### 4.1 SLA summary cho product

```
GET /api/v2/products/{id}/sla-status

Query:
    SELECT
        severity,
        COUNT(*) FILTER (WHERE sla_expiration_date > NOW()) as on_track,
        COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW()+7d) as approaching,
        COUNT(*) FILTER (WHERE sla_breached=true) as breached
    FROM findings
    WHERE product_id=$1 AND state='Active'
    GROUP BY severity

Response:
{
    "summary": {
        "critical": {"on_track": 0, "approaching": 0, "breached": 2},
        "high": {"on_track": 5, "approaching": 1, "breached": 0},
        ...
    },
    "total_breached": 2,
    "oldest_breach": {finding_id, days_overdue}
}
```

---

## 5. SLA Notification Thresholds

| Event | Trigger | Notification |
|-------|---------|-------------|
| `sla.approaching` | 7 ngày trước expiry | Warning email/slack |
| `sla.breached` | Quá expiry date | Alert email/slack/webhook |
| (optional) `sla.critical_approaching` | 1 ngày trước (Critical only) | Urgent alert |
