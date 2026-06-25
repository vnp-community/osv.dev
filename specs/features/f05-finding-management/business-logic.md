# F05 — Finding Management: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Finding State Machine

### 1.1 Các trạng thái

- **Active**: Finding đang mở, cần xử lý. Trạng thái mặc định khi tạo.
- **Mitigated**: Đã vá/giải quyết. Vẫn hiển thị trong lịch sử.
- **FalsePositive**: Xác nhận không phải lỗ hổng thực sự.
- **RiskAccepted**: Rủi ro được chấp nhận có thời hạn.
- **OutOfScope**: Nằm ngoài phạm vi đánh giá.
- **Duplicate**: Trùng với finding khác trong cùng product — tự động detect khi tạo.

### 1.2 Các chuyển trạng thái hợp lệ

```
Từ Active:
    → Mitigated     (close/fix)
    → FalsePositive (mark as false)
    → RiskAccepted  (accept risk)
    → OutOfScope    (exclude)
    → Duplicate     (auto-detect, không thủ công)

Từ Mitigated / FalsePositive / RiskAccepted / OutOfScope:
    → Active        (reopen)

Từ Duplicate:
    Không có transition thủ công nào được phép
```

### 1.3 Transition Validation

```
validate_transition(current_state, new_state):
    allowed = {
        Active:        [Mitigated, FalsePositive, RiskAccepted, OutOfScope],
        Mitigated:     [Active],
        FalsePositive: [Active],
        RiskAccepted:  [Active],
        OutOfScope:    [Active],
        Duplicate:     []  // không cho phép chuyển
    }
    if new_state NOT IN allowed[current_state]:
        return error("transition not allowed")
```

---

## 2. Hash-based Deduplication

### 2.1 Thuật toán

Mỗi finding được gắn một **hash fingerprint** dựa trên các field nhận dạng:

```
hash_code = SHA-256(
    title + "|" +
    component_name + "|" +
    component_version + "|" +
    cve_id
)
```

### 2.2 Quy trình kiểm tra khi tạo finding

```
Khi tạo finding mới trong product:
    1. Tính hash_code từ title + component + cve_id
    2. Tìm finding có cùng hash_code trong cùng product, trạng thái Active
    3. Nếu tìm thấy:
        - Đặt finding mới: is_duplicate=true, duplicate_of=original.id, state=Duplicate
        - Finding mới sẽ KHÔNG active
        - Publish NATS: finding.duplicate.detected
    4. Nếu không tìm thấy:
        - Tạo finding bình thường với state=Active
```

### 2.3 Xử lý khi original bị đóng

```
Nếu original finding chuyển sang Mitigated/FalsePositive:
    Tìm tất cả duplicates của nó
    For each duplicate:
        Đánh giá lại: có cần reopen không?
        (Hiện tại: không tự động — người dùng tự quyết định)
```

---

## 3. SLA Assignment

### 3.1 Tính sla_expiration_date

```
Khi tạo finding mới:
    1. Lấy SLA config cho product (nếu có override)
    2. Nếu không có override → dùng default global SLA
    3. sla_days = getSLADays(product_id, severity)
    4. Nếu severity == "Info": sla_expiration_date = NULL
    5. Else: sla_expiration_date = created_at + sla_days
    6. Lưu vào finding.sla_expiration_date
```

### 3.2 SLA Override Priority

```
getSLADays(product_id, severity):
    1. Check product-specific SLA config: WHERE product_id = $product_id AND severity = $severity
    2. Nếu có → return config.days
    3. Else → return global default:
         Critical: 7
         High:     30
         Medium:   90
         Low:      180
         Info:     null
```

---

## 4. Daily SLA Breach Detection

```
[Daily Cron — sla-service]
    │
    ▼
Query: SELECT * FROM findings
       WHERE state = 'Active'
         AND sla_expiration_date < NOW()
         AND sla_breached = false
    │
    ▼
For each breached finding:
    1. UPDATE findings SET sla_breached = true WHERE id = finding.id
    2. INSERT sla_breaches (finding_id, breached_at = NOW())
    3. Publish NATS: finding.sla.breached {finding_id, severity, product_id, expires_at}
    │
    ▼
notification-service nhận sự kiện → gửi Email + Slack tới product members
```

---

## 5. Product Grading Algorithm

```
calculateGrade(product_id):
    // Đếm active findings theo severity
    active = SELECT severity, COUNT(*) FROM findings
             WHERE product_id=$1 AND state='Active'
             GROUP BY severity

    critical_count = active['Critical'] ?? 0
    high_count     = active['High'] ?? 0
    total_active   = SUM(all active findings)

    if critical_count >= 3 OR total_active > 20:
        return 'F'
    elif critical_count in [1, 2]:
        return 'D'
    elif critical_count == 0 AND high_count > 5:
        return 'C'
    elif critical_count == 0 AND high_count in [1..5]:
        return 'B'
    else:  // 0 Critical, 0 High
        return 'A'
```

---

## 6. Risk Acceptance

### 6.1 Tạo Risk Acceptance

```
POST /api/v2/risk-acceptances
{
    product_id, finding_ids[], expiration_date, reason, retest_date
}

Business rules:
    1. Validate: tất cả finding_ids thuộc product_id
    2. Validate: expiration_date > today
    3. INSERT risk_acceptances record
    4. INSERT risk_acceptance_findings (junction)
    5. UPDATE findings SET state='RiskAccepted' for each finding
    6. Publish NATS: finding.state.changed per finding
```

### 6.2 Expiry Auto-Reopen

```
[Daily Cron]
    │
    ▼
Query: SELECT * FROM risk_acceptances
       WHERE expiration_date <= TODAY
         AND status = 'active'
    │
    ▼
For each expired acceptance:
    1. UPDATE risk_acceptances SET status='expired'
    2. For each linked finding:
        UPDATE findings SET state='Active'
        Publish NATS: finding.state.changed {from: RiskAccepted, to: Active}
    3. Publish NATS: risk.acceptance.expired
```

---

## 7. Bulk Operations

```
POST /api/v2/findings/bulk
{
    finding_ids: [id1, id2, ...],
    action: "close" | "reopen" | "tag",
    note: "optional note"
}

Business rules:
    1. Validate: tất cả findings thuộc cùng user's accessible products
    2. Validate: action hợp lệ cho từng finding (state machine check)
    3. For each finding:
        apply_transition(finding, action)
        create audit_event
        publish NATS: finding.state.changed
    4. Return: {success_count, failed_count, errors[]}
```

---

## 8. Audit Trail (Finding-level)

Mọi thay đổi trạng thái finding đều tạo audit record:

```
Khi state change xảy ra:
    NATS.Publish("audit.finding.state_changed", {
        entity_type: "finding",
        entity_id:   finding.id,
        action:      "state_changed",
        before:      {state: old_state},
        after:       {state: new_state},
        user_id:     actor_id,
        timestamp:   now
    })
    
audit-service subscriber → INSERT audit_events (append-only, HMAC-signed)
```
