# F11 — JIRA Integration: Data Flow

---

## 1. Push Finding to JIRA

```
Client → POST /api/v2/findings/{id}/push-to-jira
    │
    ▼
jira-service:
    1. Fetch finding từ finding-service
    2. Load jira_config cho product
    3. AES-256-GCM decrypt credentials
    4. Build JIRA issue payload (field mapping)
    │
    ▼
POST {jira_server}/rest/api/2/issue
    Headers: Authorization: Basic {base64(user:token)}
    Body: {fields: {summary, description, priority, duedate, ...}}
    │
    ├── [Success] → response {key: "SEC-123"}
    │   INSERT jira_issues {finding_id, jira_issue_key="SEC-123"}
    │   UPDATE finding.jira_issue_key = "SEC-123"
    │   INSERT jira_sync_log {direction: OSV→JIRA, status: success}
    │   Publish NATS: audit.jira.issue_created
    │
    └── [Error 401/403] → return error "JIRA auth failed"
        [Error 4xx] → return error with JIRA response
        [Timeout] → retry once after 3s, then fail
    │
    ▼
Client ← 200 {jira_issue_key, jira_issue_url}
```

---

## 2. JIRA Webhook → OSV State Update

```
JIRA Server → POST /api/v2/jira/webhook
    Headers: X-Hub-Signature: sha256={HMAC}
    Body: {event: "jira:issue_updated", issue: {key, fields: {status: {name}}}}
    │
    ▼
jira-service:
    1. Verify HMAC signature (constant-time compare)
       → [Invalid] → 401
    │
    2. Extract jira_key, new_status
    3. Lookup: finding_id = jira_issues WHERE jira_issue_key = jira_key
       → [Not found] → 200 (ignore, not OSV issue)
    │
    4. Map JIRA status → OSV state
    5. finding-service.UpdateState(finding_id, new_state)
    6. INSERT jira_sync_log {direction: JIRA→OSV, status: success}
    │
    ▼
JIRA ← 200 OK
```

---

## 3. Auto-push on Finding Create

```
[Nếu product config: auto_push_to_jira = true]

finding-service publish NATS: finding.state.changed {to: Active}
    │
    ▼
jira-service nhận event:
    Check: product.jira_config.auto_push = true?
    Check: finding NOT already in jira_issues?
    → [yes] → execute push flow (như mục 1)
    → [no]  → skip
```

---

## 4. NATS Events

| Event | Publisher | Trigger |
|-------|-----------|---------|
| `audit.jira.issue_created` | jira-service | JIRA issue created from OSV finding |
| `audit.jira.sync_failed` | jira-service | Sync attempt failed |
| `jira.sync.failed` | jira-service | → notification-service → alert admins |
