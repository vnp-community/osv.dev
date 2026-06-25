# SOL-SEED-005: Giải pháp thực thi — Assets & Scan Seed

> **CR:** [SEED-005-assets-scan-seed.md](../SEED-005-assets-scan-seed.md)  
> **Cập nhật:** 2026-06-18  
> **Domain:** `services/asset-service` + `services/scan-service` + `apps/osv` (gateway)  
> **Priority:** 🟠 HIGH

---

## 1. Phân tích kiến trúc hiện tại

Theo `01-architecture.md §3.13`, `asset-service` (`:8091`):
- Asset model: `{id, name, type, address (INET), os, services[], tags[] (GIN), risk_score, last_seen_at}`
- Schema: `osv_asset` — bảng `assets` (INET type, GIN tags index)
- **Hiện tại**: Assets được tạo **tự động** bởi `asset-service` khi nhận NATS event `scan.scan.completed` từ scan-service.
- Architecture note: `asset-service → SCANS.completed (upsert assets)` là NATS consumer.

Giải pháp: Thêm **write API trực tiếp** cho phép client tạo assets thủ công mà không cần scan, tái dùng cùng upsert logic.

Theo `01-architecture.md §3.6`, `scan-service`:
- Agent submit: `POST /api/v1/agents/report` (đã có trong code)
- Scheduled scans: `internal/scheduler/` + `delivery/http/schedule/`
- Gateway **chưa route** scheduled scan endpoints.

---

## 2. Các thay đổi cần thực hiện

### 2.1 Domain Layer — `services/asset-service/internal/domain/`

**File**: `internal/domain/entity/asset.go` (NEW — hiện service dùng entity từ scan-service)

```go
package entity

// Asset là network host/device được quản lý
type Asset struct {
    ID          uuid.UUID
    IPAddress   string       // INET type → string representation
    Hostname    string
    OS          string
    MACAddress  string
    Services    []Service
    Tags        []string
    Labels      map[string]string
    RiskScore   float64
    FindingCount int
    Status      string      // active | inactive | decommissioned
    LastSeenAt  *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Service là một port/service đang chạy trên asset
type Service struct {
    Port     int
    Protocol string  // tcp | udp
    Name     string  // http, https, ssh, postgresql...
    Product  string  // nginx, openssh...
    Version  string
}

// AssetCreateInput là input để tạo asset thủ công
type AssetCreateInput struct {
    IPAddress  string
    Hostname   string
    OS         string
    MACAddress string
    Services   []Service
    Tags       []string
    Labels     map[string]string
}

// AssetFilter là bộ lọc cho danh sách assets (mở rộng từ hiện tại)
type AssetFilter struct {
    Tag     string
    OS      string
    Query   string         // full-text trên ip, hostname
    Status  string
    HasPort *int
    Page    int
    Limit   int
}
```

**File**: `internal/domain/repository/asset_repository.go` (NEW)

```go
type AssetRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Asset, error)
    FindByIP(ctx context.Context, ip string) (*entity.Asset, error)
    FindAll(ctx context.Context, filter entity.AssetFilter) ([]*entity.Asset, int, error)
    Create(ctx context.Context, asset *entity.Asset) error
    Upsert(ctx context.Context, asset *entity.Asset) error  // ON CONFLICT (ip_address) DO UPDATE
    CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]BulkAssetResult, error)
    Update(ctx context.Context, asset *entity.Asset) error
    Delete(ctx context.Context, id uuid.UUID) error
    GetUniqueTags(ctx context.Context) ([]string, error)
    AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error
}

type BulkAssetResult struct {
    IPAddress string
    Status    string  // "created" | "updated" | "error"
    ID        *uuid.UUID
    Message   string
}
```

---

### 2.2 Use Case Layer — `services/asset-service/internal/usecase/`

#### Mở rộng `asset/tagging_risk.go` và tạo thêm

**File**: `internal/usecase/asset/asset_crud.go` (NEW)

```go
package asset

type AssetCRUDUseCase struct {
    repo      AssetRepository
    eventPub  events.Publisher
}

// Create tạo asset thủ công
func (uc *AssetCRUDUseCase) Create(ctx context.Context, actorID uuid.UUID, in entity.AssetCreateInput) (*entity.Asset, error) {
    asset := &entity.Asset{
        ID:         uuid.New(),
        IPAddress:  in.IPAddress,
        Hostname:   in.Hostname,
        OS:         in.OS,
        MACAddress: in.MACAddress,
        Services:   in.Services,
        Tags:       dedupStrings(in.Tags),
        Labels:     in.Labels,
        Status:     "active",
        CreatedAt:  time.Now().UTC(),
        UpdatedAt:  time.Now().UTC(),
    }
    if err := uc.repo.Create(ctx, asset); err != nil {
        return nil, err
    }
    uc.eventPub.Publish("asset.created", map[string]any{"asset_id": asset.ID, "ip": asset.IPAddress})
    return asset, nil
}

// BulkCreate tạo nhiều assets
func (uc *AssetCRUDUseCase) BulkCreate(ctx context.Context, inputs []entity.AssetCreateInput, updateExisting bool) ([]BulkAssetResult, error) {
    assets := make([]*entity.Asset, 0, len(inputs))
    for _, in := range inputs {
        assets = append(assets, mapInputToAsset(in))
    }
    results, err := uc.repo.CreateBulk(ctx, assets, updateExisting)
    if err != nil {
        return nil, err
    }
    // Publish batch event
    uc.eventPub.Publish("asset.batch_created", map[string]any{"count": len(results)})
    return results, nil
}

// ImportFromFile parse file và gọi BulkCreate
func (uc *AssetCRUDUseCase) ImportFromFile(ctx context.Context, r io.Reader, format string, updateExisting bool) (ImportAssetResult, error)

// Delete xóa asset
func (uc *AssetCRUDUseCase) Delete(ctx context.Context, id uuid.UUID) error

// AddVulnerabilities inject vulnerabilities thủ công
func (uc *AssetCRUDUseCase) AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error
```

---

### 2.3 Adapter Layer — HTTP Handlers (asset-service)

**File**: `internal/delivery/http/handlers.go` — Mở rộng:

```go
// Thêm vào Router():
r.Post("/assets",               h.CreateAsset)
r.Post("/assets/bulk",          h.CreateBulkAssets)   // literal TRƯỚC /assets/{id}
r.Post("/assets/import",        h.ImportAssets)
r.Delete("/assets/{id}",        h.DeleteAsset)
r.Post("/assets/{id}/vulnerabilities", h.AddVulnerabilities)

// Handlers:

func (h *Handler) CreateAsset(w http.ResponseWriter, r *http.Request) {
    var in entity.AssetCreateInput
    json.NewDecoder(r.Body).Decode(&in)
    asset, err := h.crudUC.Create(r.Context(), actorID(r), in)
    writeJSON(w, 201, asset)
}

func (h *Handler) CreateBulkAssets(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Assets         []entity.AssetCreateInput `json:"assets"`
        UpdateExisting bool                       `json:"update_existing"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    results, _ := h.crudUC.BulkCreate(r.Context(), req.Assets, req.UpdateExisting)
    
    created := countByStatus(results, "created")
    updated := countByStatus(results, "updated")
    writeJSON(w, 207, map[string]any{
        "created_count": created,
        "updated_count": updated,
        "results":       results,
    })
}

func (h *Handler) ImportAssets(w http.ResponseWriter, r *http.Request) {
    // Multipart parse → ImportFromFile
}

func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
    id, _ := uuid.Parse(chi.URLParam(r, "id"))
    h.crudUC.Delete(r.Context(), id)
    w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddVulnerabilities(w http.ResponseWriter, r *http.Request) {
    id, _ := uuid.Parse(chi.URLParam(r, "id"))
    var req struct {
        Vulnerabilities []entity.Vulnerability `json:"vulnerabilities"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    h.crudUC.AddVulnerabilities(r.Context(), id, req.Vulnerabilities)
    writeJSON(w, 201, map[string]any{"asset_id": id, "added_count": len(req.Vulnerabilities)})
}
```

---

### 2.4 Infra Layer — PostgreSQL (asset-service)

**File**: `internal/infra/postgres/asset_repo.go` (NEW — hiện chưa có)

```go
func (r *assetRepo) Create(ctx context.Context, a *entity.Asset) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO osv_asset.assets
            (id, ip_address, hostname, os, mac_address, services, tags, labels, status, created_at, updated_at)
        VALUES ($1, $2::inet, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, a.ID, a.IPAddress, a.Hostname, a.OS, a.MACAddress,
       marshalJSON(a.Services), pq.Array(a.Tags), marshalJSON(a.Labels),
       a.Status, a.CreatedAt, a.UpdatedAt)
    return err
}

func (r *assetRepo) CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]BulkAssetResult, error) {
    // Sử dụng pgx COPY hoặc batch INSERT với ON CONFLICT
    conflictClause := "ON CONFLICT (ip_address) DO NOTHING"
    if updateExisting {
        conflictClause = "ON CONFLICT (ip_address) DO UPDATE SET hostname=EXCLUDED.hostname, os=EXCLUDED.os, updated_at=NOW()"
    }
    // ... batch insert ...
}

func (r *assetRepo) GetUniqueTags(ctx context.Context) ([]string, error) {
    rows, _ := r.db.QueryContext(ctx, `SELECT DISTINCT unnest(tags) AS tag FROM osv_asset.assets ORDER BY tag`)
    // ... scan rows → []string ...
}
```

**Migration**: `migrations/asset/0001_init.sql`

```sql
CREATE SCHEMA IF NOT EXISTS osv_asset;

CREATE TABLE IF NOT EXISTS osv_asset.assets (
    id           UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address   INET NOT NULL UNIQUE,
    hostname     VARCHAR(255),
    os           VARCHAR(100),
    mac_address  VARCHAR(17),
    services     JSONB DEFAULT '[]',
    tags         TEXT[] DEFAULT '{}',
    labels       JSONB DEFAULT '{}',
    risk_score   NUMERIC(4,2) DEFAULT 0,
    finding_count INT DEFAULT 0,
    status       VARCHAR(20) DEFAULT 'active',
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- GIN index cho tags (fast tag filtering)
CREATE INDEX IF NOT EXISTS idx_assets_tags ON osv_asset.assets USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_assets_ip ON osv_asset.assets(ip_address);
CREATE INDEX IF NOT EXISTS idx_assets_hostname ON osv_asset.assets(hostname);

-- Vulnerabilities table
CREATE TABLE IF NOT EXISTS osv_asset.asset_vulnerabilities (
    id         UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    asset_id   UUID NOT NULL REFERENCES osv_asset.assets(id) ON DELETE CASCADE,
    cve_id     VARCHAR(50) NOT NULL,
    severity   VARCHAR(10) NOT NULL,
    cvss       NUMERIC(4,2),
    detected_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

### 2.5 Scan-Service — Expose Agent & Scheduled Scan routes

Theo `01-architecture.md §3.6`: `internal/scheduler/` và `delivery/http/schedule/` đã có code.

**File**: `scan-service/internal/delivery/http/scan_handler.go` — Thêm agent handlers:

```go
// RegisterAgent handles POST /api/v1/agents
func (h *Handler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
    var in entity.AgentRegisterInput
    json.NewDecoder(r.Body).Decode(&in)
    agent, apiKey, err := h.agentUC.Register(r.Context(), in)
    // Return agent + api_key (one-time)
    writeJSON(w, 201, map[string]any{
        "id":       agent.ID,
        "name":     agent.Name,
        "api_key":  apiKey,  // plaintext, one-time only
        "status":   "inactive",
    })
}

// SubmitAgentReport handles POST /api/v1/agents/{id}/reports
func (h *Handler) SubmitAgentReport(w http.ResponseWriter, r *http.Request) {
    // Decode report → queue for processing → 202 Accepted
}

// ListAgents handles GET /api/v1/agents
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request)

// GetAgent handles GET /api/v1/agents/{id}
func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request)
```

**Router** (`scan_handler.go` hoặc `router.go`):

```go
r.Post("/api/v1/agents",              h.RegisterAgent)
r.Get("/api/v1/agents",               h.ListAgents)
r.Get("/api/v1/agents/{id}",          h.GetAgent)
r.Post("/api/v1/agents/{id}/reports", h.SubmitAgentReport)
```

---

### 2.6 Gateway Layer — `apps/osv/internal/gateway/router.go`

```go
// ═══════════════════════════════════════════════
// ASSET SEED ENDPOINTS (SEED-005)
// ═══════════════════════════════════════════════
// Literal paths TRƯỚC wildcards
mux.Handle("POST /api/v1/assets",
    protected(ratelimit.Wrap(proxy.Forward("asset-service:8091"), 30)))

mux.Handle("POST /api/v1/assets/bulk",
    protected(ratelimit.Wrap(proxy.Forward("asset-service:8091"), 5)))

mux.Handle("POST /api/v1/assets/import",
    protected(ratelimit.Wrap(proxy.Forward("asset-service:8091"), 2)))

mux.Handle("DELETE /api/v1/assets/{id}",
    protected(proxy.Forward("asset-service:8091")))

mux.Handle("POST /api/v1/assets/{id}/vulnerabilities",
    protected(proxy.Forward("asset-service:8091")))

// ═══════════════════════════════════════════════
// AGENT + SCHEDULED SCAN ENDPOINTS (SEED-005)
// ═══════════════════════════════════════════════
mux.Handle("POST /api/v1/agents",
    adminOnly(proxy.Forward("scan-service:8084")))

mux.Handle("GET /api/v1/agents",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("GET /api/v1/agents/{id}",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("POST /api/v1/agents/{id}/reports",
    protected(proxy.Forward("scan-service:8084")))  // cần scope: scan:execute

// Scheduled scans (chuyển từ /schedules → /api/v1/scans/scheduled)
mux.Handle("POST /api/v1/scans/scheduled",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("GET /api/v1/scans/scheduled",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("GET /api/v1/scans/scheduled/{id}",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("PUT /api/v1/scans/scheduled/{id}",
    protected(proxy.Forward("scan-service:8084")))

mux.Handle("DELETE /api/v1/scans/scheduled/{id}",
    protected(proxy.Forward("scan-service:8084")))
```

> ⚠️ **Path rewrite**: scan-service exposed scheduled scan ở `/schedules/*`. Gateway cần rewrite `/api/v1/scans/scheduled` → `/schedules` khi forward. Dùng `proxy.ForwardWithPathRewrite("scan-service:8084", "/api/v1/scans/scheduled", "/schedules")`.

---

## 3. NATS Events mới

| Subject | Publisher | Consumers | Payload |
|---------|-----------|----------|---------|
| `asset.created` | asset-service | audit-service | `{asset_id, ip_address, actor_id}` |
| `asset.batch_created` | asset-service | audit-service | `{count, actor_id}` |
| `agent.registered` | scan-service | audit-service | `{agent_id, name, actor_id}` |
| `agent.report.submitted` | scan-service | scan-service worker | `{agent_id, report_id, package_count}` |

---

## 4. File thay đổi tổng hợp

| File | Service | Thay đổi |
|------|---------|---------|
| `internal/domain/entity/asset.go` | asset-service | **[NEW]** — định nghĩa Asset entity |
| `internal/domain/repository/asset_repository.go` | asset-service | **[NEW]** |
| `internal/usecase/asset/asset_crud.go` | asset-service | **[NEW]** |
| `internal/delivery/http/handlers.go` | asset-service | Thêm 5 handlers + routes |
| `internal/infra/postgres/asset_repo.go` | asset-service | **[NEW]** |
| `migrations/asset/0001_init.sql` | asset-service | **[NEW]** |
| `internal/delivery/http/scan_handler.go` | scan-service | Thêm agent handlers |
| `apps/osv/internal/gateway/router.go` | gateway | Thêm 14 routes |

---

## 5. Acceptance Criteria

1. `POST /api/v1/assets` với IP + hostname → `201`; `GET /api/v1/assets/{id}` trả về asset.
2. `POST /api/v1/assets/bulk` với 10 assets, 1 IP trùng và `update_existing: true` → `207 {created_count: 9, updated_count: 1}`.
3. `POST /api/v1/assets/import` CSV 20 rows → `200 {imported_count: 20}`.
4. `POST /api/v1/assets/{id}/vulnerabilities` với 3 CVEs → `201`; asset có `finding_count` tăng.
5. `POST /api/v1/agents` (Admin) → `201` với `api_key` plaintext (1 lần).
6. `POST /api/v1/agents/{id}/reports` với 50 packages → `202`; report được queue.
7. `GET /api/v1/scans/scheduled` qua gateway → `200` (không còn `404`).
8. `POST /api/v1/scans/scheduled` với `cron_expr: "0 2 * * *"` → `201`; `next_run_at` đúng.
