# SOL-004 — Risk Acceptances: Implement List Handler (P2)

**Bug**: [BUG-004](../BUG-004-findings-risk-acceptance.md)  
**Service**: `finding-service`  
**Endpoint**: `GET /api/v1/risk-acceptances`  
**HTTP Error**: `404 Not Found`

**Status**: `✅ Implemented` — via [TASK-009](../../tasks/TASK-009-*.md)

---

## Root Cause

Gateway route đã đăng ký tại [`router.go:363`](file:///Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go#L363):

```go
mux.Handle("GET /api/v1/risk-acceptances", protected(proxy.Forward("finding-service:8085")))
```

Nhưng finding-service router tại [`router.go:229`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go#L229) chỉ có `POST`:

```go
r.Route("/api/v2/risk-acceptances", func(r chi.Router) {
    r.Post("/", riskAcceptance.Create)
    // r.Get("/{id}", riskAcceptance.Get)    ← commented out
    // r.Delete("/{id}", riskAcceptance.Delete) ← commented out
})
```

→ `GET /api/v1/risk-acceptances` không có handler → 404.

---

## Giải pháp

### Bước 1: Thêm List handler vào RiskAcceptanceHandler

File: [`services/finding-service/internal/delivery/http/risk_acceptance_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/risk_acceptance_handler.go)

```go
// List handles GET /api/v1/risk-acceptances (and /api/v2/risk-acceptances)
func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
    filter := RiskAcceptanceFilter{
        Limit:     parseIntParam(r, "limit", 20),
        Offset:    parseIntParam(r, "offset", 0),
    }
    
    if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
        if pid, err := uuid.Parse(pidStr); err == nil {
            filter.ProductID = &pid
        }
    }
    
    acceptances, err := h.repo.List(r.Context(), filter)
    if err != nil {
        h.log.Error().Err(err).Msg("RiskAcceptanceHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list risk acceptances")
        return
    }
    
    // Defensive: never nil
    if acceptances == nil {
        acceptances = make([]*RiskAcceptance, 0)
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  acceptances,
        "total": len(acceptances),
    })
}
```

### Bước 2: Register routes trong finding-service router

File: [`services/finding-service/internal/delivery/http/router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go#L229)

```go
// ── Risk acceptance endpoints ──
if riskAcceptance != nil {
    // v1 compatibility
    r.Get("/api/v1/risk-acceptances", riskAcceptance.List)
    r.Post("/api/v1/risk-acceptances", riskAcceptance.Create)
    r.Delete("/api/v1/risk-acceptances/{id}", riskAcceptance.Delete)
    
    r.Route("/api/v2/risk-acceptances", func(r chi.Router) {
        r.Get("/", riskAcceptance.List)    // Uncomment + implement
        r.Post("/", riskAcceptance.Create)
        r.Get("/{id}", riskAcceptance.Get) // Uncomment + implement
        r.Delete("/{id}", riskAcceptance.Delete) // Uncomment + implement
    })
}
```

### Bước 3: Implement repo List

```go
// services/finding-service/internal/infra/postgres/risk_acceptance_repo.go (nếu chưa có)

func (r *RiskAcceptanceRepo) List(ctx context.Context, filter RiskAcceptanceFilter) ([]*RiskAcceptance, error) {
    q := `SELECT id, finding_id, accepted_by, reason, expiration_date, created_at
          FROM risk_acceptances
          WHERE ($1::uuid IS NULL OR finding_id IN (
              SELECT id FROM findings WHERE product_id = $1
          ))
          ORDER BY created_at DESC
          LIMIT $2 OFFSET $3`
    
    rows, err := r.pool.Query(ctx, q, filter.ProductID, filter.Limit, filter.Offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    result := make([]*RiskAcceptance, 0)  // never nil
    for rows.Next() {
        ra := &RiskAcceptance{}
        if err := rows.Scan(/* ... */); err != nil {
            return nil, err
        }
        result = append(result, ra)
    }
    return result, rows.Err()
}
```

---

## Response Schema

```json
{
  "data": [
    {
      "id": "uuid",
      "finding_id": "uuid",
      "accepted_by": "user@example.com",
      "reason": "Business risk accepted",
      "expiration_date": "2026-12-31T00:00:00Z",
      "created_at": "2026-06-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

---

## Files cần sửa

| File | Thay đổi |
|------|----------|
| [`risk_acceptance_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/risk_acceptance_handler.go) | Thêm `List` handler |
| [`router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go#L229) | Uncomment GET routes, thêm v1 compat |
