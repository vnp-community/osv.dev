# TASK-005: Finding-Service — Risk Acceptances v1 & SLA Config

> **Bugs**: BUG-007 (Risk Acceptances), BUG-008 (SLA Config PUT)  
> **Solution**: SOL-005  
> **Service**: `services/finding-service`  
> **File chính**: `embedded.go`, `internal/infra/postgres/risk_acceptance_repo.go` *(NEW)*  
> **Priority**: 🔴 HIGH  
> **Status**: `[x] DONE`

## Phân Tích Thực Tế

### BUG-007: Risk Acceptances

**Từ router.go**:
```go
// v1 compatibility routes
r.Get("/api/v1/risk-acceptances", riskAcceptance.List)    // ← ĐÃ CÓ
r.Post("/api/v1/risk-acceptances", riskAcceptance.Create) // ← ĐÃ CÓ
r.Delete("/api/v1/risk-acceptances/{id}", riskAcceptance.Delete) // ← ĐÃ CÓ
```

Routes **đã được register** nhưng vẫn 404. Nguyên nhân tương tự TASK-003:
- `riskAcceptance` handler có thể **nil** trong `NewRouter()` call
- Hoặc `repo` field trong `RiskAcceptanceHandler` là nil (thiếu `WithRepo()` call)

**Kiểm tra**:
```go
// risk_acceptance_handler.go
func NewRiskAcceptanceHandler(c *rauc.CreateRiskAcceptanceUseCase, r *rauc.RemoveFindingFromRAUseCase) *RiskAcceptanceHandler {
    return &RiskAcceptanceHandler{createUC: c, removeUC: r}  // repo = nil!
}

// WithRepo attaches a Repository for List/Get/Delete operations.
func (h *RiskAcceptanceHandler) WithRepo(repo ra.Repository) *RiskAcceptanceHandler {
    h.repo = repo
    return h
}

func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
    if h.repo == nil {
        respondError(w, http.StatusInternalServerError, "repo not initialized")
        return  // ← có thể đây là vấn đề!
    }
    ...
}
```

### BUG-008: SLA Config PUT

**Gateway** đã có:
```go
mux.Handle("PUT /api/v1/sla/config", adminOnly(proxy.Forward("sla-service:8086")))
```

Cần verify sla-service có handler cho `PUT /api/v1/sla/config`.

## Việc Cần Làm

### BUG-007: Bước 1 — Kiểm tra RiskAcceptanceHandler wiring

```bash
cat services/finding-service/cmd/server/main.go | grep -A30 "riskAccept\|RiskAccept"
# hoặc
cat services/finding-service/internal/config/wire.go 2>/dev/null | grep -A10 "riskAccept"
```

### BUG-007: Bước 2 — Fix wiring trong main/embed

File: `services/finding-service/cmd/server/embed.go` hoặc `cmd/server/main.go`

```go
// TRƯỚC (có thể thiếu WithRepo):
raHandler := http.NewRiskAcceptanceHandler(createRAUC, removeRAUC)
// repo chưa được attach!

// SAU (fix):
raRepo := postgres.NewRiskAcceptanceRepository(db)  // verify tên đúng
raHandler := http.NewRiskAcceptanceHandler(createRAUC, removeRAUC).
    WithRepo(raRepo)  // THÊM
```

### BUG-007: Bước 3 — Kiểm tra ra.Repository implementation

```bash
find services/finding-service -name "*.go" | xargs grep -l "RiskAcceptance\|risk_acceptance" 2>/dev/null | head -10
```

Verify interface:
```go
type Repository interface {
    Create(ctx context.Context, ra *RiskAcceptance) (*RiskAcceptance, error)
    GetByID(ctx context.Context, id uuid.UUID) (*RiskAcceptance, error)
    List(ctx context.Context, filter ListFilter) ([]*RiskAcceptance, int, error)
    ListByProduct(ctx context.Context, productID uuid.UUID) ([]*RiskAcceptance, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### BUG-007: Bước 4 — Verify List handler

```bash
grep -n "func.*List\|h.repo\|repo.List\|ListByProduct" \
  services/finding-service/internal/delivery/http/risk_acceptance_handler.go
```

Nếu `List` không handle được request từ gateway (query params khác):

```go
// Đảm bảo List handler xử lý đúng
func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
    if h.repo == nil {
        respondError(w, http.StatusInternalServerError, "risk acceptance repo not wired")
        return
    }

    productIDStr := r.URL.Query().Get("product_id")
    
    var acceptances []*ra.RiskAcceptance
    var err error
    
    if productIDStr != "" {
        productID, parseErr := uuid.Parse(productIDStr)
        if parseErr != nil {
            respondError(w, http.StatusBadRequest, "invalid product_id")
            return
        }
        acceptances, err = h.repo.ListByProduct(r.Context(), productID)
    } else {
        // List all (no product filter) — add this path if missing
        acceptances, err = h.repo.ListAll(r.Context())  // might need to add this method
    }

    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list risk acceptances")
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": acceptances,
        "total": len(acceptances),
    })
}
```

---

### BUG-008: SLA Config PUT

#### Bước 1: Kiểm tra SLA handler trong sla-service

```bash
find services/sla-service -name "*.go" | xargs grep -l "handler\|Handler\|router\|Router" 2>/dev/null | head -10
# Rồi:
find services/sla-service -name "*.go" | xargs grep -n "v1/sla\|PUT\|Update\|config" 2>/dev/null | head -20
```

#### Bước 2: Nếu thiếu PUT handler — thêm vào

```bash
find services/sla-service -name "sla_handler.go" -o -name "handler.go" | head -3 | xargs cat 2>/dev/null | head -80
```

Thêm handler:
```go
// PUT /api/v1/sla/config — update global SLA configuration
func (h *SLAConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        CriticalDays int `json:"critical_days"`
        HighDays     int `json:"high_days"`
        MediumDays   int `json:"medium_days"`
        LowDays      int `json:"low_days"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Validate
    if req.CriticalDays <= 0 || req.HighDays <= 0 || req.MediumDays <= 0 || req.LowDays <= 0 {
        respondError(w, http.StatusBadRequest, "all day values must be positive")
        return
    }

    updated, err := h.configUC.UpdateGlobal(r.Context(), SLAConfigInput{
        CriticalDays: req.CriticalDays,
        HighDays:     req.HighDays,
        MediumDays:   req.MediumDays,
        LowDays:      req.LowDays,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, http.StatusOK, updated)
}
```

Register:
```go
// sla-service router
r.Get("/api/v1/sla/config", h.GetConfig)    // đã có
r.Put("/api/v1/sla/config", h.UpdateConfig) // THÊM MỚI
```

## Build & Test

```bash
# finding-service
cd services/finding-service && go build ./...

# sla-service
cd services/sla-service && go build ./...
```

**Test BUG-007**:
```bash
curl -s "$BASE/api/v1/risk-acceptances" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {items: [], total: 0}
```

**Test BUG-008**:
```bash
curl -s -X PUT "$BASE/api/v1/sla/config" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"critical_days": 7, "high_days": 30, "medium_days": 90, "low_days": 180}' | jq .
# Expected: 200 OK
```

## Acceptance Criteria

**BUG-007**:
- [x] `GET /api/v1/risk-acceptances` → `200 OK` với `{items: [...], total: N}`
- [x] `POST /api/v1/risk-acceptances` → `201 Created`
- [x] `DELETE /api/v1/risk-acceptances/{id}` → `204 No Content` (verify)

**BUG-008**:
- [x] `PUT /api/v1/sla/config` → `200 OK`
- [x] `GET /api/v1/sla/config` → `200 OK` (verify không regression)

- [x] `go build ./...` không lỗi cho cả hai services
