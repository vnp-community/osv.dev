# TASK-GCV-008 — API Key Domain + DB Migration (gateway-service)

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-008 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | — |

## Context

Tạo domain entity `APIKey` và DB migration cho bảng `api_keys` trong `gateway-service`. Đây là foundation cho TASK-GCV-009 (validator) và TASK-GCV-012 (CRUD API).

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.1
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §3.1, §9

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/domain/entity/apikey.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/domain/repository/apikey_repo.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/migrations/XXXX_api_keys.sql
```

**Lưu ý**: Kiểm tra xem `gateway-service/internal/domain/` đã tồn tại chưa; nếu chưa, tạo cấu trúc directory.

## Implementation Spec

### entity/apikey.go

```go
// Package entity — API Key domain entity for gateway-service.
package entity

import "time"

// APIKey represents an issued API key for programmatic access.
// Plain key text is NEVER stored — only the SHA-256 hash.
type APIKey struct {
    ID          string     // UUID
    KeyHash     string     // SHA-256(plaintext_key) — stored, never plain key
    OwnerID     string     // User or org ID (from JWT claims)
    Description string     // Human-readable label, e.g. "CI/CD pipeline"
    Scopes      []string   // Permission scopes: ["cve:read", "webhook:write"]
    RateLimit   *int       // req/min override; nil = use global tier default
    LastUsedAt  *time.Time
    ExpiresAt   *time.Time // nil = no expiry
    IsActive    bool
    CreatedAt   time.Time
}

// API Key scope constants.
const (
    ScopeCVERead   = "cve:read"
    ScopeKEVRead   = "kev:read"
    ScopeWebhook   = "webhook:write"
    ScopeSyncAdmin = "sync:admin"
    ScopeReadAll   = "read:all"
)

// IsExpired reports whether the key has passed its expiry time.
func (k *APIKey) IsExpired() bool {
    if k.ExpiresAt == nil {
        return false
    }
    return k.ExpiresAt.Before(time.Now())
}

// HasScope reports whether the key has the specified scope.
func (k *APIKey) HasScope(scope string) bool {
    for _, s := range k.Scopes {
        if s == scope || s == ScopeReadAll {
            return true
        }
    }
    return false
}
```

### repository/apikey_repo.go

```go
// Package repository — API Key repository interface.
package repository

import (
    "context"
    "github.com/osv/gateway-service/internal/domain/entity"
)

// ErrAPIKeyNotFound is returned when a key lookup returns no result.
var ErrAPIKeyNotFound = errors.New("api key not found")

// APIKeyRepository defines persistence operations for API keys.
type APIKeyRepository interface {
    // FindByHash looks up an API key by its SHA-256 hash.
    // Returns ErrAPIKeyNotFound if no active key matches.
    FindByHash(ctx context.Context, hash string) (*entity.APIKey, error)

    // Save persists a new API key.
    Save(ctx context.Context, key *entity.APIKey) error

    // ListByOwner returns all active API keys for an owner (no plain key).
    ListByOwner(ctx context.Context, ownerID string) ([]*entity.APIKey, error)

    // Revoke soft-deletes (is_active=false) an API key.
    Revoke(ctx context.Context, id, ownerID string) error

    // UpdateLastUsed sets last_used_at = NOW() asynchronously.
    UpdateLastUsed(ctx context.Context, id string) error
}
```

### Migration SQL

```sql
-- Migration: create api_keys table for gateway-service API key authentication
-- Up

CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT        PRIMARY KEY,
    key_hash        TEXT        NOT NULL UNIQUE,   -- SHA-256(plaintext_key)
    owner_id        TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    scopes          TEXT[]      NOT NULL DEFAULT '{}',
    rate_limit      INT         DEFAULT NULL,      -- nil = use global default
    last_used_at    TIMESTAMPTZ DEFAULT NULL,
    expires_at      TIMESTAMPTZ DEFAULT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash
    ON api_keys(key_hash);

CREATE INDEX IF NOT EXISTS idx_api_keys_owner
    ON api_keys(owner_id)
    WHERE is_active = TRUE;

-- Down (rollback)
-- DROP TABLE IF EXISTS api_keys;
```

## Acceptance Criteria

- [x] `entity.APIKey` struct tồn tại với tất cả fields: ID, KeyHash, OwnerID, Description, Scopes, RateLimit, LastUsedAt, ExpiresAt, IsActive, CreatedAt
- [x] `APIKey.IsExpired()` trả `true` nếu `ExpiresAt` đã qua
- [x] `APIKey.HasScope("read:all")` trả `true` cho mọi scope check khi scope là `read:all`
- [x] `APIKeyRepository` interface có methods: FindByHash, Save, ListByOwner, Revoke, UpdateLastUsed
- [x] Migration tạo bảng `api_keys` với constraints đúng (UNIQUE key_hash, NOT NULL fields)
- [x] Migration idempotent (`IF NOT EXISTS`)
- [x] `go build ./...` pass không lỗi
