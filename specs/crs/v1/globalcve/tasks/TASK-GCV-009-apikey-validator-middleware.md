# TASK-GCV-009 — API Key Validator + Auth Middleware

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-009 |
| **Service** | `gateway-service` |
| **CR** | CR-GCV-008 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-008 |

## Context

Tạo `APIKeyValidator` (Redis-backed cache + PostgreSQL lookup). Cập nhật auth middleware hiện có để hỗ trợ `X-API-Key` header và `Authorization: ApiKey <key>` — cộng thêm JWT (hiện có). Cả 3 auth methods đều inject `Claims` vào context.

## Reference

- Solution: [SOL-GCV-008](../solutions/SOL-GCV-008-api-gateway-enhancement.md) §2.2
- CR: [CR-GCV-008](../CR-GCV-008-api-gateway-enhancement.md) §3.2

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/auth/apikey_validator.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/infra/postgres/apikey_pg.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/internal/auth/
        (tìm file middleware.go hoặc osv_middleware.go, update để thêm API key auth)
```

**Lưu ý**: Đọc `gateway-service/internal/auth/` để xác định cấu trúc auth hiện có trước khi sửa.

## Implementation Spec

### auth/apikey_validator.go

```go
// Package auth — API Key validator with Redis caching.
package auth

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/osv/gateway-service/internal/domain/entity"
    "github.com/osv/gateway-service/internal/domain/repository"
)

const apiKeyCacheTTL = 5 * time.Minute

// APIKeyValidator validates API keys using Redis cache + PostgreSQL.
type APIKeyValidator struct {
    repo  repository.APIKeyRepository
    cache *redis.Client
}

// NewAPIKeyValidator creates a new validator.
func NewAPIKeyValidator(repo repository.APIKeyRepository, cache *redis.Client) *APIKeyValidator {
    return &APIKeyValidator{repo: repo, cache: cache}
}

// Claims contains authenticated identity from either JWT or API key.
type APIKeyClaims struct {
    UserID   string   `json:"user_id"`
    Scopes   []string `json:"scopes"`
    AuthType string   `json:"auth_type"` // "jwt" | "api_key"
    KeyID    string   `json:"key_id,omitempty"`
}

var (
    ErrInvalidAPIKey = errors.New("invalid or inactive api key")
    ErrExpiredAPIKey = errors.New("api key has expired")
)

// Validate looks up an API key and returns claims.
// Priority: Redis cache → PostgreSQL.
func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*APIKeyClaims, error) {
    hash := sha256Hex(rawKey)
    cacheKey := "apikey:" + hash

    // 1. Redis cache (5 minute TTL)
    if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        var claims APIKeyClaims
        if err := json.Unmarshal(cached, &claims); err == nil {
            return &claims, nil
        }
    }

    // 2. DB lookup
    key, err := v.repo.FindByHash(ctx, hash)
    if err != nil {
        return nil, ErrInvalidAPIKey
    }
    if !key.IsActive {
        return nil, ErrInvalidAPIKey
    }
    if key.IsExpired() {
        return nil, ErrExpiredAPIKey
    }

    // 3. Async update last_used_at
    go v.repo.UpdateLastUsed(context.Background(), key.ID) //nolint:errcheck

    claims := &APIKeyClaims{
        UserID:   key.OwnerID,
        Scopes:   key.Scopes,
        AuthType: "api_key",
        KeyID:    key.ID,
    }

    // 4. Cache for 5 minutes
    if data, err := json.Marshal(claims); err == nil {
        v.cache.Set(ctx, cacheKey, data, apiKeyCacheTTL) //nolint:errcheck
    }

    return claims, nil
}

// sha256Hex returns the hex-encoded SHA-256 hash of s.
func sha256Hex(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}
```

### infra/postgres/apikey_pg.go

```go
// Package postgres — PostgreSQL implementation of APIKeyRepository.
package postgres

import (
    "context"
    "database/sql"
    "time"

    "github.com/jmoiern/sqlx"
    "github.com/lib/pq"
    entity "github.com/osv/gateway-service/internal/domain/entity"
    "github.com/osv/gateway-service/internal/domain/repository"
)

type pgAPIKeyRepository struct {
    db *sqlx.DB
}

func NewAPIKeyRepository(db *sqlx.DB) repository.APIKeyRepository {
    return &pgAPIKeyRepository{db: db}
}

func (r *pgAPIKeyRepository) FindByHash(ctx context.Context, hash string) (*entity.APIKey, error) {
    var row struct {
        ID          string         `db:"id"`
        OwnerID     string         `db:"owner_id"`
        Scopes      pq.StringArray `db:"scopes"`
        RateLimit   sql.NullInt64  `db:"rate_limit"`
        ExpiresAt   sql.NullTime   `db:"expires_at"`
        IsActive    bool           `db:"is_active"`
    }
    err := r.db.GetContext(ctx, &row, `
        SELECT id, owner_id, scopes, rate_limit, expires_at, is_active
        FROM api_keys
        WHERE key_hash = $1
    `, hash)
    if err == sql.ErrNoRows {
        return nil, repository.ErrAPIKeyNotFound
    }
    if err != nil {
        return nil, err
    }

    key := &entity.APIKey{
        ID:       row.ID,
        KeyHash:  hash,
        OwnerID:  row.OwnerID,
        Scopes:   []string(row.Scopes),
        IsActive: row.IsActive,
    }
    if row.RateLimit.Valid {
        rl := int(row.RateLimit.Int64)
        key.RateLimit = &rl
    }
    if row.ExpiresAt.Valid {
        key.ExpiresAt = &row.ExpiresAt.Time
    }
    return key, nil
}

func (r *pgAPIKeyRepository) Save(ctx context.Context, key *entity.APIKey) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO api_keys (id, key_hash, owner_id, description, scopes, rate_limit, expires_at, is_active, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, key.ID, key.KeyHash, key.OwnerID, key.Description,
        pq.Array(key.Scopes), key.RateLimit, key.ExpiresAt, key.IsActive, key.CreatedAt)
    return err
}

func (r *pgAPIKeyRepository) ListByOwner(ctx context.Context, ownerID string) ([]*entity.APIKey, error) {
    var rows []struct {
        ID          string         `db:"id"`
        Description string         `db:"description"`
        Scopes      pq.StringArray `db:"scopes"`
        ExpiresAt   sql.NullTime   `db:"expires_at"`
        LastUsedAt  sql.NullTime   `db:"last_used_at"`
        CreatedAt   time.Time      `db:"created_at"`
    }
    err := r.db.SelectContext(ctx, &rows, `
        SELECT id, description, scopes, expires_at, last_used_at, created_at
        FROM api_keys
        WHERE owner_id = $1 AND is_active = TRUE
        ORDER BY created_at DESC
    `, ownerID)
    if err != nil {
        return nil, err
    }

    keys := make([]*entity.APIKey, len(rows))
    for i, row := range rows {
        keys[i] = &entity.APIKey{
            ID:          row.ID,
            OwnerID:     ownerID,
            Description: row.Description,
            Scopes:      []string(row.Scopes),
            IsActive:    true,
            CreatedAt:   row.CreatedAt,
        }
        if row.ExpiresAt.Valid { keys[i].ExpiresAt = &row.ExpiresAt.Time }
        if row.LastUsedAt.Valid { keys[i].LastUsedAt = &row.LastUsedAt.Time }
    }
    return keys, nil
}

func (r *pgAPIKeyRepository) Revoke(ctx context.Context, id, ownerID string) error {
    res, err := r.db.ExecContext(ctx,
        "UPDATE api_keys SET is_active = FALSE WHERE id = $1 AND owner_id = $2", id, ownerID)
    if err != nil { return err }
    n, _ := res.RowsAffected()
    if n == 0 { return repository.ErrAPIKeyNotFound }
    return nil
}

func (r *pgAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1", id)
    return err
}
```

### Middleware Update

Tìm auth middleware trong gateway-service (có thể là `internal/auth/osv_middleware.go` hoặc tương đương), thêm API key check:

```go
// Sau JWT check, thêm:

// 2. X-API-Key header
if claims == nil {
    if rawKey := r.Header.Get("X-API-Key"); rawKey != "" {
        if apiClaims, err := apiKeyValidator.Validate(r.Context(), rawKey); err == nil {
            // Convert APIKeyClaims to existing Claims struct
            claims = convertAPIKeyClaims(apiClaims)
        } else if errors.Is(err, auth.ErrExpiredAPIKey) {
            respondJSON(w, 401, map[string]string{"error": "API key has expired."})
            return
        }
    }
}

// 3. Authorization: ApiKey <key>
if claims == nil {
    if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "ApiKey ") {
        rawKey := strings.TrimPrefix(authHeader, "ApiKey ")
        if apiClaims, err := apiKeyValidator.Validate(r.Context(), rawKey); err == nil {
            claims = convertAPIKeyClaims(apiClaims)
        }
    }
}
```

## Acceptance Criteria

- [x] `X-API-Key: gcve_validkey` → request authenticated, claims injected vào context
- [x] `Authorization: ApiKey gcve_validkey` → cũng authenticated
- [x] Invalid key → không inject claims (downstream middleware handle 401)
- [x] Expired key → 401 với `{"error":"API key has expired."}`
- [x] Inactive key → 401 với `{"error":"Authentication credentials were not provided."}`
- [x] Valid key → Redis cache populated (second request không hit PostgreSQL)
- [x] `UpdateLastUsed` gọi async (không block request)
- [x] JWT auth (Bearer token) vẫn hoạt động như cũ (không bị ảnh hưởng)
- [x] `go build ./...` pass không lỗi
