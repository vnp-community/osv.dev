# SOL-SEED-006: Giải pháp thực thi — Config Seed (SLA, Notifications, JIRA)

> **CR:** [SEED-006-config-seed.md](../SEED-006-config-seed.md)  
> **Cập nhật:** 2026-06-18  
> **Domain:** `services/sla-service` + `services/notification-service` + `services/jira-service` + `apps/osv` (gateway)  
> **Priority:** 🟡 MEDIUM

---

## 1. Phân tích kiến trúc hiện tại

Các services này đã có **đầy đủ single-item CRUD**:
- `sla-service (`:8086)`: `POST /api/v2/sla-configurations` + `POST .../assign/{product_id}` — **✅ Có, cần bulk**.
- `notification-service (`:8087)`: `POST /api/v2/notification-rules`, `/api/v2/subscriptions`, `/api/v2/webhooks` — **✅ Có, cần bulk**.
- `jira-service (`:8088)`: `POST /api/v2/jira-configurations` — **✅ Có, cần bulk**.

Chiến lược thực thi: **Thêm bulk handlers** tái dùng existing usecase logic trong vòng lặp, wrapped trong một transaction (hoặc partial-failure 207 pattern).

---

## 2. Các thay đổi cần thực hiện

### 2.1 SLA-Service — Bulk endpoints

**File**: `services/sla-service/internal/delivery/http/handler.go` — Thêm 2 handlers:

```go
// BulkCreateConfigs handles POST /api/v2/sla-configurations/bulk
func (h *Handler) BulkCreateConfigs(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Configurations []SLAConfigInput `json:"configurations"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]BulkResult, 0, len(req.Configurations))
    for _, cfg := range req.Configurations {
        // Validate: chỉ 1 config có is_default=true
        id, err := h.uc.CreateConfig(r.Context(), cfg)
        if err != nil {
            results = append(results, BulkResult{Name: cfg.Name, Status: "error", Message: err.Error()})
        } else {
            results = append(results, BulkResult{Name: cfg.Name, Status: "created", ID: id})
        }
    }
    
    writeJSON(w, 207, map[string]any{
        "created_count": countCreated(results),
        "failed_count":  countFailed(results),
        "results":       results,
    })
}

// BulkAssign handles POST /api/v2/sla-configurations/assign-bulk
func (h *Handler) BulkAssign(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Assignments []struct {
            ProductID          uuid.UUID `json:"product_id"`
            SLAConfigurationID uuid.UUID `json:"sla_configuration_id"`
        } `json:"assignments"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]AssignResult, 0, len(req.Assignments))
    for _, a := range req.Assignments {
        err := h.uc.AssignToProduct(r.Context(), a.SLAConfigurationID, a.ProductID)
        status := "assigned"
        if err != nil {
            status = "error"
        }
        results = append(results, AssignResult{ProductID: a.ProductID, Status: status})
    }
    
    writeJSON(w, 207, map[string]any{"assigned_count": countAssigned(results), "results": results})
}
```

**Router** (`sla-service`):

```go
// Literal TRƯỚC wildcard
r.Post("/api/v2/sla-configurations/bulk",         h.BulkCreateConfigs)
r.Post("/api/v2/sla-configurations/assign-bulk",  h.BulkAssign)
```

---

### 2.2 Notification-Service — Bulk endpoints

**File**: `notification-service/internal/delivery/http/router.go` — Thêm routes và handlers:

#### Bulk Notification Rules

```go
// POST /api/v2/notification-rules/bulk
func (h *Handler) BulkCreateNotificationRules(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Rules []NotificationRuleInput `json:"rules"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]BulkResult, 0, len(req.Rules))
    for _, rule := range req.Rules {
        id, err := h.notifRuleUC.Create(r.Context(), userID(r), rule)
        results = append(results, toBulkResult(rule.ProductID.String(), id, err))
    }
    writeJSON(w, 207, bulkResponse(results))
}
```

#### Bulk Subscriptions

```go
// POST /api/v2/subscriptions/bulk
func (h *Handler) BulkCreateSubscriptions(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Subscriptions []SubscriptionInput `json:"subscriptions"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]BulkResult, 0, len(req.Subscriptions))
    for _, sub := range req.Subscriptions {
        id, err := h.subscriptionUC.Create(r.Context(), userID(r), sub)
        results = append(results, toBulkResult(sub.Type+":"+sub.Value, id, err))
    }
    writeJSON(w, 207, bulkResponse(results))
}
```

#### Bulk Webhooks

```go
// POST /api/v2/webhooks/bulk
func (h *Handler) BulkCreateWebhooks(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Webhooks []WebhookInput `json:"webhooks"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]WebhookBulkResult, 0, len(req.Webhooks))
    for _, wh := range req.Webhooks {
        // Validate URL: SSRF check (block private IPs) — reuse existing isPrivateIP()
        if isPrivateURL(wh.URL) {
            results = append(results, WebhookBulkResult{URL: wh.URL, Status: "error", Message: "private IP not allowed"})
            continue
        }
        
        webhook, secret, err := h.webhookUC.Create(r.Context(), userID(r), wh)
        if err != nil {
            results = append(results, WebhookBulkResult{URL: wh.URL, Status: "error"})
        } else {
            results = append(results, WebhookBulkResult{URL: wh.URL, Status: "created", ID: &webhook.ID, Secret: secret})
        }
    }
    writeJSON(w, 207, map[string]any{"created_count": countCreated(results), "results": results})
}
```

**Router** (`notification-service`):

```go
// Literal paths TRƯỚC wildcards trong chi router
r.Post("/api/v2/notification-rules/bulk", h.BulkCreateNotificationRules)
r.Post("/api/v2/subscriptions/bulk",      h.BulkCreateSubscriptions)
r.Post("/api/v2/webhooks/bulk",           h.BulkCreateWebhooks)
```

---

### 2.3 JIRA-Service — Bulk endpoint

**File**: `jira-service/internal/delivery/http/router.go` — Thêm:

```go
// POST /api/v2/jira-configurations/bulk
func (h *Handler) BulkCreateJiraConfigs(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Configurations []JiraConfigInput `json:"configurations"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]BulkResult, 0, len(req.Configurations))
    for _, cfg := range req.Configurations {
        // AES-256-GCM encrypt api_token TRƯỚC khi lưu
        encToken, err := h.cipher.Encrypt(cfg.APIToken)
        if err != nil {
            results = append(results, BulkResult{Name: cfg.ProductID.String(), Status: "error"})
            continue
        }
        cfg.EncryptedToken = encToken
        cfg.APIToken = ""  // clear plaintext
        
        id, err := h.configUC.Create(r.Context(), cfg)
        results = append(results, toBulkResult(cfg.ProductID.String(), id, err))
    }
    writeJSON(w, 207, bulkResponse(results))
}
```

> **Security**: `api_token` được clear sau khi encrypt. Response **không trả về** `api_token` hay `encrypted_token`.

---

### 2.4 Gateway Layer — `apps/osv/internal/gateway/router.go`

```go
// ═══════════════════════════════════════════════
// CONFIG SEED BULK ENDPOINTS (SEED-006)
// ═══════════════════════════════════════════════

// SLA Bulk (Admin only)
mux.Handle("POST /api/v2/sla-configurations/bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("sla-service:8086"), 5)))

mux.Handle("POST /api/v2/sla-configurations/assign-bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("sla-service:8086"), 5)))

// Notification Rules Bulk (authenticated user — per-product scope check in service)
mux.Handle("POST /api/v2/notification-rules/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 10)))

// Subscriptions Bulk
mux.Handle("POST /api/v2/subscriptions/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 10)))

// Webhooks Bulk
mux.Handle("POST /api/v2/webhooks/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 5)))

// JIRA Configs Bulk (Admin only — contains credentials)
mux.Handle("POST /api/v2/jira-configurations/bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("jira-service:8088"), 3)))
```

> ⚠️ **Route ordering**: Tất cả `*/bulk` và `*/assign-bulk` phải đứng TRƯỚC `*/{id}` routes trong router để tránh shadowing.

---

### 2.5 Shared helper — Bulk response format

Để thống nhất format response `207` across all services, tạo shared helper:

**File**: `services/shared/httputil/bulk_response.go` (NEW)

```go
package httputil

// BulkItem là kết quả của một item trong bulk operation
type BulkItem struct {
    Key     string     `json:"key,omitempty"`     // identifier (name, id, email...)
    Status  string     `json:"status"`             // "created" | "updated" | "error" | "skipped"
    ID      *uuid.UUID `json:"id,omitempty"`
    Message string     `json:"message,omitempty"` // error detail
}

// BulkResponse là response chuẩn 207 Multi-Status
type BulkResponse struct {
    CreatedCount int        `json:"created_count"`
    FailedCount  int        `json:"failed_count"`
    Results      []BulkItem `json:"results"`
}

func NewBulkResponse(items []BulkItem) BulkResponse {
    created, failed := 0, 0
    for _, i := range items {
        if i.Status == "created" || i.Status == "updated" {
            created++
        } else if i.Status == "error" {
            failed++
        }
    }
    return BulkResponse{CreatedCount: created, FailedCount: failed, Results: items}
}
```

---

### 2.6 Rate limits cho bulk endpoints

Cập nhật rate limit table theo kiến trúc hiện có (`01-architecture.md §3.1`):

| Endpoint | Rate Limit |
|---------|-----------|
| `POST /api/v2/sla-configurations/bulk` | 5/min |
| `POST /api/v2/sla-configurations/assign-bulk` | 5/min |
| `POST /api/v2/notification-rules/bulk` | 10/min |
| `POST /api/v2/subscriptions/bulk` | 10/min |
| `POST /api/v2/webhooks/bulk` | 5/min |
| `POST /api/v2/jira-configurations/bulk` | 3/min |

---

## 3. NATS Events mới

| Subject | Publisher | Consumers | Payload |
|---------|-----------|----------|---------|
| `sla.config.batch_created` | sla-service | audit-service | `{count, actor_id}` |
| `notification.rule.batch_created` | notification-service | audit-service | `{count, actor_id}` |
| `webhook.batch_created` | notification-service | audit-service | `{count, actor_id}` |
| `jira.config.batch_created` | jira-service | audit-service | `{count, actor_id}` |

---

## 4. File thay đổi tổng hợp

| File | Service | Thay đổi |
|------|---------|---------|
| `internal/delivery/http/handler.go` | sla-service | Thêm `BulkCreateConfigs`, `BulkAssign` |
| `internal/delivery/http/router.go` | sla-service | Thêm 2 routes (literal trước wildcard) |
| `internal/delivery/http/router.go` | notification-service | Thêm 3 bulk handlers + routes |
| `internal/delivery/http/router.go` | jira-service | Thêm `BulkCreateJiraConfigs` |
| `services/shared/httputil/bulk_response.go` | shared | **[NEW]** Shared helper |
| `apps/osv/internal/gateway/router.go` | gateway | Thêm 6 gateway routes |

---

## 5. Acceptance Criteria

1. `POST /api/v2/sla-configurations/bulk` với 2 configs → `207 {"created_count": 2}`.
2. Bulk SLA với 1 `is_default: true` → chỉ 1 config được set làm default (validation layer enforce).
3. `POST /api/v2/sla-configurations/assign-bulk` với 3 product_ids → `207 {"assigned_count": 3}`.
4. `POST /api/v2/notification-rules/bulk` với 5 rules → `207 {"created_count": 5}`.
5. `POST /api/v2/subscriptions/bulk` với `type: "kev"` → user nhận alert khi có KEV mới.
6. `POST /api/v2/webhooks/bulk` với URL là private IP (10.x.x.x) → entry đó `status: "error", message: "private IP not allowed"`.
7. `POST /api/v2/jira-configurations/bulk` → `api_token` không xuất hiện trong response.
8. Tất cả bulk endpoints trả về `207` ngay cả khi tất cả items fail — không return `500`.
9. `POST /api/v2/sla-configurations/bulk` đứng TRƯỚC `POST /api/v2/sla-configurations/{id}` — không bị shadowing.
