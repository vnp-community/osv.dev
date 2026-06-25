# P0 — Crash Risk Fixes (Ngay lập tức)

> **Bugs**: MOCK-002, MOCK-012, MOCK-014  
> **Mức độ**: 🔴 High — Production crash risk  
> **Timeline**: Tuần 1, ưu tiên cao nhất

---

## MOCK-002 — Fix: Nil-check trong `ReportHandler.Create()` và `Download()`

### Vấn đề
```go
// report_handler.go L88: PANIC khi generateUC == nil
rep, err := h.generateUC.Execute(r.Context(), ...)
// report_handler.go L140: PANIC khi storage == nil
url, err := h.storage.PresignedURL(r.Context(), ...)
```

### Giải pháp

**Chiến lược**: Thêm nil-check graceful — trả `503 Service Unavailable` với message rõ ràng thay vì panic. Theo `02-technical-design.md §12.1`, các lỗi infrastructure nên map về HTTP 503.

**File cần sửa**: `services/finding-service/internal/delivery/http/report_handler.go`

```go
// Create handles POST /api/v2/reports → 201 {id, status: "pending"}
func (h *ReportHandler) Create(w http.ResponseWriter, r *http.Request) {
    // FIX MOCK-002: nil-check trước khi dereference generateUC
    if h.generateUC == nil {
        writeJSON(w, http.StatusServiceUnavailable, apiErr(
            "report generation not configured: MinIO storage required",
        ))
        return
    }

    var req struct { /* ... giữ nguyên ... */ }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, apiErr("invalid request body"))
        return
    }

    rep, err := h.generateUC.Execute(r.Context(), reportuc.GenerateInput{
        /* ... giữ nguyên ... */
    })
    if err != nil {
        writeJSON(w, http.StatusBadRequest, apiErr(err.Error()))
        return
    }
    writeJSON(w, http.StatusCreated, rep)
}

// Download handles GET /api/v2/reports/{id}/download → 302 pre-signed URL
func (h *ReportHandler) Download(w http.ResponseWriter, r *http.Request) {
    // FIX MOCK-002: nil-check trước khi dereference storage
    if h.storage == nil {
        writeJSON(w, http.StatusServiceUnavailable, apiErr(
            "report download not configured: MinIO storage required",
        ))
        return
    }

    id := r.PathValue("id")
    rep, err := h.repo.FindByID(r.Context(), id)
    /* ... phần còn lại giữ nguyên ... */
}
```

### Test cases cần viết
```go
func TestReportHandler_Create_NilGenerateUC(t *testing.T) {
    h := NewReportHandler(nil, &nilReportRepo{}, nil)
    req := httptest.NewRequest("POST", "/api/v1/reports", strings.NewReader(`{}`))
    w := httptest.NewRecorder()
    h.Create(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
    assert.Contains(t, w.Body.String(), "not configured")
}
```

### Ghi chú
Theo `01-architecture.md §3.5`: Report generation được embed trong finding-service, nhưng cần MinIO. Khi MinIO chưa cấu hình, 503 là response hợp lý để frontend biết feature chưa available.

---

## MOCK-012 — Fix: Nil-check trong `APIKeyValidator.Validate()`

### Vấn đề
```go
// apikey_validator.go L65: PANIC khi repo == nil và Redis cache miss
key, err := v.repo.FindByHash(ctx, hash)  // nil dereference!
```

### Giải pháp

**Chiến lược**: Thêm nil-check cho `repo`. Khi `repo == nil`, skip cold path và return `ErrInvalidAPIKey` — API key không thể validate mà không có DB lookup. Theo `02-technical-design.md §10.2`, validation dùng SHA-256 hash lookup qua DB.

**Phương án A (Minimal fix — khuyến nghị cho P0)**:

**File cần sửa**: `services/gateway-service/internal/auth/apikey_validator.go`

```go
// Validate looks up an API key by hashing the raw key value.
// Flow: Redis cache (5m TTL) → PostgreSQL → async update last_used_at.
func (v *APIKeyValidator) Validate(ctx context.Context, rawKey string) (*APIKeyClaims, error) {
    if rawKey == "" {
        return nil, ErrInvalidAPIKey
    }

    hash := sha256Hex(rawKey)
    cacheKey := "apikey:v1:" + hash

    // 1. Redis cache (hot path — no crypto/DB on warm requests)
    if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil {
        var claims APIKeyClaims
        if json.Unmarshal(cached, &claims) == nil {
            return &claims, nil
        }
    }

    // FIX MOCK-012: nil-check — không thể validate nếu không có DB
    if v.repo == nil {
        return nil, ErrInvalidAPIKey
    }

    // 2. PostgreSQL lookup (cold path)
    key, err := v.repo.FindByHash(ctx, hash)
    /* ... phần còn lại giữ nguyên ... */
}
```

**Phương án B (Đầy đủ — cho sprint tiếp theo)**:

Wire `APIKeyRepository` thực sự từ identity-service hoặc gateway DB vào `WireEmbedded`:

**File cần sửa**: `services/gateway-service/embedded.go:L125`

```go
// Trước: nil repository
apiKeyValidator := auth.NewAPIKeyValidator(nil, rdb)

// Sau: wire repository từ identity DB pool
// NOTE: gateway cần kết nối identity-service DB hoặc gọi identity gRPC
// Option 1: gọi identity-service gRPC để validate (theo kiến trúc phân tán)
identityAPIKeyRepo := gateway.NewIdentityAPIKeyRepoGRPC(identityGRPCAddr)
apiKeyValidator := auth.NewAPIKeyValidator(identityAPIKeyRepo, rdb)

// Option 2: dùng shared DB pool (nếu monolith mode)
apiKeyValidator := auth.NewAPIKeyValidator(pgRepo.NewAPIKeyRepo(dbPool), rdb)
```

**Khuyến nghị**: Phương án A cho P0 (ngay lập tức), Phương án B khi gateway được tách thành standalone service.

### Test cases cần viết
```go
func TestAPIKeyValidator_Validate_NilRepo_CacheMiss(t *testing.T) {
    v := auth.NewAPIKeyValidator(nil, mockRedis) // Redis miss
    _, err := v.Validate(ctx, "ak_live_testkey")
    assert.ErrorIs(t, err, auth.ErrInvalidAPIKey)
    // Should NOT panic
}
```

---

## MOCK-014 — Fix: Nil-check trong `notification-service/router.go`

### Vấn đề
```go
// router.go L73-78: đăng ký route TRỰC TIẾP không có nil-check
r.Route("/api/v2/notifications", func(r chi.Router) {
    r.Get("/", ah.ListNotifications)  // PANIC nếu ah == nil
    r.Get("/stream", sse.Stream)      // PANIC nếu sse == nil
    ...
})
```

**Giải pháp bao gồm 2 phần:**

### Phần 1: Fix router.go — Thêm nil-check

**File cần sửa**: `services/notification-service/internal/delivery/http/router.go`

```go
// SetupRouter creates the chi router for the notification service.
func SetupRouter(wh *WebhookHandler, sh *SubscriptionHandler, ih *InternalHandler,
    ah *AlertsHandler, sse *SSEHandler, rh *RuleHandler, dh *DeliveryHandler) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    r.Use(injectClaimsFromHeader)

    /* ... Webhook routes giữ nguyên ... */

    // FIX MOCK-014: guard tất cả ah.* và sse.* bằng nil-check
    // In-app Notifications — chỉ mount khi AlertsHandler được wire
    if ah != nil {
        r.Route("/api/v2/notifications", func(r chi.Router) {
            r.Get("/", ah.ListNotifications)
            r.Patch("/{id}/read", ah.MarkRead)
            r.Post("/mark-all-read", ah.MarkAllRead)
            r.Get("/unread-count", ah.UnreadCount)
            // SSE stream — chỉ mount khi SSEHandler được wire
            if sse != nil {
                r.Get("/stream", sse.Stream)
            }
        })

        // TASK-011: v1 compat routes
        r.Post("/api/v1/notifications/mark-all-read", ah.MarkAllRead)
        r.Get("/api/v1/notifications/unread-count", ah.UnreadCount)
        r.Get("/api/v1/notifications", ah.ListNotifications)
        r.Patch("/api/v1/notifications/{id}/read", ah.MarkRead)
    } else {
        // Graceful stub: trả 503 thay vì 404 để phân biệt "not wired" vs "not found"
        notImplemented := func(w http.ResponseWriter, r *http.Request) {
            respondJSON(w, http.StatusServiceUnavailable,
                map[string]string{"error": "notification service not fully initialized"})
        }
        r.Get("/api/v2/notifications", notImplemented)
        r.Get("/api/v1/notifications", notImplemented)
    }

    /* ... phần còn lại giữ nguyên ... */
    return r
}
```

### Phần 2: Wire AlertsHandler và SSEHandler trong embedded.go

**File cần sửa**: `services/notification-service/embedded.go`

Theo `01-architecture.md §3.8`: notification-service cần `AlertStore` (in-app) và SSE broker.

```go
// WireEmbedded initializes the Notification service routes on the provided ServeMux.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
    /* ... phần Redis và repos giữ nguyên ... */

    // FIX MOCK-014: Wire AlertsHandler và SSEHandler thực sự
    // AlertsRepository — đọc/ghi bảng alerts trong PostgreSQL
    alertRepo := infrapostgres.NewAlertRepository(pool)
    ahHandler := deliverhttp.NewAlertsHandler(alertRepo)

    // SSEHandler — cần EventBroker + TokenValidator
    // EventBroker: in-memory broker cho SSE streams
    sseEventBroker := broker.NewEventBroker()
    go sseEventBroker.Run(ctx) // chạy broker trong background

    // TokenValidator: validate JWT token từ query param ?token=
    // Theo 01-architecture.md §3.1: sseAuth chain dùng AuthenticateSSE
    jwtSecret := os.Getenv("JWT_SECRET")
    tokenSvc := jwt.NewTokenValidator(jwtSecret)
    sseHandler := deliverhttp.NewSSEHandler(sseEventBroker, tokenSvc)

    // SetupRouter với đầy đủ handlers
    r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, ahHandler, sseHandler, rhHandler, dhHandler)
    mux.Handle("/", r)
    return nil
}
```

### Tables cần có trong DB
```sql
-- alerts table cho in-app notifications
CREATE TABLE IF NOT EXISTS alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(36) NOT NULL,
    type        VARCHAR(50) NOT NULL,
    title       TEXT NOT NULL,
    message     TEXT,
    severity    VARCHAR(20) DEFAULT 'Info',
    is_read     BOOLEAN DEFAULT FALSE,
    entity_type VARCHAR(50),
    entity_id   VARCHAR(36),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    read_at     TIMESTAMPTZ
);
CREATE INDEX idx_alerts_user_id ON alerts(user_id);
CREATE INDEX idx_alerts_is_read ON alerts(user_id, is_read);
```

### Test cases
```go
func TestSetupRouter_NilAlertHandler_NoRouteConflict(t *testing.T) {
    r := SetupRouter(wh, sh, ih, nil, nil, rh, dh)
    req := httptest.NewRequest("GET", "/api/v2/notifications", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    // Should return 503, NOT panic
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
```
