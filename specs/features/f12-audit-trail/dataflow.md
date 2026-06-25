# F12 — Audit Trail: Data Flow

---

## 1. Event Ingestion Flow

```
[Bất kỳ service nào publish NATS event]
ví dụ: finding-service publish finding.state_changed
    │
    ▼
NATS JetStream (durable consumer "audit-consumer")
    │
    ▼
audit-service subscriber:
    1. Deserialize event payload
    2. Map → audit_event schema
    3. Compute HMAC-SHA256:
        json_bytes = canonical JSON của payload
        hmac = HMAC-SHA256(signing_key, json_bytes)
    4. INSERT audit_events (via audit_writer role)
    5. ACK NATS message
```

---

## 2. Audit Log Query Flow

```
Client → GET /api/v2/audit-log
    ?event_type=finding.state_changed&entity_id=finding-123
    │
    ▼
audit-service (audit_reader role):
    Validate user permissions (admin or product member)
    Build query with filters
    SELECT * FROM audit_events WHERE ...
    ORDER BY timestamp DESC LIMIT 50
    │
    ▼
Client ← 200 {events: [...], total, page}
```

---

## 3. Integrity Verification Flow

```
Client → POST /api/v2/audit-log/verify {event_ids: ["evt-1", "evt-2"]}
    │
    ▼
audit-service:
    For each event_id:
        1. SELECT * FROM audit_events WHERE id=$1
        2. Extract payload fields (không dùng stored hmac)
        3. Recompute: expected_hmac = HMAC-SHA256(key, canonical_json)
        4. Compare: expected_hmac == record.hmac_sha256?
           [MATCH] → {id, status: "valid"}
           [MISMATCH] → {id, status: "TAMPERED"} + alert
    │
    ▼
Client ← 200 {results: [{id, status}, ...], tampered_count}
```

---

## 4. Events Consumed (40+ types)

| NATS Subject | Event Type Recorded |
|-------------|---------------------|
| `finding.state_changed` | `finding.state_changed` |
| `finding.duplicate.detected` | `finding.duplicate_detected` |
| `finding.sla.breached` | `sla.breach_recorded` |
| `kev.new` | `kev.cve_added` |
| `audit.user.login` | `user.login` |
| `audit.user.locked` | `user.locked` |
| `audit.apikey.created` | `api_key.created` |
| `audit.apikey.revoked` | `api_key.revoked` |
| `audit.jira.issue_created` | `jira.issue_created` |
| `audit.jira.sync_failed` | `jira.sync_failed` |
| `audit.product.created` | `product.created` |
| `audit.product.member_added` | `product.member_added` |
| `risk.acceptance.expired` | `risk_acceptance.expired` |
| `report.generated` | `report.generated` |
| `ingestion.cve.synced` | `cve.synced` |
| ... | (40+ total) |
