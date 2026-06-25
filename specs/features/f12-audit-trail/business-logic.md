# F12 — Audit Trail: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Append-only Model

### 1.1 Nguyên tắc

Audit log được thiết kế là **hoàn toàn bất biến**:
- Không có `UPDATE` trên bảng `audit_events`
- Không có `DELETE` (kể cả admin)
- PostgreSQL Row-Level Security ngăn chặn modify ở database layer

### 1.2 RLS Policy

```
Hai database roles:
    audit_writer: chỉ được INSERT (dùng bởi audit-service)
    audit_reader:  chỉ được SELECT (dùng bởi API handlers)

PostgreSQL policy:
    CREATE POLICY audit_insert_only ON audit_events
        FOR INSERT TO audit_writer
        WITH CHECK (true);
    
    CREATE POLICY audit_read_only ON audit_events
        FOR SELECT TO audit_reader
        USING (true);
    
    -- No UPDATE or DELETE policy = denied for all
```

---

## 2. HMAC Integrity

### 2.1 Mục đích

Mỗi audit record có HMAC-SHA256 signature để phát hiện nếu database bị tamper trực tiếp.

### 2.2 Compute HMAC

```
Khi INSERT audit_event:
    payload = {
        id:          uuid,
        event_type:  "finding.state_changed",
        entity_type: "finding",
        entity_id:   finding_id,
        action:      "state_changed",
        before:      {state: "Active"},
        after:       {state: "Mitigated"},
        user_id:     actor_id,
        timestamp:   now_utc
    }
    
    json_bytes = JSON.marshal(payload, sorted_keys=true)
    hmac = HMAC-SHA256(audit_signing_key, json_bytes)
    // audit_signing_key từ environment variable
    
    INSERT audit_events {...payload, hmac_sha256: hex(hmac)}
```

### 2.3 Verify Integrity

```
POST /api/v2/audit-log/verify {event_ids: [id1, id2, ...]}

For each event_id:
    record = SELECT * FROM audit_events WHERE id=$1
    
    // Recompute HMAC từ stored fields
    payload = extract_payload_fields(record)
    expected_hmac = HMAC-SHA256(signing_key, JSON.marshal(payload, sorted_keys=true))
    
    if expected_hmac == record.hmac_sha256:
        results.append({id, status: "valid"})
    else:
        results.append({id, status: "TAMPERED"})
        // Alert: log security incident
```

---

## 3. Event Ingestion

### 3.1 NATS Subscriber

audit-service subscribe tất cả relevant events:

```
Subjects subscribed (durable consumer):
    finding.>          // finding.state_changed, finding.created, ...
    kev.>              // kev.new
    audit.>            // audit.user.login, audit.jira.issue_created, ...
    risk.>             // risk.acceptance.expired
    report.>           // report.generated
    ingestion.>        // ingestion.cve.synced

handleEvent(event):
    audit_event = mapToAuditEvent(event)
    computeAndInsertHMAC(audit_event)
    INSERT audit_events
```

### 3.2 Event Mapping

```
mapToAuditEvent(nats_event):
    switch event.type:
        case "finding.state_changed":
            return {
                event_type:  "finding.state_changed",
                entity_type: "finding",
                entity_id:   event.finding_id,
                action:      "state_changed",
                before:      {state: event.from_state},
                after:       {state: event.to_state},
                user_id:     event.user_id,
                timestamp:   now()
            }
        case "kev.new":
            return {
                event_type:  "kev.cve_added",
                entity_type: "cve",
                entity_id:   event.cve_id,
                action:      "kev_added",
                after:       {vendor: event.vendor, is_ransomware: event.is_ransomware},
                user_id:     "system",
                timestamp:   now()
            }
        ...
```

---

## 4. Query API

### 4.1 Filters

```
GET /api/v2/audit-log
    ?event_type=finding.state_changed
    &entity_id=finding-123
    &user_id=user-456
    &from=2026-01-01
    &to=2026-06-30
    &page=1&limit=50

Query:
    SELECT * FROM audit_events
    WHERE (event_type = $1 OR $1 IS NULL)
      AND (entity_id = $2 OR $2 IS NULL)
      AND (user_id = $3 OR $3 IS NULL)
      AND timestamp BETWEEN $4 AND $5
    ORDER BY timestamp DESC
    LIMIT $limit OFFSET $offset
```

### 4.2 Authorization

```
Ai được xem audit log:
    - Admin toàn hệ thống: xem tất cả events
    - Product owner: chỉ xem events liên quan đến product của mình
    - Auditor role (nếu có): readonly toàn bộ

Query filter thêm:
    if user.role != "admin":
        AND entity_id IN (products, engagements, findings của user's products)
```

---

## 5. Business Rules

| Rule | Chi tiết |
|------|---------|
| Append-only | Không có UPDATE/DELETE, enforced bởi PostgreSQL RLS |
| HMAC signing | Mọi record đều có HMAC trước khi INSERT |
| Best-effort không fail | Lỗi write audit log không làm fail business operation |
| System events | Một số events có user_id="system" (automated processes) |
| Retention | Không có auto-deletion — audit log giữ mãi (hoặc theo compliance policy) |
