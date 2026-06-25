# F17 — Administration: Business Logic

---

## 1. User Management

### 1.1 Create User (Admin)

```
POST /api/v1/admin/users {email, name, role, password?, ldap_user?}

Business rules:
    1. Admin only (X-User-Role: admin check)
    2. Email phải unique
    3. Nếu ldap_user=true: không cần password (auth qua LDAP)
    4. Nếu local user: password phải đủ mạnh (min 12 chars, có số và ký tự đặc biệt)
    5. Hash password với bcrypt (cost=12)
    6. INSERT users {email, name, role, active=true}
    7. Publish NATS: audit.user.created
```

### 1.2 Change Role

```
PATCH /api/v1/admin/users/{id} {role: "readonly"}

Business rules:
    1. Không thể thay đổi role của chính mình (admin tự hạ quyền)
    2. Không thể thay đổi role của admin account cuối cùng
    3. UPDATE users SET role=$1 WHERE id=$2
    4. Invalidate tất cả JWT tokens của user đó:
       SET Redis: osv:user:force_logout:{user_id} TTL {max_token_ttl}
       → Mọi JWT tiếp theo của user sẽ bị check và reject
```

### 1.3 Deactivate User

```
DELETE /api/v1/admin/users/{id}

Rules:
    1. Không xóa thật — chỉ set active=false (soft delete)
    2. Revoke tất cả active sessions
    3. Revoke tất cả API keys của user
    4. Force logout: SET Redis invalidation flag
    5. UPDATE users SET active=false, deactivated_at=NOW()
```

---

## 2. LDAP Configuration

### 2.1 Config Structure

```
LDAP Config fields:
    host:           "ldap.company.com"
    port:           636 (LDAPS) or 389 (LDAP)
    use_tls:        true/false
    bind_dn:        "cn=service-account,dc=company,dc=com"
    bind_password:  encrypted (AES-256-GCM)
    base_dn:        "dc=company,dc=com"
    user_filter:    "(&(objectClass=person)(mail={email}))"
    group_base_dn:  "ou=groups,dc=company,dc=com"
    group_mapping:  {
        "cn=security-admins,...": "admin",
        "cn=security-team,...":   "user",
        "cn=readonly,...":        "readonly"
    }
```

### 2.2 Test Connection

```
POST /api/v1/admin/ldap/{id}/test

Steps:
    1. Decrypt bind_password
    2. LDAP Connect to host:port (TLS/STARTTLS nếu configured)
    3. LDAP Bind với bind_dn + bind_password
    4. LDAP Search test: search base_dn với filter "(&(objectClass=*))" limit 1
    5. Return: {status: "success", server_info: {version, naming_contexts}}
```

---

## 3. CVE Source Management

### 3.1 Trigger Manual Sync

```
POST /api/v1/admin/sync/{source}
    source: "nvd" | "circl" | "epss" | "kev" | "cpe" | ...

data-service nhận request:
    1. Find fetcher in registry: registry.Get(source)
    2. if not found: return 404
    3. Run fetcher in goroutine (không block request):
        go fetcher.FetchAndStore()
    4. Return 202 {message: "sync triggered for {source}"}
```

### 3.2 Enable/Disable Fetcher

```
PATCH /api/v1/admin/sync/{source}/toggle {enabled: false}

data-service:
    UPDATE fetcher_config SET enabled=$1 WHERE name=$2
    if !enabled:
        Remove from active scheduler (stop ticker)
    if enabled:
        Re-add to scheduler (start ticker)
```

---

## 4. System Health

```
GET /api/v1/admin/health

Aggregate health checks từ all services:
    For each service:
        GET {service_internal_url}/health
        Check response: {status: "ok", db: "ok", nats: "ok", ...}
    
    Response:
    {
        overall: "degraded",  // ok | degraded | critical
        services: {
            identity:     {status: "ok", db: "ok", latency_ms: 2},
            data:         {status: "ok", db: "ok", nats: "ok", fetchers: {nvd: "ok", ...}},
            finding:      {status: "ok", db: "ok"},
            notification: {status: "degraded", smtp: "timeout"},
            ...
        }
    }
```

---

## 5. Data Management

### 5.1 Trigger Re-index

```
POST /api/v1/admin/data/reindex

search-service:
    1. Disable write alias, enable write to new index
    2. Stream all CVEs từ PostgreSQL
    3. Bulk index to new OpenSearch index
    4. Swap aliases
    5. Return: {status: "queued", estimated_duration: "~15min"}
```

### 5.2 Purge Old Data

```
POST /api/v1/admin/data/purge {type: "findings", older_than_days: 365}

Rules:
    1. Chỉ purge findings có state Mitigated/FalsePositive/Duplicate
    2. Tạo audit event trước khi purge
    3. Soft delete: UPDATE SET deleted_at=NOW() (không xóa thật)
    4. Return count
```
