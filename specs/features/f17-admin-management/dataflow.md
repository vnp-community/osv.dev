# F17 — Administration: Data Flow

---

## 1. User Management Flow

```
Admin → POST /api/v1/admin/users {email, name, role, password}
    │
    ▼
identity-service:
    1. Validate admin role (X-User-Role: admin)
    2. Validate email unique
    3. Hash password (bcrypt cost=12)
    4. INSERT users
    5. Publish NATS: audit.user.created
    │
    ▼
Admin ← 201 {user_id, email, role}

Admin → PATCH /api/v1/admin/users/{id} {role: "readonly"}
    │
    ▼
identity-service:
    1. Validate self-modification guard
    2. UPDATE users SET role=readonly
    3. SET Redis: osv:user:force_logout:{id} = 1  (invalidate all tokens)
    4. Publish NATS: audit.user.role_changed
    │
    ▼
Admin ← 200 {user_id, new_role}
```

---

## 2. Manual CVE Sync Flow

```
Admin → POST /api/v1/admin/sync/nvd
    │
    ▼
data-service:
    1. Validate admin role
    2. Lookup "nvd" in fetcher registry
    3. go fetcher.FetchAndStore()  // async goroutine
    │
    ▼
Admin ← 202 {status: "sync triggered", source: "nvd"}

[In background]
    Fetcher runs → upsert MongoDB → publish NATS: ingestion.cve.synced
```

---

## 3. System Health Flow

```
Admin → GET /api/v1/admin/health
    │
    ▼
apps/osv (gateway):
    Parallel health checks:
        GET identity-service:8081/health
        GET data-service:8082/health
        GET finding-service:8085/health
        GET sla-service:8086/health
        GET notification-service:8087/health
        GET jira-service:8088/health
        GET audit-service:8090/health
        GET search-service:/health
    │
    Aggregate:
        overall = "ok" if all ok
                = "degraded" if any non-critical service down
                = "critical" if core service (identity/finding) down
    │
    ▼
Admin ← 200 {overall, services: {...}}
```

---

## 4. OpenSearch Re-index Flow

```
Admin → POST /api/v1/admin/data/reindex
    │
    ▼
search-service:
    1. CREATE new OpenSearch index "cves_v{n+1}"
    2. Stream SELECT * FROM cves (PostgreSQL) với cursor
    3. Bulk index to new OpenSearch index (batches of 500)
    4. When done:
        PUT /cves/_alias → point to new index
        DELETE old index
    │
    ▼ (async, ~15 phút cho 300K CVEs)
Admin ← 202 {status: "queued", job_id}
Admin → GET /api/v1/admin/reindex/{job_id} (polling)
```

---

## 5. NATS Events from Admin

| Event | Trigger |
|-------|---------|
| `audit.user.created` | New user created |
| `audit.user.role_changed` | Role update |
| `audit.user.deactivated` | User deactivated |
| `audit.user.force_logout` | Force logout triggered |
| `audit.apikey.revoked` | Admin revokes API key |
| `audit.config.changed` | System config change |
