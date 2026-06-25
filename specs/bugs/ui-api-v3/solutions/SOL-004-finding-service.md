# SOL-004: Finding-Service & SLA-Service — CRUD, Bulk, Risk Acceptances, SLA, Grades

> **Bugs giải quyết**: BUG-006 (Findings), BUG-007 (Risk Acceptances), BUG-008 (SLA), BUG-010 (Product Grades)  
> **Service**: `services/finding-service` (port 8085), `services/sla-service` (port 8086)  
> **Architecture ref**: §3.5 Finding-Service, §3.7 SLA-Service, §5.4 Product Grading  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong finding-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `PATCH /api/v1/findings/{id}` → `PatchFinding` handler | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-004) |
| `POST /api/v1/findings/bulk/close` → `BulkClose` | `internal/delivery/http/router.go` | ✅ Đã có |
| `POST /api/v1/findings/bulk/reopen` → `BulkReopen` | `internal/delivery/http/router.go` | ✅ Đã có |
| `POST /api/v1/findings/bulk/assign` → `BulkAssign` | `internal/delivery/http/router.go` | ✅ Đã có |
| `GET /api/v1/risk-acceptances` (List) | `internal/delivery/http/router.go` | ✅ Đã có |
| `POST /api/v1/risk-acceptances` (Create) | `internal/delivery/http/router.go` | ✅ Đã có |
| `DELETE /api/v1/risk-acceptances/{id}` | `internal/delivery/http/router.go` | ✅ Đã có |
| `RiskAcceptanceHandler` wired (không còn nil) | `embedded.go` | ✅ Fixed (TASK-005) |
| Fix `memberRepo` khai báo trước khi dùng trong `rauc.NewCreate()` | `embedded.go` | ✅ Fixed |
| Fix `raEventPublisher` adapter (interface{} vs map[string]any) | `embedded.go` | ✅ Fixed |

**Build verify**: `go build ./...` ✅ finding-service



---

## BUG-006: Findings — PATCH, Notes GET, Bulk Actions

### Nguyên Nhân

Finding-service có thể đang dùng `PUT` thay vì `PATCH` cho update, và router có thể thiếu một số handlers. Theo domain model (§5.1), Finding có `TransitionTo()` method — cần expose qua HTTP PATCH.

### Fix 1: PATCH /api/v1/findings/{id} (405 → 200)

```go
// services/finding-service/internal/delivery/http/finding_handler.go

// PATCH /api/v1/findings/{id}
// Server có thể đang dùng PUT — thêm PATCH handler riêng
func (h *FindingHandler) PartialUpdate(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    findingID, err := uuid.Parse(id)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid finding ID")
        return
    }
    
    var req struct {
        Status     *string `json:"status"`      // state transition
        AssigneeID *string `json:"assignee_id"`
        Severity   *string `json:"severity"`
        Notes      *string `json:"notes"`
        SLAConfig  *string `json:"sla_config_id"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    userID := r.Header.Get("X-User-ID")
    
    finding, err := h.findingUC.GetByID(r.Context(), findingID)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }
    
    // State transition if status field provided
    if req.Status != nil {
        newState := FindingState(*req.Status)
        actorID, _ := uuid.Parse(userID)
        if err := finding.TransitionTo(newState, actorID); err != nil {
            respondError(w, http.StatusBadRequest, err.Error())
            return
        }
    }
    
    // Partial field updates
    if req.AssigneeID != nil { finding.AssigneeID = req.AssigneeID }
    if req.Severity != nil   { finding.Severity = Severity(*req.Severity) }
    
    if err := h.findingUC.Update(r.Context(), finding); err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Publish state change event
    h.nats.PublishJSON("finding.status.changed", FindingChangedEvent{
        FindingID: finding.ID,
        OldState:  finding.PreviousState,
        NewState:  finding.State,
        ActorID:   userID,
    })
    
    respondJSON(w, http.StatusOK, finding)
}
```

**Router fix**:
```go
// services/finding-service/internal/delivery/http/router.go

// Thêm PATCH route (không xóa PUT nếu đang có clients dùng)
r.PATCH("/api/v1/findings/{id}", authMiddleware(h.PartialUpdate))
r.PUT("/api/v1/findings/{id}",   authMiddleware(h.FullUpdate))  // Giữ nguyên
```

### Fix 2: GET /api/v1/findings/{id}/notes (405 → 200)

```go
// Hiện tại: POST /notes hoạt động, GET /notes trả 405
// Router có thể không có GET handler — chỉ có POST

// GET /api/v1/findings/{id}/notes
func (h *FindingHandler) GetNotes(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    findingID, _ := uuid.Parse(id)
    
    notes, err := h.noteUC.ListByFinding(r.Context(), findingID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": notes,
        "total": len(notes),
    })
}
```

```go
r.GET("/api/v1/findings/{id}/notes",  authMiddleware(h.GetNotes))
r.POST("/api/v1/findings/{id}/notes", authMiddleware(h.AddNote))  // Đã có
```

### Fix 3: Bulk Actions (405 → 200)

```go
// POST /api/v1/findings/bulk/close
func (h *FindingHandler) BulkClose(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingIDs []string `json:"finding_ids"`
        Reason     string   `json:"reason"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    userID := r.Header.Get("X-User-ID")
    actorID, _ := uuid.Parse(userID)
    
    var ids []uuid.UUID
    for _, id := range req.FindingIDs {
        uid, _ := uuid.Parse(id)
        ids = append(ids, uid)
    }
    
    updated, err := h.findingUC.BulkTransition(r.Context(), ids, StateMitigated, actorID, req.Reason)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]int{"updated": updated})
}

// POST /api/v1/findings/bulk/reopen
func (h *FindingHandler) BulkReopen(w http.ResponseWriter, r *http.Request) {
    // Similar — transition to StateActive
}

// POST /api/v1/findings/bulk/assign
func (h *FindingHandler) BulkAssign(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingIDs []string `json:"finding_ids"`
        AssigneeID string   `json:"assignee_id"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    updated, err := h.findingUC.BulkAssign(r.Context(), req.FindingIDs, req.AssigneeID)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]int{"updated": updated})
}
```

```go
// CRITICAL: Đăng ký TRƯỚC /findings/{id} để tránh conflict
r.POST("/api/v1/findings/bulk/close",  authMiddleware(h.BulkClose))
r.POST("/api/v1/findings/bulk/reopen", authMiddleware(h.BulkReopen))
r.POST("/api/v1/findings/bulk/assign", authMiddleware(h.BulkAssign))
```

---

## BUG-007: Risk Acceptances — GET List, POST Create

### Phân Tích

Theo architecture §3.5, `RiskAcceptance` đã là domain entity trong finding-service. Schema `osv_finding` đã có table `risk_acceptances` (vì DELETE/{id} hoạt động).

```go
// GET /api/v1/risk-acceptances?status=active&page=1&limit=20
func (h *RiskAcceptanceHandler) List(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    filter := RiskAcceptanceFilter{
        Page:  parseIntParam(r, "page", 1),
        Limit: parseIntParam(r, "limit", 20),
    }
    if s := r.URL.Query().Get("status"); s != "" {
        filter.Status = s  // "active", "expired"
    }
    
    acceptances, total, err := h.raUC.List(r.Context(), filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "items": acceptances,
        "total": total,
    })
}

// POST /api/v1/risk-acceptances
func (h *RiskAcceptanceHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingID string  `json:"finding_id"`
        Reason    string  `json:"reason"`
        ExpiresAt *string `json:"expires_at"` // ISO8601
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    userID := r.Header.Get("X-User-ID")
    
    // Validate finding exists
    findingID, err := uuid.Parse(req.FindingID)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid finding_id")
        return
    }
    
    ra, err := h.raUC.Create(r.Context(), RiskAcceptanceInput{
        FindingID:  findingID,
        AcceptedBy: userID,
        Reason:     req.Reason,
        ExpiresAt:  req.ExpiresAt,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Transition finding to RiskAccepted state
    h.findingUC.TransitionState(r.Context(), findingID, StateRiskAccepted, userID)
    
    // Publish event (audit + notification)
    h.nats.PublishJSON("risk_acceptance.created", map[string]interface{}{
        "finding_id": findingID,
        "accepted_by": userID,
        "expires_at": req.ExpiresAt,
    })
    
    respondJSON(w, http.StatusCreated, ra)
}
```

```go
// CRITICAL: TRƯỚC /risk-acceptances/{id}
r.GET("/api/v1/risk-acceptances",      authMiddleware(raHandler.List))
r.POST("/api/v1/risk-acceptances",     authMiddleware(raHandler.Create))
r.DELETE("/api/v1/risk-acceptances/{id}", authMiddleware(raHandler.Delete))  // Đã có
```

---

## BUG-008: SLA Config — PUT

### Nguyên Nhân

SLA-service có GET handler nhưng thiếu PUT/PATCH handler. Theo architecture §3.7, SLA config có schema với `CriticalDays`, `HighDays`, etc.

```go
// services/sla-service/internal/delivery/http/sla_handler.go

// PUT /api/v1/sla/config (thêm mới — hiện chỉ có GET)
func (h *SLAHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        CriticalDays int `json:"critical_days"`
        HighDays     int `json:"high_days"`
        MediumDays   int `json:"medium_days"`
        LowDays      int `json:"low_days"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Validate
    if req.CriticalDays <= 0 || req.HighDays <= 0 {
        respondError(w, http.StatusBadRequest, "days must be positive")
        return
    }
    
    updated, err := h.slaUC.UpdateGlobalConfig(r.Context(), SLAConfigInput{
        CriticalDays: req.CriticalDays,
        HighDays:     req.HighDays,
        MediumDays:   req.MediumDays,
        LowDays:      req.LowDays,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    respondJSON(w, http.StatusOK, updated)
}
```

```go
r.GET("/api/v1/sla/config", authMiddleware(h.GetConfig))  // Đã có
r.PUT("/api/v1/sla/config", adminOnly(h.UpdateConfig))     // THÊM MỚI
```

---

## BUG-010: Product Grades — GET List

### Phân Tích

Theo architecture §5.4, `ComputeGrade()` đã được implement trong finding-service. Cần expose endpoint `GET /api/v1/products/grades` để aggregate grades của tất cả products.

```go
// services/finding-service/internal/delivery/http/product_handler.go

// GET /api/v1/products/grades
// ROUTING: Đây là static path — PHẢI đăng ký TRƯỚC /products/{id}
func (h *ProductHandler) GetGrades(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    
    // Try Redis cache first (grade:all:{userID} với TTL 5 phút)
    cacheKey := "grades:all"
    if cached, err := h.cache.Get(r.Context(), cacheKey); err == nil {
        w.Header().Set("X-Cache", "HIT")
        w.Write(cached)
        return
    }
    
    // List all products accessible by user
    products, _, err := h.productUC.List(r.Context(), ProductFilter{UserID: userID})
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Compute grade for each product (or read from cache)
    type GradeItem struct {
        Grade         string    `json:"grade"`
        ProductID     string    `json:"product_id"`
        ProductName   string    `json:"product_name"`
        CriticalCount int       `json:"critical_count"`
        HighCount     int       `json:"high_count"`
        TotalActive   int       `json:"total_active"`
        ComputedAt    time.Time `json:"computed_at"`
    }
    
    var grades []GradeItem
    for _, p := range products {
        result, err := h.gradingUC.ComputeForProduct(r.Context(), p.ID)
        if err != nil {
            continue
        }
        grades = append(grades, GradeItem{
            Grade:         result.Grade,
            ProductID:     p.ID.String(),
            ProductName:   p.Name,
            CriticalCount: result.CriticalCount,
            HighCount:     result.HighCount,
            TotalActive:   result.TotalActive,
            ComputedAt:    result.ComputedAt,
        })
    }
    
    // Aggregate by grade
    distribution := map[string][]GradeItem{}
    for _, g := range grades {
        distribution[g.Grade] = append(distribution[g.Grade], g)
    }
    
    result := map[string]interface{}{
        "grades":  grades,
        "summary": distribution,
        "total":   len(grades),
    }
    
    // Cache for 5 minutes
    h.cache.Set(r.Context(), cacheKey, result, 5*time.Minute)
    
    respondJSON(w, http.StatusOK, result)
}
```

```go
// CRITICAL: PHẢI đăng ký TRƯỚC /products/{id}
r.GET("/api/v1/products/grades", authMiddleware(h.GetGrades))  // THÊM MỚI — TRƯỚC
r.GET("/api/v1/products/types",  authMiddleware(h.GetTypes))   // Đã có
r.GET("/api/v1/products",        authMiddleware(h.List))        // Đã có
r.POST("/api/v1/products",       authMiddleware(h.Create))      // Đã có
r.GET("/api/v1/products/{id}",   authMiddleware(h.GetByID))     // Đã có — SAU
r.PATCH("/api/v1/products/{id}", authMiddleware(h.Update))      // THÊM PATCH (BUG-009)
```
