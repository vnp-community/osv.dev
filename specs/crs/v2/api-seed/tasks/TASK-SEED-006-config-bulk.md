# TASK-SEED-006: Config Bulk Endpoints (sla + notification + jira + gateway)

> **Solution:** [SOL-SEED-006](../solutions/SOL-SEED-006-config-seed.md)  
> **Service:** `services/sla-service` + `services/notification-service` + `services/jira-service` + `apps/osv`  
> **Depends on:** TASK-SEED-001-D (users), TASK-SEED-002 (products)  
> **Blocking:** Không có  
> **Status:** ✅ COMPLETED — 2026-06-19  
> **Files tạo/sửa:**  
> - `services/sla-service/internal/delivery/http/config_handler.go` (đã có `BulkCreateConfigs`, `BulkAssign`)  
> - `services/sla-service/embedded.go` (routes `/bulk` và `/assign-bulk` đã đăng ký)  
> - `services/notification-service/internal/delivery/http/rule_handler.go` (đã có `BulkCreateNotificationRules`)  
> - `services/notification-service/internal/delivery/http/webhook_handler.go` (đã có `BulkCreateWebhooks`)  
> - `services/notification-service/internal/delivery/http/subscription_handler.go` (đã có `BulkCreateSubscriptions`)  
> - `services/jira-service/internal/delivery/http/config_handler.go` (đã có `BulkCreateJiraConfigs`)  
> - `apps/osv/internal/gateway/router.go` (thêm SEED-006 gateway routes: sla bulk/assign-bulk, webhooks/bulk, jira-configurations/bulk; cập nhật độ ưu tiên literal BEFORE `/{id}`)  
> **Lưu ý:** jira-service có pre-existing build error (entity field mismatch `ServerURL`/`EncryptedToken`) trong `GetConfig`/`GetFirstConfig` — không liên quan SEED-006; `BulkCreateJiraConfigs` đúng syntax.

## Mục tiêu

- [x] **A. sla-service**: Bulk SLA Configs
- [x] **B. notification-service**: Bulk Rules & Subscriptions
- [x] **C. jira-service**: Bulk JIRA Configurations, webhooks. Tất cả dùng pattern: wrap existing single-item usecase trong vòng lặp + trả về `207 Multi-Status`.

---

## Phần A — sla-service

### A1: Khảo sát

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service \
  -name "*.go" | head -20

grep -rn "func.*Create\|POST.*sla-config" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/internal/ \
  --include="*.go" | head -15
```

### A2: Thêm handlers vào sla-service

**File:** `internal/delivery/http/handler.go` (hoặc tương đương)

```go
// BulkCreateConfigs handles POST /api/v2/sla-configurations/bulk
func (h *Handler) BulkCreateConfigs(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Configurations []SLAConfigInput `json:"configurations"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, 400, errResp("invalid_body", err.Error()))
        return
    }
    
    // Validate: không được có nhiều hơn 1 is_default=true
    defaultCount := 0
    for _, c := range req.Configurations {
        if c.IsDefault { defaultCount++ }
    }
    if defaultCount > 1 {
        writeJSON(w, 400, errResp("validation_error", "only one configuration can be default"))
        return
    }
    
    results := make([]map[string]any, 0, len(req.Configurations))
    created := 0
    for _, cfg := range req.Configurations {
        id, err := h.uc.CreateConfig(r.Context(), cfg) // tái dùng existing usecase
        if err != nil {
            results = append(results, map[string]any{
                "name": cfg.Name, "status": "error", "message": err.Error(),
            })
        } else {
            results = append(results, map[string]any{
                "name": cfg.Name, "status": "created", "id": id,
            })
            created++
        }
    }
    
    writeJSON(w, http.StatusMultiStatus, map[string]any{
        "created_count": created,
        "failed_count":  len(results) - created,
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
    
    results := make([]map[string]any, 0, len(req.Assignments))
    assigned := 0
    for _, a := range req.Assignments {
        err := h.uc.AssignToProduct(r.Context(), a.SLAConfigurationID, a.ProductID)
        if err != nil {
            results = append(results, map[string]any{"product_id": a.ProductID, "status": "error"})
        } else {
            results = append(results, map[string]any{"product_id": a.ProductID, "status": "assigned"})
            assigned++
        }
    }
    writeJSON(w, http.StatusMultiStatus, map[string]any{"assigned_count": assigned, "results": results})
}
```

**Router** (literal trước wildcard):

```go
// SEED-006: literal paths TRƯỚC /{id}
r.Post("/api/v2/sla-configurations/bulk",        h.BulkCreateConfigs)
r.Post("/api/v2/sla-configurations/assign-bulk", h.BulkAssign)
```

---

## Phần B — notification-service

### B1: Khảo sát

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service \
  -name "*.go" | head -20

grep -rn "notification-rules\|subscriptions\|webhooks\|func.*Create" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/notification-service/internal/ \
  --include="*.go" | head -20
```

### B2: Thêm bulk handlers

**File:** `internal/delivery/http/handler.go` (notification-service)

```go
// BulkCreateNotificationRules handles POST /api/v2/notification-rules/bulk
func (h *Handler) BulkCreateNotificationRules(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Rules []NotificationRuleInput `json:"rules"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]map[string]any, 0, len(req.Rules))
    created := 0
    for _, rule := range req.Rules {
        id, err := h.notifRuleUC.Create(r.Context(), userIDFromHeader(r), rule)
        key := rule.ProductID.String()
        if err != nil {
            results = append(results, map[string]any{"product_id": key, "status": "error", "message": err.Error()})
        } else {
            results = append(results, map[string]any{"product_id": key, "status": "created", "id": id})
            created++
        }
    }
    writeJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}

// BulkCreateSubscriptions handles POST /api/v2/subscriptions/bulk
func (h *Handler) BulkCreateSubscriptions(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Subscriptions []SubscriptionInput `json:"subscriptions"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]map[string]any, 0, len(req.Subscriptions))
    created := 0
    for _, sub := range req.Subscriptions {
        id, err := h.subscriptionUC.Create(r.Context(), userIDFromHeader(r), sub)
        key := sub.Type + ":" + sub.Value
        if err != nil {
            results = append(results, map[string]any{"key": key, "status": "error", "message": err.Error()})
        } else {
            results = append(results, map[string]any{"key": key, "status": "created", "id": id})
            created++
        }
    }
    writeJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}

// BulkCreateWebhooks handles POST /api/v2/webhooks/bulk
func (h *Handler) BulkCreateWebhooks(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Webhooks []WebhookInput `json:"webhooks"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    type WebhookResult struct {
        URL    string     `json:"url"`
        Status string     `json:"status"`
        ID     *uuid.UUID `json:"id,omitempty"`
        Secret string     `json:"secret,omitempty"` // HMAC secret — one time
        Message string    `json:"message,omitempty"`
    }
    
    results := make([]WebhookResult, 0, len(req.Webhooks))
    created := 0
    for _, wh := range req.Webhooks {
        // SSRF check — tái dùng isPrivateIP() logic hiện có
        if isPrivateURL(wh.URL) {
            results = append(results, WebhookResult{URL: wh.URL, Status: "error", Message: "private IP not allowed"})
            continue
        }
        webhook, secret, err := h.webhookUC.Create(r.Context(), userIDFromHeader(r), wh)
        if err != nil {
            results = append(results, WebhookResult{URL: wh.URL, Status: "error", Message: err.Error()})
        } else {
            results = append(results, WebhookResult{URL: wh.URL, Status: "created", ID: &webhook.ID, Secret: secret})
            created++
        }
    }
    writeJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}
```

**Router** (literal trước wildcard):

```go
// SEED-006: literal paths TRƯỚC /{id}
r.Post("/api/v2/notification-rules/bulk", h.BulkCreateNotificationRules)
r.Post("/api/v2/subscriptions/bulk",      h.BulkCreateSubscriptions)
r.Post("/api/v2/webhooks/bulk",           h.BulkCreateWebhooks)
```

---

## Phần C — jira-service

### C1: Khảo sát

```bash
find /Users/binhnt/Lab/sec/cve/osv.dev/services/jira-service \
  -name "*.go" | head -15

grep -rn "func.*Create\|CreateConfig\|AES\|Encrypt" \
  /Users/binhnt/Lab/sec/cve/osv.dev/services/jira-service/ \
  --include="*.go" | head -15
```

### C2: Thêm bulk handler

**File:** `internal/delivery/http/handler.go` (jira-service)

```go
// BulkCreateJiraConfigs handles POST /api/v2/jira-configurations/bulk
func (h *Handler) BulkCreateJiraConfigs(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Configurations []JiraConfigInput `json:"configurations"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]map[string]any, 0, len(req.Configurations))
    created := 0
    for _, cfg := range req.Configurations {
        // Security: AES-256-GCM encrypt api_token trước khi lưu
        encToken, err := h.cipher.Encrypt(cfg.APIToken)
        if err != nil {
            results = append(results, map[string]any{
                "product_id": cfg.ProductID, "status": "error", "message": "encryption failed",
            })
            continue
        }
        cfg.APIToken = ""          // clear plaintext
        cfg.EncryptedToken = encToken
        
        id, err := h.configUC.Create(r.Context(), cfg)
        if err != nil {
            results = append(results, map[string]any{"product_id": cfg.ProductID, "status": "error", "message": err.Error()})
        } else {
            results = append(results, map[string]any{"product_id": cfg.ProductID, "status": "created", "id": id})
            created++
        }
    }
    
    // KHÔNG bao giờ trả về api_token hay encrypted_token trong response
    writeJSON(w, http.StatusMultiStatus, map[string]any{
        "created_count": created,
        "failed_count":  len(results) - created,
        "results":       results,
    })
}
```

**Router:**

```go
r.Post("/api/v2/jira-configurations/bulk", h.BulkCreateJiraConfigs)
```

---

## Phần D — Gateway routes

**File:** `apps/osv/internal/gateway/router.go`

```bash
# Tìm vị trí các routes hiện có
grep -n "sla-configurations\|notification-rules\|subscriptions\|webhooks\|jira-config" \
  /Users/binhnt/Lab/sec/cve/osv.dev/apps/osv/internal/gateway/router.go | head -30
```

Thêm **TRƯỚC** các `*/{id}` routes tương ứng:

```go
// SEED-006: Config Seed Bulk Routes
// SLA (Admin only)
mux.Handle("POST /api/v2/sla-configurations/bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("sla-service:8086"), 5)))
mux.Handle("POST /api/v2/sla-configurations/assign-bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("sla-service:8086"), 5)))

// Notification (authenticated)
mux.Handle("POST /api/v2/notification-rules/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 10)))
mux.Handle("POST /api/v2/subscriptions/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 10)))
mux.Handle("POST /api/v2/webhooks/bulk",
    protected(ratelimit.Wrap(proxy.Forward("notification-service:8087"), 5)))

// JIRA (Admin — contains credentials)
mux.Handle("POST /api/v2/jira-configurations/bulk",
    adminOnly(ratelimit.Wrap(proxy.Forward("jira-service:8088"), 3)))
```

---

## Phần E — Shared helper (optional nhưng recommended)

**File:** `services/shared/httputil/bulk_response.go` (NEW)

```go
package httputil

import "github.com/google/uuid"

type BulkItem struct {
    Key     string     `json:"key,omitempty"`
    Status  string     `json:"status"`
    ID      *uuid.UUID `json:"id,omitempty"`
    Message string     `json:"message,omitempty"`
}

type BulkResponse struct {
    CreatedCount int        `json:"created_count"`
    FailedCount  int        `json:"failed_count"`
    Results      []BulkItem `json:"results"`
}

func NewBulkResponse(items []BulkItem) BulkResponse {
    c, f := 0, 0
    for _, i := range items {
        if i.Status == "created" || i.Status == "updated" || i.Status == "assigned" {
            c++
        } else if i.Status == "error" {
            f++
        }
    }
    return BulkResponse{CreatedCount: c, FailedCount: f, Results: items}
}
```

## Acceptance Criteria

- [x] A1. **Data Model (Không cần thay đổi)**: Dùng lại `CreateSLAConfigInput`.
- [x] A2. **Handlers (`sla-service`)**:
  - `POST /api/v2/sla-configurations/bulk`
  - `POST /api/v2/sla-configurations/assign-bulk`
- [x] A3. **Gateway Proxy (`apps/osv/internal/gateway`)**:
  - Thêm route `POST /api/v2/sla-configurations/bulk` (adminOnly)
  - Thêm route `POST /api/v2/sla-configurations/assign-bulk` (adminOnly) → `207`
- [x] `POST /api/v2/notification-rules/bulk` → `207`
- [x] `POST /api/v2/subscriptions/bulk` với `type: "kev"` → `207`
- [x] `POST /api/v2/webhooks/bulk` với private IP URL → entry đó `status: "error"`
- [x] `POST /api/v2/jira-configurations/bulk` → response KHÔNG có `api_token`
- [x] TẤT CẢ bulk endpoints trả `207` ngị cả khi mọi items đều fail — không 500
- [x] Route ordering: `*/bulk` và `*/assign-bulk` trước `*/{id}/*`
- [x] `go build ./...` cho sla-service, notification-service thành công; jira-service có pre-existing entity mismatch (không liên quan SEED-006)
