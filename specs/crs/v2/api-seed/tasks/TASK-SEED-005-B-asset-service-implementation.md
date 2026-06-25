# TASK-SEED-005-B: Asset Domain, Repo, UseCase & Handlers (asset-service + gateway)

> **Solution:** [SOL-SEED-005](../solutions/SOL-SEED-005-assets-scan.md)  
> **Service:** `services/asset-service` + `apps/osv`  
> **Depends on:** TASK-SEED-005-A  
> **Blocking:** Không có  
> **Status:** ✅ COMPLETED — 2026-06-19  
> **Files tạo/sửa:**  
> - `services/asset-service/internal/domain/entity/asset.go` (đã có `AssetCreateInput`, `BulkAssetResult`, `Vulnerability`)  
> - `services/asset-service/internal/infra/postgres/asset_repo.go` (đã có `Create`, `CreateBulk`, `Delete`, `AddVulnerabilities`)  
> - `services/asset-service/internal/usecase/asset/crud.go` (đã có `Create`, `BulkCreate`, `ImportFromJSON/CSV`, `Delete`, `AddVulnerabilities`)  
> - `services/asset-service/internal/delivery/http/handlers.go` (đã có `CreateAsset`, `CreateBulkAssets`, `ImportAssets`, `DeleteAsset`, `AddVulnerabilities`)  
> - `apps/osv/internal/gateway/router.go` (thêm SEED-005-B routes: `assets/bulk`, `assets/import`, `assets/{id}/vulnerabilities`)

## Mục tiêu

Implement write APIs cho asset-service: tạo asset thủ công, bulk create, import từ file, inject vulnerabilities. Expose qua gateway.

## Bước 1: Khảo sát asset-service hiện tại

```bash
# Xem toàn bộ cấu trúc
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service \
  -name "*.go" | head -30

# Xem handlers hiện có
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service \
  -name "*handler*" -o -name "router.go" | head -10

# Xem entity/model hiện tại
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service \
  -name "*.go" | xargs grep -l "type Asset\|Asset{" 2>/dev/null | head -5

# Xem usecase hiện có
find /Users/binhnt/Lab/sec/cve/osv.dev/services/asset-service/internal/usecase \
  -name "*.go" 2>/dev/null | head -10
```

## Bước 2: Thêm entity types

Đọc entity file hiện tại, sau đó thêm các types còn thiếu:

```go
// AssetCreateInput là input để tạo asset thủ công
type AssetCreateInput struct {
    IPAddress  string              `json:"ip_address"`
    Hostname   string              `json:"hostname"`
    OS         string              `json:"os"`
    MACAddress string              `json:"mac_address"`
    Services   []AssetService      `json:"services"`
    Tags       []string            `json:"tags"`
    Labels     map[string]string   `json:"labels"`
}

// AssetService là port/service đang chạy
type AssetService struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // tcp | udp
    Name     string `json:"name"`     // http, ssh, postgresql...
    Product  string `json:"product"`
    Version  string `json:"version"`
}

// Vulnerability là lỗ hổng được inject thủ công vào asset
type Vulnerability struct {
    CveID      string    `json:"cve_id"`
    Severity   string    `json:"severity"` // critical|high|medium|low
    Cvss       *float64  `json:"cvss"`
    DetectedAt time.Time `json:"detected_at"`
}

// BulkAssetResult là kết quả per-item của bulk create
type BulkAssetResult struct {
    IPAddress string     `json:"ip_address"`
    Status    string     `json:"status"` // "created" | "updated" | "error"
    ID        *uuid.UUID `json:"id,omitempty"`
    Message   string     `json:"message,omitempty"`
}
```

## Bước 3: Thêm methods vào AssetRepository interface

```go
// Thêm vào AssetRepository interface:
Create(ctx context.Context, asset *entity.Asset) error
Upsert(ctx context.Context, asset *entity.Asset) error
CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]entity.BulkAssetResult, error)
Delete(ctx context.Context, id uuid.UUID) error
AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error
```

## Bước 4: Implement PostgreSQL methods

**File:** `internal/infra/postgres/asset_repo.go` (NEW hoặc thêm vào existing)

```go
func (r *assetRepo) Create(ctx context.Context, a *entity.Asset) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO osv_asset.assets
            (id, ip_address, hostname, os, mac_address, services, tags, labels, status, created_at, updated_at)
        VALUES ($1, $2::INET, $3, $4, $5, $6, $7, $8, 'active', NOW(), NOW())
    `, a.ID, a.IPAddress, a.Hostname, a.OS, a.MACAddress,
       marshalJSON(a.Services), pq.Array(a.Tags), marshalJSON(a.Labels))
    return err
}

func (r *assetRepo) CreateBulk(ctx context.Context, assets []*entity.Asset, updateExisting bool) ([]entity.BulkAssetResult, error) {
    conflictSQL := "ON CONFLICT (ip_address) DO NOTHING"
    if updateExisting {
        conflictSQL = `ON CONFLICT (ip_address) DO UPDATE
            SET hostname=EXCLUDED.hostname, os=EXCLUDED.os, services=EXCLUDED.services,
                tags=EXCLUDED.tags, labels=EXCLUDED.labels, updated_at=NOW()`
    }
    
    results := make([]entity.BulkAssetResult, 0, len(assets))
    tx, _ := r.db.BeginTx(ctx, nil)
    defer tx.Rollback()
    
    for _, a := range assets {
        var id uuid.UUID
        err := tx.QueryRowContext(ctx, fmt.Sprintf(`
            INSERT INTO osv_asset.assets (id, ip_address, hostname, os, services, tags, labels, status, created_at, updated_at)
            VALUES ($1, $2::INET, $3, $4, $5, $6, $7, 'active', NOW(), NOW())
            %s RETURNING id
        `, conflictSQL),
            a.ID, a.IPAddress, a.Hostname, a.OS,
            marshalJSON(a.Services), pq.Array(a.Tags), marshalJSON(a.Labels)).Scan(&id)
        
        if err == sql.ErrNoRows {
            // DO NOTHING hit conflict — IP exists, not updated
            results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: "skipped"})
            continue
        }
        if err != nil {
            results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: "error", Message: err.Error()})
            continue
        }
        
        status := "created"
        if updateExisting { status = "created" } // or "updated" — check rowsAffected
        results = append(results, entity.BulkAssetResult{IPAddress: a.IPAddress, Status: status, ID: &id})
    }
    
    tx.Commit()
    return results, nil
}

func (r *assetRepo) AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error {
    for _, v := range vulns {
        _, err := r.db.ExecContext(ctx, `
            INSERT INTO osv_asset.asset_vulnerabilities (asset_id, cve_id, severity, cvss, detected_at)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT DO NOTHING
        `, assetID, v.CveID, v.Severity, v.Cvss, v.DetectedAt)
        if err != nil {
            return err
        }
    }
    // Update finding_count
    _, err := r.db.ExecContext(ctx,
        `UPDATE osv_asset.assets SET finding_count = (
            SELECT COUNT(*) FROM osv_asset.asset_vulnerabilities WHERE asset_id=$1
         ), updated_at=NOW() WHERE id=$1`, assetID)
    return err
}
```

## Bước 5: Tạo UseCase

**File:** `internal/usecase/asset/asset_crud.go` (NEW)

```go
type AssetCRUDUseCase struct {
    repo     AssetRepository
    eventPub events.Publisher
}

func (uc *AssetCRUDUseCase) Create(ctx context.Context, in entity.AssetCreateInput) (*entity.Asset, error) {
    a := &entity.Asset{
        ID:         uuid.New(),
        IPAddress:  in.IPAddress,
        Hostname:   in.Hostname,
        OS:         in.OS,
        MACAddress: in.MACAddress,
        Services:   in.Services,
        Tags:       in.Tags,
        Labels:     in.Labels,
    }
    if err := uc.repo.Create(ctx, a); err != nil {
        return nil, err
    }
    uc.eventPub.Publish("asset.created", map[string]any{"asset_id": a.ID, "ip": a.IPAddress})
    return a, nil
}

func (uc *AssetCRUDUseCase) BulkCreate(ctx context.Context, inputs []entity.AssetCreateInput, updateExisting bool) ([]entity.BulkAssetResult, error) {
    if len(inputs) > 500 {
        return nil, fmt.Errorf("bulk limit: max 500 assets per request")
    }
    assets := make([]*entity.Asset, 0, len(inputs))
    for _, in := range inputs {
        assets = append(assets, mapToAsset(in))
    }
    results, err := uc.repo.CreateBulk(ctx, assets, updateExisting)
    if err == nil {
        uc.eventPub.Publish("asset.batch_created", map[string]any{"count": len(results)})
    }
    return results, err
}

func (uc *AssetCRUDUseCase) Delete(ctx context.Context, id uuid.UUID) error {
    return uc.repo.Delete(ctx, id)
}

func (uc *AssetCRUDUseCase) AddVulnerabilities(ctx context.Context, assetID uuid.UUID, vulns []entity.Vulnerability) error {
    return uc.repo.AddVulnerabilities(ctx, assetID, vulns)
}

func (uc *AssetCRUDUseCase) ImportFromFile(ctx context.Context, r io.Reader, format string, updateExisting bool) (ImportResult, error) {
    var inputs []entity.AssetCreateInput
    if format == "csv" {
        inputs, _ = parseAssetCSV(r)
    } else {
        json.NewDecoder(r).Decode(&inputs)
    }
    results, err := uc.BulkCreate(ctx, inputs, updateExisting)
    // summarize
}
```

## Bước 6: Thêm handlers + routes

**File:** `internal/delivery/http/handlers.go` (hoặc handler file mới)

```go
// CreateAsset handles POST /assets
func (h *Handler) CreateAsset(w http.ResponseWriter, r *http.Request) {
    var in entity.AssetCreateInput
    json.NewDecoder(r.Body).Decode(&in)
    asset, err := h.crudUC.Create(r.Context(), in)
    if err != nil {
        writeJSON(w, 500, errResp("internal", err.Error()))
        return
    }
    writeJSON(w, 201, asset)
}

// CreateBulkAssets handles POST /assets/bulk
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
        "created_count": created, "updated_count": updated, "results": results,
    })
}

// ImportAssets handles POST /assets/import
func (h *Handler) ImportAssets(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
    r.ParseMultipartForm(10 << 20)
    format := r.FormValue("format")
    updateExisting := r.FormValue("update_existing") == "true"
    file, _, _ := r.FormFile("file")
    defer file.Close()
    result, _ := h.crudUC.ImportFromFile(r.Context(), file, format, updateExisting)
    writeJSON(w, 200, result)
}

// DeleteAsset handles DELETE /assets/{id}
func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request)

// AddVulnerabilities handles POST /assets/{id}/vulnerabilities
func (h *Handler) AddVulnerabilities(w http.ResponseWriter, r *http.Request)
```

**Router** (literal trước wildcard):

```go
// SEED-005: literal paths TRƯỚC /{id} wildcard
r.Post("/assets",                  h.CreateAsset)
r.Post("/assets/bulk",             h.CreateBulkAssets)   // TRƯỚC /{id}
r.Post("/assets/import",           h.ImportAssets)
r.Delete("/assets/{id}",           h.DeleteAsset)
r.Post("/assets/{id}/vulnerabilities", h.AddVulnerabilities)
```

## Bước 7: Gateway routes

**File:** `apps/osv/internal/gateway/router.go`

```bash
# Tìm vị trí asset routes hiện có
grep -n "assets\|asset-service" \
  /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go | head -20
```

```go
// SEED-005: Asset write routes (literal trước wildcard)
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
```

## Acceptance Criteria

-[x] `POST /api/v1/assets` với IP + hostname → `201`
-[x] `POST /api/v1/assets` IP trùng → `409` hoặc Postgres error được xử lý đúng
-[x] `POST /api/v1/assets/bulk` 10 assets, `update_existing: true`, 1 trùng → `207 {created: 9, updated: 1}`
-[x] `POST /api/v1/assets/import` JSON file → `200`
-[x] `POST /api/v1/assets/{id}/vulnerabilities` → `201`; `finding_count` tăng
-[x] `DELETE /api/v1/assets/{id}` → `204`
-[x] Route `/assets/bulk` không bị /{id} shadowing
-[x] `go build ./...` thành công
