# TASK-BE-014 — jira-service: JIRA Config HTTP Endpoints

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-014 |
| **Service** | `services/jira-service` |
| **Solution Ref** | [SOL-UI-004 §4](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 3h |

---

## Context

UI Admin Panel > Integrations cần quản lý JIRA config qua HTTP. jira-service hiện có entity JiraConfig và repository nhưng chưa có HTTP handler.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/jira-service/internal/adapter/http/config_handler.go` |
| MODIFY | `services/jira-service/internal/adapter/http/router.go` |

---

## Implementation

```go
// services/jira-service/internal/adapter/http/config_handler.go

package http

import (
	"encoding/json"
	"net/http"
	"time"
)

type ConfigHandler struct {
	configRepo  JiraConfigRepository
	jiraClient  JiraAPIClient  // existing JIRA HTTP client in jira-service
	crypto      CryptoService  // AES-256-GCM encrypt/decrypt
	platformURL string         // for webhook URL construction
}

// GET /jira-configs → alias /jira/config
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configRepo.FindFirst(r.Context())
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "No JIRA configuration found")
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"id":                  cfg.ID,
		"jira_url":            cfg.ServerURL,
		"project_key":         cfg.ProjectKey,
		"username":            cfg.Username,
		"api_token_preview":   maskToken(cfg.EncryptedToken), // "ATATT3x...xxx" → show first+last
		"is_active":           cfg.Active,
		"webhook_url":         h.platformURL + "/api/v1/jira/webhook",
		"created_at":          cfg.CreatedAt.Format(time.RFC3339),
		"last_sync_at":        formatTime(cfg.LastSyncAt),
	})
}

// POST /jira/config
func (h *ConfigHandler) CreateOrUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JiraURL    string `json:"jira_url"`
		ProjectKey string `json:"project_key"`
		Username   string `json:"username"`
		APIToken   string `json:"api_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if req.JiraURL == "" || req.Username == "" || req.APIToken == "" {
		respondError(w, 400, "VALIDATION_ERROR", "jira_url, username, and api_token are required")
		return
	}

	// Test connection first
	if err := h.jiraClient.TestConnection(r.Context(), req.JiraURL, req.Username, req.APIToken); err != nil {
		respondError(w, 400, "JIRA_CONNECTION_FAILED",
			"Cannot connect to JIRA: "+err.Error())
		return
	}

	// Encrypt token
	encToken, err := h.crypto.Encrypt([]byte(req.APIToken))
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Encryption failed")
		return
	}

	cfg := &JiraConfig{
		ServerURL:      req.JiraURL,
		ProjectKey:     req.ProjectKey,
		Username:       req.Username,
		EncryptedToken: encToken,
		Active:         true,
		WebhookURL:     h.platformURL + "/api/v1/jira/webhook",
	}

	if err := h.configRepo.Upsert(r.Context(), cfg); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to save configuration")
		return
	}

	respondJSON(w, 201, map[string]interface{}{
		"id":          cfg.ID,
		"jira_url":    cfg.ServerURL,
		"project_key": cfg.ProjectKey,
		"webhook_url": cfg.WebhookURL,
		"is_active":   cfg.Active,
		"created_at":  cfg.CreatedAt.Format(time.RFC3339),
	})
}

// POST /jira/config/test
func (h *ConfigHandler) TestConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configRepo.FindFirst(r.Context())
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "No JIRA configuration found. Create one first.")
		return
	}

	// Decrypt token for test
	plainToken, err := h.crypto.Decrypt(cfg.EncryptedToken)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to decrypt token")
		return
	}

	start := time.Now()
	jiraVersion, projectFound, err := h.jiraClient.GetVersionAndProject(
		r.Context(), cfg.ServerURL, cfg.Username, string(plainToken), cfg.ProjectKey,
	)
	elapsed := time.Since(start)

	if err != nil {
		respondJSON(w, 200, map[string]interface{}{
			"success":          false,
			"error":            err.Error(),
			"response_time_ms": elapsed.Milliseconds(),
		})
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"success":          true,
		"jira_version":     jiraVersion,
		"project_found":    projectFound,
		"response_time_ms": elapsed.Milliseconds(),
	})
}

// ─── Helpers ─────────────────────────────────

func maskToken(encryptedToken string) string {
	if len(encryptedToken) < 16 {
		return "***"
	}
	// Show "ATxx...xxxx" pattern — only first 4 and last 4 chars
	return encryptedToken[:4] + "..." + encryptedToken[len(encryptedToken)-4:]
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
```

### Router:

```go
// services/jira-service/internal/adapter/http/router.go
mux.HandleFunc("GET  /jira/config",      h.Config.GetConfig)
mux.HandleFunc("POST /jira/config",      h.Config.CreateOrUpdateConfig)
mux.HandleFunc("POST /jira/config/test", h.Config.TestConfig)
// Legacy path compatibility:
mux.HandleFunc("GET  /jira-configs",     h.Config.GetConfig)
```

---

## Verification

```bash
cd services/jira-service
go build ./...

# Create config
curl -X POST http://localhost:8088/jira/config \
  -H "X-User-Role: admin" \
  -H "Content-Type: application/json" \
  -d '{"jira_url":"https://acme.atlassian.net","project_key":"SEC","username":"user@acme.com","api_token":"ATATT..."}' | jq .

# Test config
curl -X POST http://localhost:8088/jira/config/test \
  -H "X-User-Role: admin" | jq '.success'
```

---

## Checklist

- [x] `GetConfig` redact API token — chỉ show `api_token_preview` (first 4 + last 4 chars)
- [x] `CreateOrUpdateConfig` test connection TRƯỚC khi lưu — return 400 nếu fail
- [x] API token encrypted với AES-256-GCM trước khi lưu vào DB
- [x] `TestConfig` decrypt token, test `jira_url` + `project_key`
- [x] Response `test/config` trả về `success`, `jira_version`, `project_found`, `response_time_ms`
- [x] `go build ./...` thành công

---

# TASK-BE-015 — audit-service: Audit Log HTTP List Endpoint

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-015 |
| **Service** | `services/audit-service` |
| **Solution Ref** | [SOL-UI-004 §3](../solutions/SOL-UI-004-finding-product-reports-admin.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 2h |

---

## Context

audit-service ghi nhận tất cả thay đổi hệ thống nhưng chưa expose HTTP để UI đọc. Admin panel cần `GET /api/v1/audit-log` có filter + pagination.

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `services/audit-service/internal/adapter/http/handler.go` |
| MODIFY | `services/audit-service/internal/adapter/http/router.go` |

---

## Implementation

```go
// services/audit-service/internal/adapter/http/handler.go

type AuditHandler struct {
	auditRepo AuditRepository
}

// GET /audit-log → /api/v1/audit-log
func (h *AuditHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	filter := AuditFilter{
		UserID:     r.URL.Query().Get("user_id"),
		Action:     r.URL.Query().Get("action"),
		EntityType: r.URL.Query().Get("entity_type"),
		EntityID:   r.URL.Query().Get("entity_id"),
		DateFrom:   parseTimeParam(r.URL.Query().Get("date_from")),
		DateTo:     parseTimeParam(r.URL.Query().Get("date_to")),
	}
	page, ps := parsePagination(r)

	events, total, err := h.auditRepo.List(r.Context(), filter, page, ps)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"events":    mapEvents(events),
		"total":     total,
		"page":      page,
		"page_size": ps,
	})
}

// AuditEventDTO — response shape
type AuditEventDTO struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	UserEmail  string      `json:"user_email"`
	Action     string      `json:"action"`      // "finding.status.changed" | "user.role.updated" | ...
	EntityType string      `json:"entity_type"` // "finding" | "user" | "product"
	EntityID   string      `json:"entity_id"`
	OldValue   interface{} `json:"old_value"`
	NewValue   interface{} `json:"new_value"`
	IPAddress  string      `json:"ip_address"`
	UserAgent  string      `json:"user_agent"`
	CreatedAt  string      `json:"created_at"`
}
```

### SQL:

```sql
-- audit-service list query
SELECT id, user_id, user_email, action, entity_type, entity_id,
       old_value, new_value, ip_address, user_agent, created_at
FROM audit_events
WHERE
    ($1::text IS NULL OR user_id = $1)
    AND ($2::text IS NULL OR action = $2)
    AND ($3::text IS NULL OR entity_type = $3)
    AND ($4::text IS NULL OR entity_id = $4)
    AND ($5::timestamptz IS NULL OR created_at >= $5)
    AND ($6::timestamptz IS NULL OR created_at <= $6)
ORDER BY created_at DESC
LIMIT $7 OFFSET ($8 - 1) * $7;
```

### Router:

```go
mux.HandleFunc("GET /audit-log", h.Audit.ListAuditLog)
```

---

## Verification

```bash
cd services/audit-service
go build ./...

curl -H "X-User-Role: admin" \
  "http://localhost:8090/audit-log?entity_type=finding&page=1&page_size=20" | jq '.total'
```

---

## Checklist

- [x] `ListAuditLog` support filters: `user_id`, `action`, `entity_type`, `entity_id`, `date_from`, `date_to`
- [x] Pagination với max `page_size` = 200
- [x] `AuditEventDTO` có `old_value`, `new_value` để xem thay đổi
- [x] SQL parameterized với NULL handling (allow NULL filter = no filter)
- [x] `go build ./...` thành công
