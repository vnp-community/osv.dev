# F12 — Audit Trail & Compliance

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-010  
**Services:** `audit-service` (port 8090)  
**UI Routes:** `/admin/audit`  
**UI Components:** `AuditLogs`

---

## 1. Mô tả

Audit Trail cung cấp log bất biến (immutable) cho mọi sự kiện quan trọng trong hệ thống — từ thay đổi trạng thái finding đến hành động admin. Mỗi event được ký bằng HMAC-SHA256, lưu trong append-only store với Row-Level Security, đảm bảo không thể sửa/xóa sau khi ghi.

---

## 2. Audit Coverage

Audit-service subscribe **40+ NATS subjects** để capture toàn bộ lifecycle events:

### 2.1 Finding Events
| Event | Action logged |
|-------|--------------|
| `finding.created` | Finding creation with full data snapshot |
| `finding.status.changed` | State transition: from → to |
| `finding.batch_created` | Bulk import summary |
| `finding.sla.breached` | SLA breach detection |

### 2.2 Risk & Acceptance Events
| Event | Action logged |
|-------|--------------|
| `risk_acceptance.created` | Acceptance với findings list |
| `risk_acceptance.expired` | Auto-expiry với re-activate list |
| `risk_acceptance.revoked` | Manual revocation |

### 2.3 JIRA Events
| Event | Action logged |
|-------|--------------|
| `jira.issue.created` | JIRA ticket creation |
| `jira.issue.resolved` | JIRA resolution → finding close |

### 2.4 Auth Events
| Event | Action logged |
|-------|--------------|
| User login | IP, timestamp, success/failure |
| User logout | Session end |
| API key created | Key ID, scopes |
| API key revoked | Key ID |
| User role changed | Before/after roles |
| Failed login attempts | IP, count |

### 2.5 Admin Events
| Event | Action logged |
|-------|--------------|
| User created | User details |
| User deactivated | Admin action |
| JIRA config changed | Config ID (not credentials) |
| SLA config changed | Before/after config |
| Webhook added/removed | Webhook URL |

---

## 3. Audit Event Structure

```json
{
  "id": "ae-001",
  "action": "finding.status.changed",
  "entity_type": "finding",
  "entity_id": "finding-001",
  "before": {
    "status": "Active",
    "sla_expiration_date": "2026-06-25"
  },
  "after": {
    "status": "Mitigated",
    "mitigated_at": "2026-06-18T10:00:00Z"
  },
  "actor": {
    "user_id": "bob-001",
    "email": "bob@company.com",
    "ip_address": "192.168.1.100",
    "user_agent": "Mozilla/5.0..."
  },
  "product_id": "prod-001",
  "timestamp": "2026-06-18T10:00:00Z",
  "trace_id": "trace-abc123",
  "hmac": "sha256:a3f2b1c4..."
}
```

---

## 4. Immutability Guarantees

### 4.1 Append-Only Storage
- PostgreSQL Row-Level Security (RLS)
- `INSERT` chỉ được phép, `UPDATE` và `DELETE` bị chặn ở database level
- Revoke quyền DELETE từ application user

### 4.2 HMAC-SHA256 Signature
```
hmac = HMAC-SHA256(signing_key, json_serialize(event_without_hmac_field))
```
- Signing key từ environment variable (không hardcode)
- Cho phép verify tamper nếu cần điều tra

### 4.3 Monthly Partitioning
```sql
audit_events_2026_01
audit_events_2026_02
...
audit_events_2026_06
```
- Partitioned by `timestamp` → performance tốt khi query by date range
- Old partitions có thể archive ra cold storage

---

## 5. Audit Log Query API

```
GET /api/v2/audit-log
```

**Query Parameters:**
| Parameter | Mô tả |
|-----------|-------|
| `entity_type` | finding, product, user, webhook... |
| `entity_id` | Specific entity ID |
| `action` | Specific action type |
| `actor_id` | Filter by user |
| `product_id` | Filter by product |
| `from` | Start timestamp (RFC3339) |
| `to` | End timestamp (RFC3339) |
| `limit` | Max records (default 50, max 200) |
| `cursor` | Pagination cursor |

**Response:**
```json
{
  "events": [...],
  "total": 1250,
  "next_cursor": "ae-500"
}
```

---

## 6. UI: AuditLogs

**Route:** `/admin/audit`  
**Component:** `AuditLogs`

**Features:**
- Timeline view của audit events
- Filter panel (entity type, action, user, date range)
- Event detail expanded view
- Export to CSV
- Search by entity ID hoặc user

---

## 7. Compliance Use Cases

| Use Case | Cách sử dụng |
|----------|-------------|
| **SOC 2** | Xuất audit log cho auditors, chứng minh access control |
| **PCI-DSS** | Tracking ai thay đổi gì trong security findings |
| **ISO 27001** | Evidence cho corrective action tracking |
| **Internal investigation** | Trace sequence of events khi incident |

---

## 8. Database Schema (`osv_audit`)

| Table | Mô tả |
|-------|-------|
| `audit_events` | Append-only, partitioned by month, HMAC-signed |

**Retention:** 7 năm (configurable)  
**Archive:** Old partitions → MinIO/S3 cold storage

---

## 9. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Audit write | Async via NATS (no user blocking) |
| Audit query | < 200ms (indexed by timestamp + entity_id) |
| Immutability | Row-Level Security (database-level) |
| Retention | 7 năm mặc định |
| NATS subscriptions | 40+ subjects |
| Partition | Monthly partitioning |
