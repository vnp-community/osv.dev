# F07 — SLA Management: Data Flow

---

## 1. SLA Assignment khi tạo finding

```
finding-service nhận request tạo finding mới
    │
    ▼
gọi sla-service.GetSLADays(product_id, severity)
    │
    ├── sla-service: query product-specific config
    │   └── Nếu không có → trả về global default
    │
    ▼
finding-service:
    sla_expiration_date = created_at + sla_days
    INSERT finding với sla_expiration_date
    │
    ▼
Client ← 201 {finding_id, sla_expiration_date, sla_days}
```

---

## 2. Daily Breach Check Flow

```
[Cron — sla-service]
    │
    ▼
Phase 1: Approaching warning (7 days ahead)
    SELECT findings WHERE approaching AND not yet notified
        │
        ▼
    Publish NATS: finding.sla.approaching per finding
        │
        ▼
    notification-service → Warning Email/Slack to product members

Phase 2: Breach detection
    SELECT findings WHERE state='Active' AND sla_expiration_date < NOW()
                      AND sla_breached = false
        │
        ▼
    For each:
        UPDATE findings SET sla_breached=true
        INSERT sla_breaches record
        Publish NATS: finding.sla.breached
            │
            ▼
        notification-service → Breach Alert Email/Slack/Webhook
        audit-service        → Ghi audit event sla.breach
```

---

## 3. SLA Recalculation on Reopen

```
finding-service: finding.state.changed {from: Mitigated, to: Active}
    │
    ▼
sla-service nhận NATS event:
    sla_days = getSLADays(product_id, severity)
    new_expiry = NOW() + sla_days
    UPDATE findings SET sla_expiration_date=new_expiry, sla_breached=false
```

---

## 4. SLA Config API Flow

```
Admin → POST /api/v2/sla-configs {product_id, severity, days}
    │
    ▼
sla-service:
    1. Validate product_id, severity, days
    2. UPSERT sla_configs (insert or update existing)
    3. Publish NATS: audit.sla_config.updated
    │
    ▼
Client ← 201 {config_id, product_id, severity, days}
```

---

## 5. NATS Events

| Event | Publisher | Trigger | Subscribers |
|-------|-----------|---------|------------|
| `finding.sla.approaching` | sla-service (cron) | 7 ngày trước expiry | notification-service |
| `finding.sla.breached` | sla-service (cron) | Quá expiry date | notification-service, audit-service |
| `finding.state.changed` (to Active) | finding-service | Reopen | sla-service → recalculate |
