# Data Models — gateway-service

> **Service**: `services/gateway-service`  
> **Mô tả**: API Gateway trung tâm. Xác thực JWT/API Key, phân quyền (RBAC), rate limiting, reverse proxy đến các upstream microservices.  
> **Storage**: PostgreSQL (API keys), Redis (token cache, rate limit counters)  
> **Go packages**: `domain/entity`, `domain/auth`, `domain/policy`

---

## 1. APIKey (Gateway)

API key cho phép truy cập programmatic, không cần JWT session.

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | UUID |
| `key_hash` | string | No | SHA-256(plaintext_key) — không lưu raw key |
| `owner_id` | string | No | User/Org ID từ JWT claims |
| `description` | string | Yes | Nhãn mô tả, ví dụ "CI/CD pipeline" |
| `scopes` | []string | No | Permission scopes |
| `rate_limit` | int | Yes | req/min override; null = dùng global tier |
| `last_used_at` | timestamp | Yes | |
| `expires_at` | timestamp | Yes | Null = không hết hạn |
| `is_active` | bool | No | |
| `created_at` | timestamp | No | |

**Scopes hợp lệ (Gateway)**:

| Scope | Mô tả |
|-------|-------|
| `cve:read` | Đọc CVE data |
| `kev:read` | Đọc KEV catalog |
| `webhook:write` | Quản lý webhooks |
| `sync:admin` | Admin trigger sync |
| `read:all` | Full read access (wildcard) |

---

## 2. UpstreamRoute

Quy tắc routing từ path prefix đến upstream service.

| Trường | Kiểu | Mô tả |
|--------|------|-------|
| `path_prefix` | string | URL prefix để match |
| `upstream` | string | Tên upstream service |
| `skip_auth` | bool | Bỏ qua xác thực (public endpoint) |
| `required_perm` | string | Permission cần thiết; rỗng = chỉ cần authenticated |
| `use_api_key` | bool | Cho phép xác thực bằng API key |

---

## 3. Route Table (GlobalCVE Routes)

Bảng routing mặc định, khớp theo prefix dài nhất.

### v2 Routes (GlobalCVE Platform)

| Path Prefix | Upstream | Auth | Permission |
|------------|---------|------|-----------|
| `/api/v2/cves` | cve-search-service | Public | — |
| `/api/v2/kev/check` | kev-service | Public | — |
| `/api/v2/kev/stats` | kev-service | Public | — |
| `/api/v2/kev` | kev-service | Public | — |
| `/api/v2/webhooks` | notification-service | Required | — |
| `/api/v2/sync/status` | cve-sync-service | Required | — |
| `/api/v2/sync/trigger` | cve-sync-service | Required | `system:configure` |

### v1 Routes (cve-search compat)

| Path Prefix | Upstream | Auth | Permission |
|------------|---------|------|-----------|
| `/api/v1/auth` | identity-service | Public | — |
| `/api/v1/cve` | cve-service | Required | — |
| `/api/v1/browse` | browse-service | Required | — |
| `/api/v1/search` | browse-service | Required | — |
| `/api/v1/vendors` | browse-service | Required | — |
| `/api/v1/products` | browse-service | Required | — |
| `/api/v1/versions` | browse-service | Required | — |
| `/api/v1/query` | query-service | Required | — |
| `/api/v1/fulltext` | search-service | Required | — |
| `/api/v1/cwe` | taxonomy-service | Required | — |
| `/api/v1/capec` | taxonomy-service | Required | — |
| `/api/v1/ranking` | ranking-service | Required | — |
| `/api/v1/dbinfo` | info-service | Required | — |
| `/api/v1/ingest` | ingest-service | Required | `admin` |

---

## 4. Principal (Auth Context)

Authenticated identity value object (runtime, không lưu DB).  
Package: `domain/auth`

| Trường | Kiểu | Nullable | Mô tả |
|--------|------|----------|-------|
| `id` | string | No | User UUID hoặc `anon:{IP}` |
| `type` | PrincipalType | No | Loại xác thực |
| `email` | string | Yes | Từ JWT email claim |
| `org_id` | string | Yes | Từ JWT org_id claim |
| `roles` | []Role | No | Danh sách roles |
| `rate_limit_tier` | string | No | `free` \| `standard` \| `premium` \| `unlimited` \| `internal` |
| `metadata` | map[string]string | Yes | Key-value context (client name, org, v.v.) |
| `permissions` | []string | No | Flat list permissions từ identity-service ValidateToken |
| `api_key_id` | string | Yes | UUID của API key nếu type = API_KEY |

**Enums — PrincipalType**:

| Giá trị | Mô tả |
|---------|-------|
| `API_KEY` | Xác thực bằng API key |
| `OAUTH2` | Xác thực bằng OAuth2 JWT |
| `SERVICE_ACCOUNT` | Service-to-service auth |
| `ANONYMOUS` | Không có credentials |

**Enums — Role**: `reader` \| `importer` \| `admin`

---

## 5. RBAC Policy (MethodPermissions)

Permission matrix theo URL path prefix và HTTP method.  
Package: `domain/policy`

| Path Prefix | HTTP Method | Required Permission |
|------------|------------|--------------------|
| `/api/v1/scans` | POST | `scan:create` |
| `/api/v1/scans` | GET | `scan:read` |
| `/api/v1/scans` | DELETE | `scan:delete` |
| `/api/v1/assets` | POST/PUT/PATCH/DELETE | `asset:write` |
| `/api/v1/assets` | GET | `asset:read` |
| `/api/v1/cves` | * | `scan:read` |
| `/api/v1/schedules` | POST/PUT/DELETE | `scan:create` |
| `/api/v1/schedules` | GET | `scan:read` |
| `/api/v1/reports` | * | `report:download` |
| `/api/v1/notifications` | * | `system:configure` |
| `/api/v1/agents` | POST/DELETE | `asset:write` |
| `/api/v1/agents` | GET | `asset:read` |

## 6. Relationships

```
APIKey ──────────────────── User (owner_id FK)
UpstreamRoute ───────────── Upstream Service (logical)
Principal ───────────────── (runtime, không lưu DB)
APIKey ──────────────────── Principal (via key_hash lookup)
```

---

## 6. Authentication Flow

```
Request
  │
  ├─ JWT Bearer → Validate → Principal (user_id, role, scopes)
  │
  ├─ API Key Header → key_hash lookup → APIKey → Principal
  │
  └─ Skip Auth → Public endpoint (no Principal required)
        │
        └─→ Route Matching (longest prefix wins)
               │
               └─→ Permission Check (required_perm)
                     │
                     └─→ Reverse Proxy → Upstream Service
```
