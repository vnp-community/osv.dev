# TASK-GCV-031 — Notification HTTP API + Router

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-031 |
| **Service** | `notification-service` |
| **CR** | CR-GCV-006 |
| **Phase** | 3 — Notifications |
| **Priority** | 🟡 Medium |
| **Prerequisites** | TASK-GCV-030 |

## Context

Implement HTTP API cho `notification-service`: webhook CRUD, subscription CRUD, delivery history, và test ping endpoint. Tất cả endpoints yêu cầu JWT authentication (owner scoped).

## Reference

- Solution: [SOL-GCV-006](../solutions/SOL-GCV-006-notification-webhook.md) §4.4

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/webhook_handler.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/subscription_handler.go
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/delivery/http/router.go
        (hoặc MODIFY nếu đã tồn tại)
```

## Implementation Spec

### webhook_handler.go

```go
package http

type WebhookHandler struct {
    registerUC  *usecase.RegisterWebhookUseCase
    deliverer   *usecase.WebhookDeliverer
    webhookRepo repository.WebhookRepository
}

// POST /api/v2/webhooks
// Body: {"url":"https://example.com/webhook","events":["kev.new","cve.new.critical"],"secret":"optional"}
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil {
        respondError(w, 401, "authentication required")
        return
    }

    var req struct {
        URL    string   `json:"url"`
        Events []string `json:"events"`
        Secret string   `json:"secret"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    if req.URL == "" {
        respondError(w, 400, "url is required")
        return
    }
    events := make([]entity.EventType, len(req.Events))
    for i, e := range req.Events { events[i] = entity.EventType(e) }

    wh, err := h.registerUC.Execute(r.Context(), usecase.RegisterWebhookInput{
        URL:     req.URL,
        Events:  events,
        Secret:  req.Secret,
        OwnerID: claims.UserID,
    })

    switch {
    case errors.Is(err, usecase.ErrInsecureURL):
        respondError(w, 400, "webhook URL must use HTTPS")
    case errors.Is(err, usecase.ErrSSRFBlocked):
        respondError(w, 400, "webhook URL points to a private network (SSRF protection)")
    case errors.Is(err, usecase.ErrPingFailed):
        respondError(w, 400, "webhook URL is not reachable")
    case err != nil:
        respondError(w, 500, "failed to register webhook")
    default:
        respondJSON(w, 201, wh)
    }
}

// GET /api/v2/webhooks — list owner's webhooks
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil { respondError(w, 401, ""); return }

    webhooks, err := h.webhookRepo.FindByOwner(r.Context(), claims.UserID)
    if err != nil { respondError(w, 500, "failed to list webhooks"); return }
    respondJSON(w, 200, map[string]interface{}{"webhooks": webhooks})
}

// DELETE /api/v2/webhooks/{id} — revoke webhook
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil { respondError(w, 401, ""); return }

    id := chi.URLParam(r, "id")
    if err := h.webhookRepo.Delete(r.Context(), id, claims.UserID); err != nil {
        respondError(w, 404, "webhook not found")
        return
    }
    w.WriteHeader(204)
}

// GET /api/v2/webhooks/{id}/deliveries — delivery history
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil { respondError(w, 401, ""); return }

    id    := chi.URLParam(r, "id")
    limit := parseInt(r.URL.Query().Get("limit"), 50)

    deliveries, err := h.webhookRepo.ListDeliveries(r.Context(), id, limit)
    if err != nil { respondError(w, 500, ""); return }
    respondJSON(w, 200, map[string]interface{}{"deliveries": deliveries})
}

// POST /api/v2/webhooks/{id}/test — send test ping event
func (h *WebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    if claims == nil { respondError(w, 401, ""); return }

    id := chi.URLParam(r, "id")
    err := h.deliverer.Deliver(r.Context(), usecase.DeliveryInput{
        WebhookID: id,
        EventType: "webhook.test",
        CVEID:     "test",
        Payload:   map[string]interface{}{"message": "Test ping from GlobalCVE"},
    })
    if err != nil {
        respondError(w, 502, "test delivery failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]string{"status": "delivered"})
}
```

### subscription_handler.go

```go
// POST /api/v2/subscriptions
// Body: {"type":"vendor","value":"apache","min_severity":"CRITICAL","min_epss":0.8}
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) { ... }

// GET /api/v2/subscriptions
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) { ... }

// DELETE /api/v2/subscriptions/{id}
func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) { ... }
```

### router.go — Route Table

```go
func SetupRouter(wh *WebhookHandler, sh *SubscriptionHandler) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RealIP, middleware.Recoverer)
    r.Use(authMiddleware)

    // Webhook management
    r.Route("/api/v2/webhooks", func(r chi.Router) {
        r.Get("/",                  wh.List)
        r.Post("/",                 wh.Create)
        r.Delete("/{id}",           wh.Delete)
        r.Get("/{id}/deliveries",   wh.ListDeliveries)
        r.Post("/{id}/test",        wh.Test)
    })

    // Subscription management
    r.Route("/api/v2/subscriptions", func(r chi.Router) {
        r.Get("/",      sh.List)
        r.Post("/",     sh.Create)
        r.Delete("/{id}", sh.Delete)
    })

    // Health
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        respondJSON(w, 200, map[string]string{"status": "ok"})
    })

    return r
}
```

## Acceptance Criteria

- [x] `POST /api/v2/webhooks {"url":"http://..."}` → 400 "must use HTTPS"
- [x] `POST /api/v2/webhooks {"url":"https://10.0.0.1/..."}` → 400 SSRF protected
- [x] `POST /api/v2/webhooks {"url":"https://example.com/wh","events":["kev.new"]}` → 201
- [x] `GET /api/v2/webhooks` → owner's webhooks only
- [x] `DELETE /api/v2/webhooks/{id}` wrong owner → 404
- [x] `GET /api/v2/webhooks/{id}/deliveries` → delivery history
- [x] `POST /api/v2/webhooks/{id}/test` → test ping sent, 200
- [x] `POST /api/v2/subscriptions {"type":"vendor","value":"apache"}` → 201
- [x] `DELETE /api/v2/subscriptions/{id}` → 204
- [x] `/health` → 200 `{"status":"ok"}`
- [x] Unauthenticated requests → 401
- [x] `go build ./...` pass


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Verified directly from codebase.
