# F05 — Finding Management: Data Flow

---

## 1. Tạo Finding Mới

```
Client → POST /api/v2/findings {cve_id, title, severity, component, test_id}
    │
    ▼
finding-service:
    1. Validate: test_id tồn tại và thuộc product mà user có quyền
    2. Tính hash_code = SHA-256(title + component + version + cve_id)
    3. Query: tìm finding có cùng hash_code trong product (state=Active)
       │
       ├── [Trùng] → đặt is_duplicate=true, state=Duplicate
       │             Publish NATS: finding.duplicate.detected
       │
       └── [Không trùng] → state=Active
    │
    4. Lấy SLA config → tính sla_expiration_date
    5. INSERT finding vào DB
    6. Publish NATS: finding.state.changed {from: null, to: Active}
    │
    ▼
audit-service nhận → ghi audit_event
    │
    ▼
sla-service nhận → schedule breach check
    │
    ▼
Client ← 201 {finding_id, state, sla_expiration_date, is_duplicate}
```

---

## 2. Chuyển Trạng Thái Finding

```
Client → POST /api/v2/findings/{id}/close
         (hoặc /reopen, /false-positive, ...)
    │
    ▼
finding-service:
    1. Fetch finding từ DB
    2. Validate transition: current_state → requested_state hợp lệ?
       → Nếu không hợp lệ: return 422
    3. UPDATE findings SET state=new_state, updated_at=NOW()
    4. INSERT finding_notes nếu có note
    5. Publish NATS: finding.state.changed {
           finding_id, product_id, severity,
           from: old_state, to: new_state,
           user_id
       }
    │
    ▼
audit-service ← finding.state.changed → INSERT audit_event (HMAC-signed)
notification-service ← finding.state.changed → notify subscribers nếu configured
    │
    ▼
Client ← 200 {finding, new_state}
```

---

## 3. SLA Breach Detection (Daily Cron)

```
[sla-service — Daily cron, thường 0am UTC]
    │
    ▼
SELECT findings WHERE state='Active' AND sla_expiration_date < NOW()
                  AND sla_breached = false
    │
    ▼
For each finding:
    UPDATE findings SET sla_breached=true
    INSERT sla_breaches (finding_id, breached_at)
    Publish NATS: finding.sla.breached {
        finding_id, severity, product_id,
        expires_at, owner_emails[]
    }
    │
    ▼
notification-service nhận:
    → Gửi Email + Slack tới product members có role user/admin
```

---

## 4. Product Grade Calculation

```
Client → GET /api/v2/products/{id}/grade
    │
    ▼
finding-service:
    SELECT severity, COUNT(*) FROM findings
    WHERE product_id=$1 AND state='Active'
    GROUP BY severity
    │
    ▼
Áp dụng grading algorithm → Grade A/B/C/D/F
    │
    ▼
Client ← 200 {grade, breakdown: {critical, high, medium, low, info, total}}
```

---

## 5. Risk Acceptance Flow

```
Client → POST /api/v2/risk-acceptances {product_id, finding_ids[], expiration_date, reason}
    │
    ▼
finding-service:
    1. Validate findings thuộc product
    2. Validate expiration_date > today
    3. INSERT risk_acceptances
    4. INSERT risk_acceptance_findings (junction)
    5. For each finding:
        UPDATE state='RiskAccepted'
        Publish NATS: finding.state.changed
    │
    ▼
Client ← 201 {risk_acceptance_id, finding_count, expiration_date}

---

[Daily Cron — sla-service]
    │
    ▼
SELECT * FROM risk_acceptances WHERE expiration_date <= TODAY AND status='active'
    │
    ▼
For each expired acceptance:
    UPDATE risk_acceptances SET status='expired'
    For each linked finding:
        UPDATE state='Active'
        Publish NATS: finding.state.changed {from: RiskAccepted, to: Active}
    Publish NATS: risk.acceptance.expired
```

---

## 6. NATS Events

### `finding.state.changed`
```
Publisher:   finding-service
Trigger:     Mọi state transition (kể cả tạo mới và risk acceptance)
Payload:     {finding_id, product_id, severity, from_state, to_state, user_id, timestamp}

Subscribers:
    audit-service        → INSERT audit_event (append-only, HMAC)
    notification-service → alert nếu product có notification config
    sla-service          → khi to_state=Active: recalculate SLA
```

### `finding.sla.breached`
```
Publisher:   sla-service (daily cron)
Trigger:     Finding Active + sla_expiration_date < NOW()
Payload:     {finding_id, product_id, severity, sla_deadline, days_overdue}

Subscribers:
    notification-service → Email/Slack alert tới product members
    audit-service        → log breach event
```

### `finding.duplicate.detected`
```
Publisher:   finding-service (on create)
Payload:     {new_finding_id, original_finding_id, hash_code}

Subscribers:
    audit-service → log dedup event
```
