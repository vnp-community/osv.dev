# SOL-V6-003: Fix 405 — 4 Endpoints Wrong Method Registration

**Bugs:** BUG-V6-014, BUG-V6-015, BUG-V6-016, BUG-V6-017  
**Task:** TASK-V6-003  
**Services:** `finding-service` (:8085), `sla-service` (:8086), `notification-service` (:8087), `identity-service` (:8081)  
**Kiến trúc tham chiếu:** `01-architecture.md §3.1, §3.5, §3.6, §3.7, §3.8`, `02-technical-design.md §3`

---

## Root Cause Analysis

Gateway (`apps/osv`) dùng Go 1.22+ `net/http` `ServeMux` với pattern `"METHOD /path"`. Lỗi 405 nghĩa là route path tồn tại nhưng HTTP method không khớp với registration.

**Routing reference từ `01-architecture.md §3.1`:**
```
Go 1.22+ ServeMux: mux.Handle("GET /api/v1/admin/users", ...)
Route conflict: /admin/users/invite (POST) vs /admin/users/{id} (GET)
→ Go ServeMux ưu tiên exact match trước, nên /invite không conflict
→ Nhưng nếu chỉ register POST /admin/users/{id} → GET /admin/users/{id} → 405
```

---

## Bug-by-Bug Analysis & Fix

---

### BUG-V6-014: `GET /api/v1/findings/{id}/notes` → 405

**Service:** `finding-service` (:8085)  
**Route group:** `Sprint 2: /api/v1/findings/* (CRUD, bulk, notes, audit) → finding-service:8085`

**Root cause:** Route `/findings/{id}/notes` chỉ được register với `POST` (tạo note), thiếu `GET` (list notes).

**Fix — Router registration trong finding-service:**

```go
// services/finding-service/internal/delivery/http/router.go

// BEFORE (chỉ có POST):
mux.Handle("POST /findings/{id}/notes", authMiddleware(h.AddNote))

// AFTER (thêm GET):
mux.Handle("GET  /findings/{id}/notes", authMiddleware(h.ListNotes))
mux.Handle("POST /findings/{id}/notes", authMiddleware(h.AddNote))
```

**Handler implementation:**

```go
// services/finding-service/internal/delivery/http/finding_handler.go

// GET /findings/{id}/notes → list notes cho finding
func (h *FindingHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
    findingID, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid finding ID")
        return
    }

    // Verify finding exists và user có quyền xem
    if _, err := h.findingUC.GetByID(r.Context(), findingID); err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusNotFound, "finding not found")
            return
        }
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    notes, err := h.noteUC.ListByFinding(r.Context(), findingID)
    if err != nil {
        h.log.Error().Err(err).Msg("list notes failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "notes": notes,
        "total": len(notes),
    })
}
```

**Response schema:**
```json
{
  "notes": [
    {
      "id": "uuid",
      "finding_id": "uuid",
      "content": "Note content",
      "author_id": "uuid",
      "author_name": "John Doe",
      "created_at": "2026-06-24T..."
    }
  ],
  "total": 1
}
```

---

### BUG-V6-015: `PUT /api/v1/sla/config` → 405

**Service:** `sla-service` (:8086)  
**Route group:** `Sprint 4: /api/v1/sla/config → sla-service:8086`

**Root cause:** `sla-service` chỉ expose `GET /sla/config` (đọc config) nhưng chưa có `PUT` handler để cập nhật.

**Fix — Router registration trong sla-service:**

```go
// services/sla-service/internal/delivery/http/router.go

// BEFORE:
mux.Handle("GET /sla/config", authMiddleware(h.GetConfig))

// AFTER:
mux.Handle("GET /sla/config", authMiddleware(h.GetConfig))
mux.Handle("PUT /sla/config", authMiddleware(h.UpdateConfig))
```

**Use case:**

```go
// services/sla-service/internal/usecase/update_config.go

type UpdateSLAConfigInput struct {
    CriticalDays int `json:"critical_days" validate:"required,min=1,max=365"`
    HighDays     int `json:"high_days"     validate:"required,min=1,max=365"`
    MediumDays   int `json:"medium_days"   validate:"required,min=1,max=365"`
    LowDays      int `json:"low_days"      validate:"required,min=1,max=365"`
    // ProductID nil = global default; non-nil = per-product override
    ProductID    *uuid.UUID `json:"product_id,omitempty"`
}

type UpdateSLAConfigUseCase struct {
    repo SLAConfigRepository
    nats *nats.Conn
}

func (uc *UpdateSLAConfigUseCase) Execute(ctx context.Context, in UpdateSLAConfigInput) (*domain.SLAConfig, error) {
    // Validate: CriticalDays < HighDays < MediumDays < LowDays (logical ordering)
    if in.CriticalDays >= in.HighDays || in.HighDays >= in.MediumDays || in.MediumDays >= in.LowDays {
        return nil, fmt.Errorf("%w: SLA days must be in ascending order (critical < high < medium < low)", ErrInvalidInput)
    }

    cfg := &domain.SLAConfig{
        CriticalDays: in.CriticalDays,
        HighDays:     in.HighDays,
        MediumDays:   in.MediumDays,
        LowDays:      in.LowDays,
        ProductID:    in.ProductID,
    }

    if err := uc.repo.Upsert(ctx, cfg); err != nil {
        return nil, fmt.Errorf("upsert sla config: %w", err)
    }

    // Publish event để finding-service tái tính SLA deadlines
    uc.nats.PublishJSON("sla.config.updated", map[string]interface{}{
        "critical_days": cfg.CriticalDays,
        "high_days":     cfg.HighDays,
        "medium_days":   cfg.MediumDays,
        "low_days":      cfg.LowDays,
        "product_id":    cfg.ProductID,
    })

    return cfg, nil
}
```

**Handler:**

```go
// PUT /sla/config
func (h *SLAHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var input usecase.UpdateSLAConfigInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    cfg, err := h.updateConfigUC.Execute(r.Context(), input)
    if err != nil {
        if errors.Is(err, domain.ErrInvalidInput) {
            writeError(w, http.StatusBadRequest, err.Error())
            return
        }
        h.log.Error().Err(err).Msg("update SLA config failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, cfg)
}
```

---

### BUG-V6-016: `GET /api/v1/webhooks/stats` → 405

**Service:** `notification-service` (:8087)  
**Route group:** `Sprint 2: /api/v1/webhooks/* → notification-service:8087`

**Root cause:** Route conflict trong Go 1.22+ ServeMux.  
`/webhooks/stats` vs `/webhooks/{id}` — nếu register `/webhooks/{id}` trước, `GET /webhooks/stats` sẽ match `{id}=stats` thay vì route riêng.  
Hoặc `/webhooks/stats` chỉ được register với `POST` thay vì `GET`.

**Fix — Đăng ký exact path TRƯỚC pattern path:**

```go
// services/notification-service/internal/delivery/http/router.go

// QUAN TRỌNG: Trong Go 1.22+ ServeMux, exact paths có độ ưu tiên cao hơn wildcards.
// Nhưng để chắc chắn, đăng ký stats TRƯỚC {id} routes.

// Webhook collection + static routes (ĐĂNG KÝ TRƯỚC)
mux.Handle("GET  /webhooks",              authMiddleware(h.ListWebhooks))
mux.Handle("POST /webhooks",              authMiddleware(h.CreateWebhook))
mux.Handle("GET  /webhooks/stats",        authMiddleware(h.GetWebhookStats))     // ← THÊM
mux.Handle("GET  /webhooks/deliveries",   authMiddleware(h.ListDeliveries))

// Webhook item routes (ĐĂNG KÝ SAU - wildcard)
mux.Handle("GET    /webhooks/{id}",       authMiddleware(h.GetWebhook))
mux.Handle("PATCH  /webhooks/{id}",       authMiddleware(h.UpdateWebhook))
mux.Handle("DELETE /webhooks/{id}",       authMiddleware(h.DeleteWebhook))
mux.Handle("POST   /webhooks/{id}/test",  authMiddleware(h.TestWebhook))
```

**Handler:**

```go
// GET /webhooks/stats → tổng hợp webhook delivery statistics
func (h *WebhookHandler) GetWebhookStats(w http.ResponseWriter, r *http.Request) {
    // Parse time range từ query params
    since := time.Now().Add(-24 * time.Hour) // default: last 24h
    if s := r.URL.Query().Get("since"); s != "" {
        if t, err := time.Parse(time.RFC3339, s); err == nil {
            since = t
        }
    }

    stats, err := h.statsUC.Execute(r.Context(), since)
    if err != nil {
        h.log.Error().Err(err).Msg("get webhook stats failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    writeJSON(w, http.StatusOK, stats)
}
```

**Response schema:**
```json
{
  "total_webhooks": 5,
  "total_deliveries_24h": 142,
  "success_rate": 98.6,
  "failed_deliveries": 2,
  "by_event_type": {
    "finding.created": 80,
    "scan.completed": 62
  }
}
```

---

### BUG-V6-017: `GET /api/v1/admin/users/{id}` → 405

**Service:** `identity-service` (:8081)  
**Route group:** `Sprint 1: /api/v1/admin/users/* → identity-service:8081 | Admin`

**Root cause:** Trong Go 1.22+ ServeMux, `POST /admin/users/invite` và `GET /admin/users/{id}` có thể conflict nếu chỉ register `POST /admin/users/{id}` mà không có `GET`.

Thực tế: chỉ `POST /admin/users/{id}/unlock` và `POST /admin/users/{id}/reset-password` được đăng ký, thiếu `GET /admin/users/{id}`.

**Fix — Thêm GET handler:**

```go
// services/identity-service/internal/delivery/http/router.go

// Admin users routes — đăng ký exact paths TRƯỚC wildcards
mux.Handle("GET  /admin/users",              adminOnly(h.ListUsers))
mux.Handle("POST /admin/users/invite",       adminOnly(h.InviteUser))       // Exact path — ưu tiên cao hơn {id}

// Wildcard user routes (ĐĂNG KÝ SAU exact paths)
mux.Handle("GET  /admin/users/{id}",         adminOnly(h.GetUser))          // ← THÊM
mux.Handle("POST /admin/users/{id}/unlock",  adminOnly(h.UnlockUser))
mux.Handle("POST /admin/users/{id}/reset-password", adminOnly(h.ResetPassword))
mux.Handle("DELETE /admin/users/{id}",       adminOnly(h.DeleteUser))
```

**Handler:**

```go
// GET /admin/users/{id} → chi tiết một user
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    userID, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid user ID")
        return
    }

    user, err := h.getUserUC.Execute(r.Context(), userID)
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusNotFound, "user not found")
            return
        }
        h.log.Error().Err(err).Msg("get user failed")
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    // AdminUser schema có thêm các fields nhạy cảm so với User schema
    writeJSON(w, http.StatusOK, AdminUserDTO{
        ID:                  user.ID,
        Email:               user.Email,
        Name:                user.Name,
        Role:                user.Role,
        Permissions:         user.Permissions(),
        MFAEnabled:          user.MFAEnabled,
        IsActive:            user.IsActive,
        FailedLoginAttempts: user.FailedLoginAttempts,
        CreatedAt:           user.CreatedAt,
        LastLoginAt:         user.LastLoginAt,
    })
}
```

**Response schema:**
```json
{
  "id": "uuid",
  "email": "user@company.com",
  "name": "John Doe",
  "role": "user",
  "permissions": ["scan:create", "finding:read"],
  "mfa_enabled": true,
  "is_active": true,
  "failed_login_attempts": 0,
  "created_at": "2026-01-01T...",
  "last_login_at": "2026-06-24T..."
}
```

---

## Summary of Router Changes

| Service | File | Changes |
|---------|------|---------|
| `finding-service` | `delivery/http/router.go` | Add `GET /findings/{id}/notes` |
| `sla-service` | `delivery/http/router.go` | Add `PUT /sla/config` |
| `notification-service` | `delivery/http/router.go` | Add `GET /webhooks/stats`, reorder routes (exact before wildcard) |
| `identity-service` | `delivery/http/router.go` | Add `GET /admin/users/{id}`, reorder routes |

---

## Verification

```bash
# BUG-V6-014: GET /findings/{id}/notes
curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/findings/FINDING_ID/notes
# Expected: 200 {"notes": [...], "total": N}

# BUG-V6-015: PUT /sla/config
curl -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"critical_days":7,"high_days":30,"medium_days":90,"low_days":180}' \
  https://c12.openledger.vn/api/v1/sla/config
# Expected: 200 {...}

# BUG-V6-016: GET /webhooks/stats
curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/webhooks/stats
# Expected: 200 {"total_webhooks": N, ...}

# BUG-V6-017: GET /admin/users/{id}
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://c12.openledger.vn/api/v1/admin/users/USER_ID
# Expected: 200 {"id": "...", "email": "...", ...}
```
