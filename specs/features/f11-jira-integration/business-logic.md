# F11 — JIRA Integration: Business Logic

> Mô tả bằng ngôn ngữ tự nhiên + pseudo-code.

---

## 1. Credential Management

### 1.1 Lưu trữ an toàn

JIRA credentials (API token, password) được mã hóa trước khi lưu DB:

```
Khi save config:
    plaintext_creds = {username: "user@corp.com", api_token: "xxx"}
    encrypted_bytes = AES-256-GCM.encrypt(plaintext_creds, master_key)
    jira_configs.encrypted_creds = base64(encrypted_bytes)
    // master_key từ environment variable, không lưu DB

Khi use config:
    encrypted_bytes = base64.decode(jira_configs.encrypted_creds)
    plaintext_creds = AES-256-GCM.decrypt(encrypted_bytes, master_key)
```

### 1.2 Test connectivity

```
POST /api/v2/jira-configs/{id}/test

jira-service:
    1. Decrypt credentials
    2. GET {server_url}/rest/api/2/myself
       Headers: Authorization: Basic base64(user:token)
    3. Response 200 → return {status: "connected", user: {displayName}}
    4. Response 401 → return error "invalid credentials"
    5. Timeout → return error "connection timeout"
```

---

## 2. OSV Finding → JIRA Issue (Push)

### 2.1 Trigger

```
Hai cách trigger:
    1. Manual: POST /api/v2/findings/{id}/push-to-jira
    2. Auto: finding-service publish "finding.created" → jira-service nhận → auto-push
       (chỉ nếu product config có auto_push=true)
```

### 2.2 Create JIRA Issue

```
pushToJIRA(finding_id):
    1. Fetch finding details (finding-service)
    2. Load JIRA config cho product (jira_configs)
    3. Decrypt credentials
    4. Map fields:
        summary      = finding.title
        description  = buildDescription(finding)
        priority     = mapSeverity(finding.severity)
        duedate      = finding.sla_expiration_date
        custom_fields = {CVE_ID: finding.cve_id, component: finding.component}
    5. POST {jira_server}/rest/api/2/issue
       Body: {fields: {project, issuetype, summary, ...}}
    6. Response: {key: "SEC-123", id: "...", url: "..."}
    7. INSERT jira_issues {finding_id, jira_issue_key="SEC-123", jira_issue_url}
    8. UPDATE finding: jira_issue_key = "SEC-123"
    9. INSERT jira_sync_log {direction: "OSV→JIRA", status: "success"}

buildDescription(finding):
    """
    *CVE ID:* {finding.cve_id}
    *Severity:* {finding.severity} (CVSS: {finding.cvss3})
    *EPSS Score:* {finding.epss}
    *Known Exploited:* {finding.is_kev}
    *Component:* {finding.component_name} {finding.component_version}
    *SLA Deadline:* {finding.sla_expiration_date}
    
    *Description:*
    {finding.description}
    
    *References:*
    {finding.references.join('\n')}
    
    _Created by OSV Platform_
    """
```

---

## 3. JIRA → OSV Sync (Bidirectional)

### 3.1 Webhook Verification

Khi JIRA gửi webhook event tới `/api/v2/jira/webhook`:

```
verifyJIRAWebhook(request):
    secret = load jira_config.webhook_secret for project
    received_sig = request.header["X-Hub-Signature"]
    expected_sig = "sha256=" + HMAC-SHA256(secret, request.body_bytes)
    
    if NOT constant_time_compare(received_sig, expected_sig):
        return 401 "invalid signature"
    
    return ok
```

### 3.2 Status Sync

```
handleJIRAWebhook(event):
    if event.type != "jira:issue_updated": return
    
    jira_key = event.issue.key  // vd: "SEC-123"
    jira_status = event.issue.fields.status.name
    
    finding_id = jira_issues.find(jira_issue_key = jira_key).finding_id
    if not finding_id: return  // issue không thuộc OSV
    
    target_state = mapJIRAStatus(jira_status):
        "Done" / "Resolved" → "Mitigated"
        "Won't Fix"         → "RiskAccepted"
        "False Positive"    → "FalsePositive"
        other               → null (không update)
    
    if target_state:
        finding-service.UpdateState(finding_id, target_state, actor="jira-sync")
        INSERT jira_sync_log {direction: "JIRA→OSV", status: "success"}
```

---

## 4. Conflict Resolution

Khi cả hai phía update cùng lúc:

```
Quy tắc: Last-write-wins

Khi JIRA webhook tới:
    if finding.last_updated_by == "jira-sync": 
        skip (tránh vòng lặp)
    else:
        apply JIRA status change
```

---

## 5. Business Rules

| Rule | Chi tiết |
|------|---------|
| Credentials mã hóa | AES-256-GCM, master key từ env var |
| Webhook HMAC | Mọi incoming JIRA webhook đều phải có valid HMAC |
| SSRF protection | JIRA server URL không được là private IP |
| Auto-push | Configurable per product, mặc định off |
| No double-push | Check jira_issues trước khi tạo — một finding chỉ có 1 JIRA issue |
| Sync loop prevention | Track `last_updated_by` để tránh JIRA→OSV→JIRA vòng lặp |
