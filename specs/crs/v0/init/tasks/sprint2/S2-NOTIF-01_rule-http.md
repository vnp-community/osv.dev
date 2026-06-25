# S2-NOTIF-01 — Thêm Rule Management HTTP Endpoints (notification-service)


## ✅ Execution Status: COMPLETED
## Metadata
- **Task ID**: S2-NOTIF-01
- **Service**: notification-service
- **Sprint**: 2 (P1)
- **Ước tính**: 3 giờ
- **Dependencies**: S1-NOTIF-01 (PostgreSQL rule repo phải có trước)
- **Spec nguồn**: `specs/develop/07_notification-service-upgrade.md` § "P1 — Thêm: HTTP Endpoints cho Rule Management"

## Context

```bash
# Đọc existing delivery:
cat services/notification-service/internal/delivery/http/integration_handler.go

# Đọc domain rule entity:
cat services/notification-service/internal/domain/rule/entity.go

# Đọc manage_subscription UC:
cat services/notification-service/internal/usecase/manage_subscription/register.go

# Đọc main.go để biết router setup:
cat services/notification-service/cmd/server/main.go
```

## Goal

Thêm HTTP handlers cho CRUD notification rules + alert history. Sử dụng PostgreSQL repos từ S1-NOTIF-01.

## Files to Create

### File 1: `services/notification-service/internal/delivery/http/rule_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/infra/persistence/postgres"
)

// RuleHandler handles notification rule CRUD operations.
type RuleHandler struct {
	ruleRepo *postgres.PostgresRuleRepo
	log      zerolog.Logger
}

// NewRuleHandler creates a new RuleHandler.
func NewRuleHandler(ruleRepo *postgres.PostgresRuleRepo, log zerolog.Logger) *RuleHandler {
	return &RuleHandler{ruleRepo: ruleRepo, log: log}
}

// List handles GET /rules
// Returns all active rules for the authenticated user.
func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := extractUserIDFromContext(r)  // from JWT middleware
	if userID == uuid.Nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rules, err := h.ruleRepo.FindByUserID(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("rule.List")
		http.Error(w, `{"error":"failed to list rules"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// Create handles POST /rules
func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := extractUserIDFromContext(r)
	if userID == uuid.Nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	nr := req.ToDomainRule(userID)
	if err := h.ruleRepo.Create(r.Context(), nr); err != nil {
		h.log.Error().Err(err).Msg("rule.Create")
		http.Error(w, `{"error":"failed to create rule"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(nr)
}

// Get handles GET /rules/{id}
func (h *RuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, `{"error":"invalid rule ID"}`, http.StatusBadRequest)
		return
	}

	rule, err := h.ruleRepo.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"rule not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// Update handles PUT /rules/{id}
func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, `{"error":"invalid rule ID"}`, http.StatusBadRequest)
		return
	}

	var req UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	nr := req.ToDomainRule(id)
	if err := h.ruleRepo.Update(r.Context(), nr); err != nil {
		http.Error(w, `{"error":"failed to update rule"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nr)
}

// Delete handles DELETE /rules/{id}
// Soft-delete: sets is_active = false
func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, `{"error":"invalid rule ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.ruleRepo.Delete(r.Context(), id); err != nil {
		http.Error(w, `{"error":"failed to delete rule"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Request/Response types ===

// CreateRuleRequest is the request body for POST /rules
type CreateRuleRequest struct {
	ProductID            *uuid.UUID `json:"product_id,omitempty"`
	ScanAdded            []string   `json:"scan_added"`
	FindingAdded         []string   `json:"finding_added"`
	FindingStatusChanged []string   `json:"finding_status_changed"`
	SLABreach            []string   `json:"sla_breach"`
	SLAExpiringSoon      []string   `json:"sla_expiring_soon"`
}

// ToDomainRule converts request to domain entity.
func (r *CreateRuleRequest) ToDomainRule(userID uuid.UUID) *rule.NotificationRule {
	nr := &rule.NotificationRule{
		UserID:               userID,
		ScanAdded:            r.ScanAdded,
		FindingAdded:         r.FindingAdded,
		FindingStatusChanged: r.FindingStatusChanged,
		SLABreach:            r.SLABreach,
		SLAExpiringSoon:      r.SLAExpiringSoon,
		IsActive:             true,
	}
	if r.ProductID != nil {
		nr.ProductID = *r.ProductID
	}
	return nr
}

// UpdateRuleRequest is the request body for PUT /rules/{id}
type UpdateRuleRequest struct {
	ScanAdded            []string `json:"scan_added"`
	FindingAdded         []string `json:"finding_added"`
	FindingStatusChanged []string `json:"finding_status_changed"`
	SLABreach            []string `json:"sla_breach"`
	SLAExpiringSoon      []string `json:"sla_expiring_soon"`
}

// ToDomainRule converts update request to domain entity.
func (r *UpdateRuleRequest) ToDomainRule(id uuid.UUID) *rule.NotificationRule {
	return &rule.NotificationRule{
		ID:                   id,
		ScanAdded:            r.ScanAdded,
		FindingAdded:         r.FindingAdded,
		FindingStatusChanged: r.FindingStatusChanged,
		SLABreach:            r.SLABreach,
		SLAExpiringSoon:      r.SLAExpiringSoon,
	}
}

// extractUserIDFromContext extracts user ID from JWT context.
// Adjust based on how JWT middleware stores user info.
func extractUserIDFromContext(r *http.Request) uuid.UUID {
	// Pattern 1: từ context key (most common)
	if id, ok := r.Context().Value("user_id").(string); ok {
		if uid, err := uuid.Parse(id); err == nil {
			return uid
		}
	}
	return uuid.Nil
}
```

### File 2: `services/notification-service/internal/delivery/http/alert_handler.go`

```go
package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/infra/persistence/postgres"
)

// AlertHandler handles alert history queries.
type AlertHandler struct {
	alertRepo *postgres.PostgresAlertRepo
	log       zerolog.Logger
}

// NewAlertHandler creates a new AlertHandler.
func NewAlertHandler(alertRepo *postgres.PostgresAlertRepo, log zerolog.Logger) *AlertHandler {
	return &AlertHandler{alertRepo: alertRepo, log: log}
}

// List handles GET /alerts
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			offset = (n - 1) * limit
		}
	}

	alerts, err := h.alertRepo.ListRecent(r.Context(), limit, offset)
	if err != nil {
		h.log.Error().Err(err).Msg("alert.List")
		http.Error(w, `{"error":"failed to list alerts"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
	})
}

// Get handles GET /alerts/{id}
func (h *AlertHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, `{"error":"invalid alert ID"}`, http.StatusBadRequest)
		return
	}

	alert, err := h.alertRepo.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"alert not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alert)
}
```

### File 3: `services/notification-service/internal/delivery/http/router.go`

```go
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/infra/persistence/postgres"
)

// RouterDeps holds HTTP handler dependencies.
type RouterDeps struct {
	RuleRepo  *postgres.PostgresRuleRepo
	AlertRepo *postgres.PostgresAlertRepo
	Log       zerolog.Logger
}

// NewRouter creates the HTTP router for notification-service.
// NOTE: Existing Jira handler is wired in separately in main.go
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	ruleH := NewRuleHandler(deps.RuleRepo, deps.Log)
	alertH := NewAlertHandler(deps.AlertRepo, deps.Log)

	// Rule management
	r.Get("/rules", ruleH.List)
	r.Post("/rules", ruleH.Create)
	r.Get("/rules/{id}", ruleH.Get)
	r.Put("/rules/{id}", ruleH.Update)
	r.Delete("/rules/{id}", ruleH.Delete)

	// Alert history
	r.Get("/alerts", alertH.List)
	r.Get("/alerts/{id}", alertH.Get)

	// Health
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}
```

## Files to Extend

### Extend: `services/notification-service/cmd/server/main.go`

```go
// Thêm HTTP server (nếu chưa có hoặc chỉ có Jira handler):

httpRouter := notif_http.NewRouter(notif_http.RouterDeps{
    RuleRepo:  ruleRepo,   // postgres repo từ S1-NOTIF-01
    AlertRepo: alertRepo,  // postgres repo từ S1-NOTIF-01
    Log:       logger,
})

go func() {
    log.Info().Str("addr", ":8087").Msg("notification HTTP server starting")
    http.ListenAndServe(":8087", httpRouter)
}()
```

## Verification

```bash
cd services/notification-service && go build ./...

# Test với RULE_BACKEND=postgres:
export RULE_BACKEND=postgres

# Create rule:
curl -X POST http://localhost:8087/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"finding_added":["email","slack"],"sla_breach":["email"]}'
# Expected: 201 Created với rule JSON

# List rules:
curl http://localhost:8087/rules -H "Authorization: Bearer TOKEN"
# Expected: {"rules":[...],"total":1}

# Get specific rule:
curl http://localhost:8087/rules/RULE_ID
# Expected: rule JSON

# Delete rule:
curl -X DELETE http://localhost:8087/rules/RULE_ID
# Expected: 204 No Content

# List alerts:
curl http://localhost:8087/alerts
# Expected: {"alerts":[...]}
```

## Notes

- `extractUserIDFromContext()` cần điều chỉnh theo cách JWT middleware lưu user_id trong context
- Nếu không có JWT middleware trong notification-service, có thể skip auth check cho MVP và thêm sau
- `rule.NotificationRule` import cần điều chỉnh theo actual package path
