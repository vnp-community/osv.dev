# F07 — SLA Enforcement & Tracking

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-006  
**Services:** `sla-service` (port 8086)  
**UI Routes:** `/dashboard/sla`  
**UI Components:** `SLADashboard`

---

## 1. Mô tả

SLA (Service Level Agreement) Enforcement tự động tính toán deadline xử lý cho mỗi finding dựa trên severity, phát hiện breach khi deadline vượt quá, và gửi cảnh báo qua nhiều kênh. Hỗ trợ cấu hình SLA policy riêng theo từng product.

---

## 2. Default SLA Policy

| Severity | SLA Days | Mô tả |
|----------|----------|-------|
| **Critical** | 7 ngày | Must be fixed urgently |
| **High** | 30 ngày | Fix within a month |
| **Medium** | 90 ngày | Fix within a quarter |
| **Low** | 180 ngày | Fix within 6 months |
| **Info** | — | No SLA (informational only) |

**Calculation:** `sla_expiration_date = finding.created_at + sla_days`

---

## 3. Per-Product SLA Override

Security managers có thể configure SLA policy riêng cho từng product:

```json
{
  "product_id": "prod-001",
  "overrides": {
    "critical": 3,
    "high": 14,
    "medium": 30,
    "low": 90
  },
  "reason": "PCI-DSS compliance requirement"
}
```

**APIs:**
```
GET /api/v1/sla/configs              → List all SLA configs
POST /api/v1/sla/configs             → Create/update SLA config
GET /api/v1/sla/configs/{product_id} → Get product SLA config
```

---

## 4. SLA Breach Detection

### 4.1 Daily Cron Job
- Chạy mỗi ngày lúc 8:00 AM UTC
- Query tất cả active findings với `sla_expiration_date <= today`
- Publish NATS event: `finding.sla.breached` cho mỗi breach

### 4.2 Breach Event Payload
```json
{
  "finding_id": "finding-001",
  "product_id": "prod-001",
  "severity": "CRITICAL",
  "expires_at": "2026-06-15T00:00:00Z",
  "days_overdue": 3
}
```

**Subscribers:** notification-service, audit-service

### 4.3 SLA Status States
| Status | Mô tả |
|--------|-------|
| `compliant` | Còn hạn, > 3 ngày |
| `at_risk` | Còn 1–3 ngày |
| `breached` | Đã quá hạn |
| `n/a` | Severity = Info |

---

## 5. SLA Dashboard

### 5.1 API
```
GET /api/v1/dashboard/sla   → SLA metrics aggregated per product
```

### 5.2 Response
```json
{
  "products": [
    {
      "product_id": "prod-001",
      "product_name": "Payment API",
      "sla_compliance_rate": 0.85,
      "findings": {
        "total_active": 12,
        "compliant": 8,
        "at_risk": 2,
        "breached": 2
      },
      "by_severity": {
        "critical": {"breached": 1, "at_risk": 0},
        "high": {"breached": 1, "at_risk": 2}
      },
      "trend": [...]
    }
  ],
  "overall_compliance": 0.91
}
```

### 5.3 UI: SLADashboard
- Compliance rate per product
- Trend chart (SLA breach over time)
- Table: findings sắp breach (at_risk)
- Drill-down vào product → finding list

---

## 6. Finding Fields (SLA-related)

```json
{
  "sla_expiration_date": "2026-06-25T00:00:00Z",
  "sla_days_left": 7,
  "sla_status": "at_risk",
  "sla_config_id": "sla-product-001"
}
```

---

## 7. Notification Integration

Khi SLA breach phát hiện:
- **Email:** Assigned user + product owner
- **Slack:** Security channel
- **In-app:** Notification center
- **Webhook:** Configured webhooks với `sla.breached` event type

---

## 8. NATS Events

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `finding.sla.breached` | sla-service | Daily cron job |

**Subscribers:** notification-service (notify), audit-service (log)

---

## 9. Database Schema (`osv_sla`)

| Table | Mô tả |
|-------|-------|
| `sla_configs` | Global và per-product SLA policies |
| `sla_product_assignments` | Product → SLA config mapping |
| `sla_breaches` | History của các breach events |

---

## 10. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| SLA breach notification | < 5 phút sau khi phát hiện |
| Daily cron accuracy | ± 1 phút |
| SLA dashboard response | < 200ms |
