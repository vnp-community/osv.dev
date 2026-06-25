# TASK-P1-001 — Wire Agent PostgreSQL Repository cho scan-service

**Bug:** MOCK-007  
**Priority:** 🔴 P1 — Data Correctness  
**Effort:** ~2 giờ  
**Service:** `scan-service`  
**Loại thay đổi:** New files + DB migration + Refactor handler + Wire embedded.go

---

## Mục tiêu

`AgentHandler.RegisterAgent()` hiện tại là fake implementation — agent được tạo với random UUID nhưng không lưu vào DB. `GET /api/v1/agents` luôn trả `[]`. Cần wire PostgreSQL repository thực sự.

---

## Preconditions

- [ ] Đọc `services/scan-service/internal/delivery/http/agent_handler.go` — hiểu cấu trúc hiện tại
- [ ] Đọc `services/scan-service/embedded.go` — xem cách khởi tạo
- [ ] Kiểm tra thư mục `services/scan-service/internal/domain/` — xem entities đã có
- [ ] Kiểm tra `services/scan-service/internal/infra/postgres/` — xem convention repo
- [ ] Xác định module name: `grep -r "^module " services/scan-service/go.mod`

---

## Steps

### Step 1 — Kiểm tra domain entity Agent đã tồn tại chưa

```bash
find services/scan-service/internal/domain -name "agent*"
grep -r "type Agent struct" services/scan-service/
```

Nếu chưa có, tạo file mới:

**File mới**: `services/scan-service/internal/domain/agent.go`

```go
package domain

import (
    "context"
    "time"
    "github.com/google/uuid"
)

type AgentStatus string

const (
    AgentStatusInactive AgentStatus = "inactive"
    AgentStatusActive   AgentStatus = "active"
    AgentStatusOffline  AgentStatus = "offline"
)

type Agent struct {
    ID          uuid.UUID
    Name        string
    Hostname    string
    IPAddress   string
    OS          string
    Tags        []string
    APIKeyHash  string      // SHA-256 hex — không lưu plaintext
    Status      AgentStatus
    CreatedAt   time.Time
    LastSeenAt  *time.Time
}

type AgentRepository interface {
    Create(ctx context.Context, agent *Agent) error
    FindByID(ctx context.Context, id uuid.UUID) (*Agent, error)
    FindByAPIKeyHash(ctx context.Context, hash string) (*Agent, error)
    List(ctx context.Context) ([]*Agent, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status AgentStatus) error
    UpdateLastSeen(ctx context.Context, id uuid.UUID) error
}
```

### Step 2 — Tạo DB migration

Kiểm tra thư mục migrations của scan-service:
```bash
find services/scan-service -name "*.sql" -path "*/migrations/*" | head -5
```

Tạo migration file theo convention đang có:

**File mới**: (đặt theo convention, ví dụ `services/scan-service/internal/infra/postgres/migrations/002_add_scan_agents.sql`)

```sql
CREATE TABLE IF NOT EXISTS scan_agents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    hostname     VARCHAR(255),
    ip_address   VARCHAR(45),
    os           VARCHAR(50),
    tags         TEXT[] DEFAULT '{}',
    api_key_hash CHAR(64) NOT NULL UNIQUE,
    status       VARCHAR(20) DEFAULT 'inactive' CHECK (status IN ('inactive','active','offline')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_scan_agents_status ON scan_agents(status);
CREATE INDEX IF NOT EXISTS idx_scan_agents_api_key_hash ON scan_agents(api_key_hash);
```

### Step 3 — Tạo AgentRepo PostgreSQL

Kiểm tra convention của các repo khác trong scan-service:
```bash
ls services/scan-service/internal/infra/postgres/
cat services/scan-service/internal/infra/postgres/scan_repo.go 2>/dev/null | head -60
```

**File mới**: `services/scan-service/internal/infra/postgres/agent_repo.go`

```go
package postgres

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "<module>/internal/domain"
)

type AgentRepo struct {
    db *pgxpool.Pool
}

func NewAgentRepo(db *pgxpool.Pool) *AgentRepo {
    return &AgentRepo{db: db}
}

func (r *AgentRepo) Create(ctx context.Context, agent *domain.Agent) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO scan_agents (id, name, hostname, ip_address, os, tags, api_key_hash, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, agent.ID, agent.Name, agent.Hostname, agent.IPAddress,
       agent.OS, agent.Tags, string(agent.APIKeyHash), string(agent.Status), agent.CreatedAt)
    return err
}

func (r *AgentRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
    row := r.db.QueryRow(ctx, `
        SELECT id, name, hostname, ip_address, os, tags, api_key_hash,
               status, created_at, last_seen_at
        FROM scan_agents WHERE id = $1
    `, id)
    return scanAgent(row)
}

func (r *AgentRepo) FindByAPIKeyHash(ctx context.Context, hash string) (*domain.Agent, error) {
    row := r.db.QueryRow(ctx, `
        SELECT id, name, hostname, ip_address, os, tags, api_key_hash,
               status, created_at, last_seen_at
        FROM scan_agents WHERE api_key_hash = $1
    `, hash)
    return scanAgent(row)
}

func (r *AgentRepo) List(ctx context.Context) ([]*domain.Agent, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, name, hostname, ip_address, os, tags, api_key_hash,
               status, created_at, last_seen_at
        FROM scan_agents ORDER BY created_at DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var agents []*domain.Agent
    for rows.Next() {
        a, err := scanAgent(rows)
        if err != nil {
            continue
        }
        agents = append(agents, a)
    }
    return agents, rows.Err()
}

func (r *AgentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.AgentStatus) error {
    _, err := r.db.Exec(ctx,
        `UPDATE scan_agents SET status = $2 WHERE id = $1`, id, string(status))
    return err
}

func (r *AgentRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
    now := time.Now().UTC()
    _, err := r.db.Exec(ctx,
        `UPDATE scan_agents SET last_seen_at = $2, status = 'active' WHERE id = $1`, id, now)
    return err
}

// scanAgent scans a row into an Agent struct.
// Works with both pgx.Row and pgx.Rows.
func scanAgent(row interface{ Scan(...any) error }) (*domain.Agent, error) {
    a := &domain.Agent{}
    var status string
    err := row.Scan(
        &a.ID, &a.Name, &a.Hostname, &a.IPAddress,
        &a.OS, &a.Tags, &a.APIKeyHash, &status,
        &a.CreatedAt, &a.LastSeenAt,
    )
    if err != nil {
        return nil, err
    }
    a.Status = domain.AgentStatus(status)
    return a, nil
}
```

### Step 4 — Refactor AgentHandler để dùng real repo

Mở `services/scan-service/internal/delivery/http/agent_handler.go`.

Sửa `RegisterAgent()` để:
1. Thêm nil-check cho `h.agentRepo` → trả 503 nếu nil
2. Gọi `h.agentRepo.Create(...)` để lưu vào DB
3. Giữ logic generate `rawKey` + hash nhưng lưu `keyHash` vào DB

Sửa `ListAgents()` để:
1. Gọi `h.agentRepo.List(...)` thực sự
2. Nil-check: nếu repo nil → trả `{"agents":[],"count":0}`

> **Quan trọng**: Đọc kỹ cấu trúc handler hiện tại trước khi sửa. Giữ nguyên format JSON response.

**Helper sha256Hex** (thêm vào file nếu chưa có):
```go
import "crypto/sha256"
import "encoding/hex"

func sha256Hex(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}
```

### Step 5 — Wire trong embedded.go

Mở `services/scan-service/embedded.go`.

Tìm dòng:
```go
agentHandler := httpdelivery.NewAgentHandler(nil, logger)
```

Sửa thành:
```go
agentRepo := postgres.NewAgentRepo(pool)
agentHandler := httpdelivery.NewAgentHandler(agentRepo, logger)
```

---

## Acceptance Criteria

- [ ] `POST /api/v1/agents` → agent được lưu vào bảng `scan_agents` với `api_key_hash` (không phải plaintext)
- [ ] `GET /api/v1/agents` → trả danh sách agents thực từ DB
- [ ] Response `POST /api/v1/agents` vẫn có `api_key` (plaintext — one-time only)
- [ ] Sau khi restart service, agents đã đăng ký vẫn còn trong `GET /api/v1/agents`
- [ ] `go build ./services/scan-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/scan-service/...
go vet ./services/scan-service/...
go test ./services/scan-service/internal/... -v -run Agent

# Manual test (nếu service đang chạy)
curl -X POST http://localhost:8084/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","hostname":"host01","ip_address":"192.168.1.10","os":"linux"}'
# Expect: 201 với api_key

curl http://localhost:8084/api/v1/agents
# Expect: {"agents":[{id, name, ...}], "count": 1}
```
