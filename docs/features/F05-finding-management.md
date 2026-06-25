# F05 — Finding Lifecycle Management

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-003, CR-DD-004, CR-DD-005, CR-DD-010  
**Services:** `finding-service` (port 8085), `audit-service` (port 8090)  
**UI Routes:** `/findings`, `/findings/:id`, `/findings/risk-acceptance`  
**UI Components:** `FindingsList`, `FindingDetail`, `RiskAcceptanceCenter`

---

## 1. Mô tả

Finding Management cung cấp vòng đời đầy đủ cho việc quản lý các phát hiện lỗ hổng bảo mật (findings) — từ khi phát hiện đến khi được xử lý. Bao gồm: state machine 6 trạng thái, deduplication tự động, risk acceptance, và audit trail bất biến.

---

## 2. Finding State Machine

### 2.1 Các Trạng thái

```
          ┌──────────────────────────────────────────────┐
          ▼                                              │
       [Active] ──────────────────────────────► [Mitigated]
          │                                              │
          ├──────────────────────────────► [FalsePositive]
          │                                              │
          ├──────────────────────────────► [RiskAccepted]
          │                                              │
          ├──────────────────────────────► [OutOfScope]
          │
          └── (auto) ────────────────────► [Duplicate]
```

**Priority ordering (khi conflict):**
```
Duplicate > FalsePositive > OutOfScope > RiskAccepted > Mitigated > Active
```

### 2.2 State Transitions

| Từ | Đến | Điều kiện |
|----|-----|-----------|
| `Active` | `Mitigated` | User close/fix finding |
| `Active` | `FalsePositive` | User mark as FP với comment |
| `Active` | `RiskAccepted` | Risk acceptance record created |
| `Active` | `OutOfScope` | User mark out-of-scope |
| Any (non-Duplicate) | `Active` | User reopen finding |
| `Active` | `Duplicate` | Auto-detected by dedup system |

**Invalid transitions → HTTP 409 `INVALID_TRANSITION`**

### 2.3 APIs
```
PATCH /api/v1/findings/{id}              → Update state + comment
POST /api/v1/findings/bulk/reopen        → Bulk reopen
POST /api/v1/findings/bulk/assign        → Bulk assign owner
GET /api/v1/findings/stats               → Finding statistics by status/severity
```

---

## 3. Hash-Based Deduplication

### 3.1 Algorithm
```go
HashCode = SHA-256(title + component_name + component_version + cve_id)
```

### 3.2 Dedup Logic
Khi tạo finding mới:
1. Tính HashCode
2. Check existing hash trong cùng Product
3. Nếu tìm thấy duplicate:
   - `duplicate = true`
   - `active = false`
   - `duplicate_finding_id = <original_id>`
4. Nếu không: tạo finding mới với `active = true`

### 3.3 3 Dedup Algorithms
- **Exact match:** SHA-256 hash đầy đủ
- **CVE-based:** Cùng CVE ID + product scope
- **Similarity-based:** Title similarity > 85%

---

## 4. Risk Acceptance

### 4.1 Tổng quan
Cho phép security manager chấp nhận rủi ro cho một hoặc nhiều findings trong khoảng thời gian nhất định.

### 4.2 Risk Acceptance Entity
```json
{
  "id": "ra-001",
  "product_id": "prod-001",
  "findings": ["finding-001", "finding-002"],
  "expiration_date": "2026-12-31",
  "reason": "Compensating controls in place",
  "retest_date": "2026-09-01",
  "accepted_by": "carol@company.com",
  "created_at": "2026-06-18"
}
```

### 4.3 Auto-Expiry Workflow
Khi `expiration_date` đến:
1. NATS event `risk_acceptance.expired` published
2. Linked findings tự động chuyển về `Active`
3. Notification gửi đến responsible team

### 4.4 APIs
```
GET /api/v2/risk-acceptances           → List với expiry status
POST /api/v2/risk-acceptances          → Create new acceptance
GET /api/v2/risk-acceptances/{id}      → Detail
DELETE /api/v2/risk-acceptances/{id}   → Revoke acceptance
```

---

## 5. Finding Notes & Comments

**API:**
```
POST /api/v1/findings/{id}/notes       → Add note/comment
GET /api/v1/findings/{id}/notes        → Get all notes (ordered by time)
```

**Note fields:** `body`, `author`, `created_at`, `is_private`

---

## 6. Bulk Finding Operations

**APIs:**
```
POST /api/v2/findings/bulk             → Bulk close/reopen/tag
POST /api/v1/findings/bulk/reopen      → Bulk reopen
POST /api/v1/findings/bulk/assign      → Bulk assign to user
```

**Request:**
```json
{
  "finding_ids": ["f-001", "f-002", "f-003"],
  "action": "close",
  "comment": "Fixed in v2.1.0"
}
```

---

## 7. Finding Fields Schema

```json
{
  "id": "finding-001",
  "title": "Log4Shell RCE in authentication service",
  "cve_id": "CVE-2021-44228",
  "severity": "CRITICAL",
  "status": "Active",
  "component_name": "log4j-core",
  "component_version": "2.14.1",
  "product_id": "prod-001",
  "engagement_id": "eng-001",
  "test_id": "test-001",
  "is_kev": true,
  "has_exploit": true,
  "epss_score": 0.9754,
  "sla_expiration_date": "2026-06-25",
  "sla_days_left": 7,
  "duplicate": false,
  "duplicate_finding_id": null,
  "jira_key": "SEC-123",
  "jira_url": "https://jira.company.com/SEC-123",
  "assigned_to": "bob@company.com",
  "created_at": "2026-06-18T08:00:00Z",
  "updated_at": "2026-06-18T09:00:00Z"
}
```

---

## 8. Audit Trail

Mọi state change của finding đều được ghi vào `audit-service`:

```json
{
  "action": "finding.status.changed",
  "entity_type": "finding",
  "entity_id": "finding-001",
  "before": {"status": "Active"},
  "after": {"status": "Mitigated"},
  "user_id": "bob-001",
  "timestamp": "2026-06-18T10:00:00Z",
  "hmac": "sha256:..."
}
```

**Properties:**
- Append-only (Row-Level Security)
- HMAC-SHA256 per event (tamper-evident)
- Partitioned by month cho performance

---

## 9. NATS Events

| Event | Publisher | Subscribers |
|-------|-----------|------------|
| `finding.created` | finding-service | notification-service, sla-service |
| `finding.status.changed` | finding-service | notification-service, jira-service, audit-service, sla-service |
| `risk_acceptance.expired` | finding-service | notification-service, audit-service |

---

## 10. Database Schema (`osv_finding`)

| Table | Mô tả |
|-------|-------|
| `findings` | Core finding data + state + hashes |
| `finding_notes` | Comments và notes per finding |
| `finding_groups` | Grouping related findings |
| `risk_acceptances` | Risk acceptance records |
| `report_runs` | Report generation history |

---

## 11. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Finding state change | < 100ms |
| Dedup check | < 50ms per finding |
| Audit log write | Async via NATS (no user impact) |
| Bulk operation (100 findings) | < 2 giây |
