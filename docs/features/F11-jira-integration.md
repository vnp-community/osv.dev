# F11 — JIRA Bidirectional Integration

**Status:** ✅ v2.1 Implemented  
**CR References:** CR-DD-008  
**Services:** `jira-service` (port 8088)  
**Database Schema:** `osv_jira`

---

## 1. Mô tả

JIRA Integration kết nối findings trong OSV với JIRA tickets theo hai chiều: OSV tạo ticket khi finding active, JIRA thông báo OSV khi ticket resolved để tự động close finding. Credentials được bảo mật bằng AES-256-GCM encryption.

---

## 2. Bidirectional Sync Flow

### 2.1 OSV → JIRA (Finding → Ticket)

**Trigger:** Finding được tạo với severity >= High VÀ status = Active

**Flow:**
```
finding.created event (NATS)
    → jira-service receives event
    → Check product JIRA config
    → Create JIRA issue via REST API
    → Store jira_key + jira_url in finding
    → Notify: jira.issue.created (NATS)
```

**JIRA Issue Fields:**
```json
{
  "project": "SEC",
  "summary": "[OSV] CVE-2021-44228 - Log4Shell in auth-service",
  "description": "...",
  "issuetype": "Bug",
  "priority": "Critical",
  "labels": ["security", "osv-finding"],
  "customfield_cve": "CVE-2021-44228",
  "customfield_epss": "0.9754",
  "customfield_osv_finding_id": "finding-001"
}
```

### 2.2 JIRA → OSV (Ticket → Finding)

**Flow:**
```
JIRA webhook: issue.resolved
    → jira-service endpoint receives event
    → Verify HMAC-SHA256 signature
    → Find linked finding by jira_key
    → Update finding: IsMitigated = true
    → Publish: finding.status.changed (NATS)
```

**Webhook Endpoint:** `POST /api/v1/jira/webhook`  
**Verification:** HMAC-SHA256 `X-Hub-Signature`

---

## 3. JIRA Configuration

### 3.1 Per-Product JIRA Config
```json
{
  "product_id": "prod-001",
  "jira_url": "https://company.atlassian.net",
  "project_key": "SEC",
  "username": "osv-bot@company.com",
  "api_token": "[AES-256-GCM encrypted]",
  "issue_type": "Bug",
  "min_severity": "HIGH",
  "auto_create": true,
  "custom_fields": {}
}
```

### 3.2 Credentials Security
- API tokens encrypted bằng **AES-256-GCM**
- Encryption key từ environment variable
- Tokens không bao giờ xuất hiện trong logs
- Tokens không trả về qua API (write-only)

### 3.3 APIs
```
GET /api/v1/jira/configs              → List JIRA configs (tokens masked)
POST /api/v1/jira/configs             → Create JIRA config
PUT /api/v1/jira/configs/{id}         → Update config
DELETE /api/v1/jira/configs/{id}      → Remove config
POST /api/v1/jira/configs/{id}/test   → Test connection
```

---

## 4. Sync Rules

### 4.1 Create Ticket Conditions
- Finding severity: HIGH hoặc CRITICAL
- Finding status: Active
- Product có JIRA config với `auto_create = true`
- Chưa có `jira_key` (chưa có ticket)

### 4.2 Close Finding Conditions
- JIRA issue.resolved event nhận được
- HMAC signature verified
- Finding tìm thấy theo `jira_key`

### 4.3 Re-sync
- Khi finding reopened → JIRA ticket reopened (nếu config)
- Manual sync: `POST /api/v1/jira/sync/{finding_id}`

---

## 5. Finding Fields (JIRA-related)

```json
{
  "jira_key": "SEC-123",
  "jira_url": "https://company.atlassian.net/browse/SEC-123",
  "jira_status": "In Progress",
  "jira_assigned_to": "alice@company.com"
}
```

---

## 6. NATS Events

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `jira.issue.created` | jira-service | After JIRA ticket created |

**Subscribers:** notification-service (in-app notification), audit-service (log)

---

## 7. Database Schema (`osv_jira`)

| Table | Mô tả |
|-------|-------|
| `jira_configs` | Per-product JIRA config (encrypted tokens) |
| `jira_issues` | OSV finding ↔ JIRA key mapping |

---

## 8. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| JIRA API call timeout | 10 giây |
| Credentials | AES-256-GCM encrypted at rest |
| Webhook verification | HMAC-SHA256 (reject if invalid) |
| Retry on failure | 3 attempts, exponential backoff |
