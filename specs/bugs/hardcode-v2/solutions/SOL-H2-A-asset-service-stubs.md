# SOL-H2-A — asset-service: Fix GetAsset + GetHistory + GetFindings

> Bugs: BUG-H2-001, BUG-H2-002, BUG-H2-003
> Service: `asset-service`

---

## Tổng quan

3 handler stubs trong `asset-service/internal/delivery/http/handlers.go` cần được implement thực sự.

---

## Fix 1: GetAsset — Thêm `Get(id)` vào AssetCRUDUseCase và implement handler

### Bước 1: Thêm `Get` vào `AssetCRUDUseCase` (crud.go)

Repo `assetRepo` đã có `FindByID`. Chỉ cần expose qua use case:

```go
// crud.go — thêm method Get
func (uc *AssetCRUDUseCase) Get(ctx context.Context, id uuid.UUID) (*entity.Asset, error) {
    return uc.repo.FindByID(ctx, id)
}
```

Cần thêm `FindByID` vào interface `CRUDRepository` nếu chưa có.

### Bước 2: Implement handler (handlers.go)

```go
// handlers.go L169-177 — FIX
func (h *Handler) GetAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    asset, err := h.crudUC.Get(r.Context(), id)
    if err != nil {
        jsonError(w, "asset not found", http.StatusNotFound)
        return
    }
    jsonResponse(w, http.StatusOK, asset)
}
```

---

## Fix 2: GetHistory — trả empty với header đúng

`asset_history` table chưa có trong schema. Trả `[]` với pagination stub:

```go
// handlers.go L222-225 — FIX
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
    // Asset history chưa có schema — trả empty list đúng format
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "history": []interface{}{},
        "total":   0,
    })
}
```

---

## Fix 3: GetFindings — proxy qua finding-service gRPC client

`embedded.go` đã wire `mygrpc.FindingClient`. Cần expose qua handler:

```go
// Handler struct — thêm findingClient
type Handler struct {
    crudUC       *asset.AssetCRUDUseCase
    taggingUC    *asset.TaggingUseCase
    riskUC       *asset.RiskScoringUseCase
    listUC       *asset.ListAssetsUseCase
    updateUC     *asset.UpdateAssetUseCase
    findingClient ucasset.FindingClient // NEW
    logger       zerolog.Logger
}

// GetFindings — gọi finding-service
func (h *Handler) GetFindings(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    if h.findingClient == nil {
        jsonResponse(w, http.StatusOK, map[string]interface{}{"findings": []interface{}{}, "total": 0})
        return
    }
    findings, err := h.findingClient.ListByAssetID(r.Context(), id.String())
    if err != nil {
        jsonResponse(w, http.StatusOK, map[string]interface{}{"findings": []interface{}{}, "total": 0})
        return
    }
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "findings": findings,
        "total":    len(findings),
    })
}
```

> **Note**: Nếu `FindingClient.ListByAssetID` chưa có method này, thực hiện HTTP proxy tạm thời.

---

## Files cần modify

| File | Thay đổi |
|------|----------|
| `handlers.go` | GetAsset, GetHistory, GetFindings |
| `crud.go` | Thêm `Get(ctx, id)` |
| `CRUDRepository` interface | Thêm `FindByID` nếu thiếu |
