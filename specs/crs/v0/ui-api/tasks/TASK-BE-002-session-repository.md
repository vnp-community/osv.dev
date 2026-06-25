# TASK-BE-002 — identity-service: Sessions Table + Repository

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-002 |
| **Service** | `services/identity-service` |
| **Solution Ref** | [SOL-UI-001 §2.6, §3](../solutions/SOL-UI-001-auth-service-extension.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 2h |
| **Status** | ✅ DONE |

---

## Context

identity-service hiện có `users`, `api_keys` tables trong `osv_identity` schema nhưng chưa có `sessions` table để lưu refresh tokens. Refresh token rotation (token family tracking) cần bảng này.

---

## Goal

1. Tạo SQL migration `sessions` table
2. Implement `PostgresSessionRepository` với các methods cần thiết cho TASK-BE-001

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/identity-service/db/migrations/003_add_sessions.sql` |
| CREATE | `services/identity-service/internal/infra/postgres/session_repo.go` |
| CREATE | `services/identity-service/internal/domain/session.go` |

---

## Implementation

### File 1: `services/identity-service/db/migrations/003_add_sessions.sql`

```sql
-- +migrate Up
CREATE TABLE IF NOT EXISTS sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256(refresh_token) hex
    token_family        UUID NOT NULL,                 -- for reuse detection
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked             BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id       ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash    ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_family        ON sessions(token_family);
CREATE INDEX idx_sessions_expires_active ON sessions(expires_at)
    WHERE revoked = FALSE;

-- +migrate Down
DROP TABLE IF EXISTS sessions;
```

### File 2: `services/identity-service/internal/domain/session.go`

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a user's refresh token session
type Session struct {
	ID                UUID
	UserID            uuid.UUID
	RefreshTokenHash  string    // SHA-256 hex of plaintext refresh token
	TokenFamily       uuid.UUID // For token family reuse detection
	ExpiresAt         time.Time
	Revoked           bool
	RevokedAt         *time.Time
	CreatedAt         time.Time
}

func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

func (s *Session) IsValid() bool {
	return !s.Revoked && !s.IsExpired()
}
```

### File 3: `services/identity-service/internal/infra/postgres/session_repo.go`

```go
package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-org/osv/services/identity-service/internal/domain"
)

const refreshTokenTTL = 7 * 24 * time.Hour

type PostgresSessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// CreateSession generates a new refresh token, stores its hash, returns plaintext token
func (r *PostgresSessionRepository) CreateSession(ctx context.Context, userID uuid.UUID) (string, error) {
	// Generate 32 random bytes → base64url plaintext token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	plaintext := hex.EncodeToString(raw) // 64-char hex string

	// Hash for storage
	hash := sha256Hex(plaintext)

	_, err := r.db.Exec(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, token_family, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, hash, uuid.New(), time.Now().UTC().Add(refreshTokenTTL))

	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return plaintext, nil
}

// ValidateRefreshToken finds and validates a session by plaintext token
func (r *PostgresSessionRepository) ValidateRefreshToken(ctx context.Context, plaintext string) (*domain.Session, error) {
	hash := sha256Hex(plaintext)

	var s domain.Session
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, token_family, expires_at, revoked, revoked_at, created_at
		FROM sessions
		WHERE refresh_token_hash = $1
	`, hash).Scan(
		&s.ID, &s.UserID, &s.RefreshTokenHash, &s.TokenFamily,
		&s.ExpiresAt, &s.Revoked, &s.RevokedAt, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidToken
		}
		return nil, fmt.Errorf("query session: %w", err)
	}

	// Check if already revoked (reuse detection)
	if s.Revoked {
		return &s, domain.ErrTokenReused
	}

	if s.IsExpired() {
		return nil, domain.ErrTokenExpired
	}

	return &s, nil
}

// RevokeSession marks a specific session as revoked
func (r *PostgresSessionRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE sessions
		SET revoked = TRUE, revoked_at = $1
		WHERE id = $2
	`, now, sessionID)
	return err
}

// RevokeByToken revokes a session by its plaintext token hash
func (r *PostgresSessionRepository) RevokeByToken(ctx context.Context, plaintext string) error {
	hash := sha256Hex(plaintext)
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE sessions
		SET revoked = TRUE, revoked_at = $1
		WHERE refresh_token_hash = $2
	`, now, hash)
	return err
}

// RevokeFamilyByToken revokes all sessions in the same token family (reuse attack mitigation)
func (r *PostgresSessionRepository) RevokeFamilyByToken(ctx context.Context, plaintext string) error {
	hash := sha256Hex(plaintext)
	now := time.Now().UTC()

	// Find token family
	var family uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT token_family FROM sessions WHERE refresh_token_hash = $1
	`, hash).Scan(&family)
	if err != nil {
		return fmt.Errorf("find family: %w", err)
	}

	// Revoke all sessions in family
	_, err = r.db.Exec(ctx, `
		UPDATE sessions
		SET revoked = TRUE, revoked_at = $1
		WHERE token_family = $2 AND revoked = FALSE
	`, now, family)
	return err
}

// CleanupExpiredSessions deletes sessions expired > 24h ago (run as periodic job)
func (r *PostgresSessionRepository) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	result, err := r.db.Exec(ctx, `
		DELETE FROM sessions
		WHERE expires_at < NOW() - INTERVAL '1 day'
		   OR (revoked = TRUE AND revoked_at < NOW() - INTERVAL '1 day')
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// sha256Hex returns hex-encoded SHA-256 of input
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
```

---

## Verification

```bash
cd services/identity-service

# Apply migration (assuming golang-migrate or similar)
migrate -path db/migrations -database "$POSTGRES_DSN" up

# Verify table created
psql "$POSTGRES_DSN" -c "\d sessions"

# Build
go build ./internal/infra/postgres/...

# Unit test (mock DB)
go test ./internal/infra/postgres/... -v -run TestSession
```

**Expected:**
```
Table "public.sessions"
 id | refresh_token_hash | token_family | expires_at | revoked | ...
```

---

## Checklist

- [x] `003_add_sessions.sql` migration tạo table với đúng columns + indexes
- [x] `session.go` domain entity với `IsValid()` và `IsExpired()` methods
- [x] `session_repo.go` implement đủ 5 methods: CreateSession, ValidateRefreshToken, RevokeSession, RevokeByToken, RevokeFamilyByToken
- [x] `sha256Hex` helper dùng `crypto/sha256` (không phải MD5 hay SHA1)
- [x] `CreateSession` generate 32 random bytes via `crypto/rand` (không `math/rand`)
- [x] `ValidateRefreshToken` return `ErrTokenReused` nếu session đã bị revoke (reuse detection)
- [x] Migration có both Up và Down sections
- [x] `go build ./...` thành công

## Notes for AI

- Module path thực tế là gì? Check `go.mod` của `identity-service` và thay `github.com/your-org/osv` cho đúng
- Nếu project dùng `database/sql` thay vì `pgx/v5`, adapt query methods tương ứng
- `CleanupExpiredSessions` nên được gọi từ goroutine trong `main.go` mỗi 24h: `time.NewTicker(24 * time.Hour)`
