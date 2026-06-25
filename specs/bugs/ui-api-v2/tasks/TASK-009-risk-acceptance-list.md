# TASK-009 — finding-service: Implement Risk Acceptance List Handler

**Bug**: [BUG-004](../BUG-004-findings-risk-acceptance.md)  
**Solution**: [SOL-004](../solutions/SOL-004-risk-acceptances.md)  
**Priority**: 🟡 P2  
**Effort**: ~20 phút  
**Status**: `[x] DONE`

---

## Mô tả

`GET /api/v1/risk-acceptances` trả `404`. Router của finding-service chỉ có `POST` — thiếu `GET`. Cần implement `List` handler và đăng ký v1 compat routes.

---

## File cần sửa

**File 1**: [`services/finding-service/internal/delivery/http/risk_acceptance_handler.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/risk_acceptance_handler.go)  
**File 2**: [`services/finding-service/internal/delivery/http/router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go)

---

## Thay đổi 1 — risk_acceptance_handler.go: Thêm List

**Mở file** và **thêm** handler `List`:

```go
// List handles GET /api/v1/risk-acceptances
func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
    // Optional filter by product
    var productID *uuid.UUID
    if pidStr := r.URL.Query().Get("product_id"); pidStr != "" {
        if pid, err := uuid.Parse(pidStr); err == nil {
            productID = &pid
        }
    }

    limit := parseIntParam(r, "limit", 20)
    offset := parseIntParam(r, "offset", 0)

    acceptances, err := h.repo.List(r.Context(), productID, limit, offset)
    if err != nil {
        h.log.Error().Err(err).Msg("RiskAcceptanceHandler.List")
        respondError(w, http.StatusInternalServerError, "failed to list risk acceptances")
        return
    }

    // Defensive: never nil
    if acceptances == nil {
        acceptances = make([]*riskacceptance.RiskAcceptance, 0)
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "data":  acceptances,
        "total": len(acceptances),
    })
}
```

---

## Thay đổi 2 — Thêm List vào repo interface (nếu chưa có)

**Tìm** file domain/interface của risk acceptance và **thêm**:

```go
type Repository interface {
    Create(ctx context.Context, ra *RiskAcceptance) error
    List(ctx context.Context, productID *uuid.UUID, limit, offset int) ([]*RiskAcceptance, error)  // thêm mới
    // ...
}
```

**Implement trong postgres repo**:

```go
func (r *RiskAcceptanceRepo) List(ctx context.Context, productID *uuid.UUID, limit, offset int) ([]*RiskAcceptance, error) {
    q := `
        SELECT ra.id, ra.finding_id, ra.accepted_by, ra.reason, ra.expiration_date, ra.created_at
        FROM risk_acceptances ra
        LEFT JOIN findings f ON f.id = ra.finding_id
        WHERE ($1::uuid IS NULL OR f.product_id = $1)
        ORDER BY ra.created_at DESC
        LIMIT $2 OFFSET $3
    `
    rows, err := r.pool.Query(ctx, q, productID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := make([]*RiskAcceptance, 0)  // never nil
    for rows.Next() {
        ra := &RiskAcceptance{}
        if err := rows.Scan(&ra.ID, &ra.FindingID, &ra.AcceptedBy,
            &ra.Reason, &ra.ExpirationDate, &ra.CreatedAt); err != nil {
            return nil, err
        }
        result = append(result, ra)
    }
    return result, rows.Err()
}
```

---

## Thay đổi 3 — router.go: Thêm v1 routes

**Tìm** section risk acceptance trong [`router.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/internal/delivery/http/router.go#L229):

```go
    // ── Risk acceptance endpoints ──
    if riskAcceptance != nil {
        r.Route("/api/v2/risk-acceptances", func(r chi.Router) {
            r.Post("/", riskAcceptance.Create)
            // r.Get("/{id}", riskAcceptance.Get)
            // r.Delete("/{id}", riskAcceptance.Delete)
        })
    }
```

**Thay bằng**:

```go
    // ── Risk acceptance endpoints ──
    if riskAcceptance != nil {
        // v1 compatibility routes
        r.Get("/api/v1/risk-acceptances", riskAcceptance.List)
        r.Post("/api/v1/risk-acceptances", riskAcceptance.Create)
        r.Delete("/api/v1/risk-acceptances/{id}", riskAcceptance.Delete)

        r.Route("/api/v2/risk-acceptances", func(r chi.Router) {
            r.Get("/", riskAcceptance.List)
            r.Post("/", riskAcceptance.Create)
            r.Get("/{id}", riskAcceptance.Get)
            r.Delete("/{id}", riskAcceptance.Delete)
        })
    }
```

---

## Acceptance Criteria

- [ ] `GET /api/v1/risk-acceptances` trả `200` với `{"data": [], "total": 0}`
- [ ] `GET /api/v2/risk-acceptances` cũng trả `200`
- [ ] Response `data` là array (kể cả empty)
- [ ] `go build ./...` không có lỗi

---

## Verify

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service
go build ./...

curl -s -H "Authorization: Bearer <token>" \
  "https://c12.openledger.vn/api/v1/risk-acceptances" | jq '.data | type'
# Expected: "array"
```
