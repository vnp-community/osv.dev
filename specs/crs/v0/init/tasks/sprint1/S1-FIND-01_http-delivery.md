# S1-FIND-01 — Thêm HTTP Delivery Layer (finding-service)

## ✅ Execution Status: COMPLETED
- **Executed**: 2026-06-13
- **Result**: `go build` PASSED


## Metadata
- **Task ID**: S1-FIND-01
- **Service**: finding-service
- **Sprint**: 1 (P0 — Blocking vì gateway cần HTTP)
- **Ước tính**: 4-5 giờ
- **Dependencies**: Không có
- **Spec nguồn**: `specs/develop/05_finding-service-upgrade.md` § "P0 — Thêm: HTTP Delivery Layer"

## Context

```bash
# Đọc existing gRPC handler để hiểu response patterns:
cat services/finding-service/internal/delivery/grpc/server/finding_server.go

# Đọc use cases để biết methods available:
cat services/finding-service/internal/usecase/finding/use_cases.go
cat services/finding-service/internal/usecase/sla/sla_use_cases.go

# Đọc HTTP pattern của services khác (e.g., identity-service):
cat services/identity-service/internal/adapter/handler/http/auth_handler.go
cat services/identity-service/internal/adapter/handler/http/router.go

# Đọc domain entity để biết JSON fields:
cat services/finding-service/internal/domain/finding/entity.go
```

## Goal

Thêm HTTP REST delivery layer vào finding-service, expose trên port 8085. gRPC server giữ nguyên.

## Files to Create

### File 1: `services/finding-service/internal/delivery/http/middleware.go`

```go
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// CommonMiddleware returns standard middleware chain.
func CommonMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		chain := middleware.RequestID(
			middleware.RealIP(
				middleware.Logger(
					middleware.Recoverer(next),
				),
			),
		)
		return chain
	}
}

// respondJSON writes a JSON response.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
```

### File 2: `services/finding-service/internal/delivery/http/finding_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/usecase/finding"
)

// FindingHandler handles HTTP requests for findings.
type FindingHandler struct {
	uc  *finding.UseCases  // adjust to actual type name
	log zerolog.Logger
}

// NewFindingHandler creates a new FindingHandler.
func NewFindingHandler(uc *finding.UseCases, log zerolog.Logger) *FindingHandler {
	return &FindingHandler{uc: uc, log: log}
}

// List handles GET /findings
func (h *FindingHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query params: page, limit, severity, status, product_id
	findings, err := h.uc.List(r.Context(), finding.ListFilter{
		Page:  parseIntParam(r, "page", 1),
		Limit: parseIntParam(r, "limit", 20),
	})
	if err != nil {
		h.log.Error().Err(err).Msg("finding.List")
		respondError(w, http.StatusInternalServerError, "failed to list findings")
		return
	}
	respondJSON(w, http.StatusOK, findings)
}

// Get handles GET /findings/{id}
func (h *FindingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}

	f, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "finding not found")
		return
	}
	respondJSON(w, http.StatusOK, f)
}

// Create handles POST /findings
func (h *FindingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req finding.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	f, err := h.uc.Create(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("finding.Create")
		respondError(w, http.StatusInternalServerError, "failed to create finding")
		return
	}
	respondJSON(w, http.StatusCreated, f)
}

// Update handles PUT /findings/{id}
func (h *FindingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}

	var req finding.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id

	f, err := h.uc.Update(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update finding")
		return
	}
	respondJSON(w, http.StatusOK, f)
}

// Verify handles POST /findings/{id}/verify
func (h *FindingHandler) Verify(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}
	if err := h.uc.Verify(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to verify finding")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Mitigate handles POST /findings/{id}/mitigate
func (h *FindingHandler) Mitigate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}
	if err := h.uc.Mitigate(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to mitigate finding")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AcceptRisk handles POST /findings/{id}/accept-risk
func (h *FindingHandler) AcceptRisk(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}
	var req struct {
		Justification string `json:"justification"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.uc.AcceptRisk(r.Context(), id, req.Justification); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to accept risk")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
```

### File 3: `services/finding-service/internal/delivery/http/sla_handler.go`

```go
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/usecase/sla"
)

// SLAHandler handles SLA-related HTTP requests.
type SLAHandler struct {
	uc  *sla.UseCases
	log zerolog.Logger
}

// NewSLAHandler creates a new SLAHandler.
func NewSLAHandler(uc *sla.UseCases, log zerolog.Logger) *SLAHandler {
	return &SLAHandler{uc: uc, log: log}
}

// Breached handles GET /sla/breached
func (h *SLAHandler) Breached(w http.ResponseWriter, r *http.Request) {
	findings, err := h.uc.GetBreached(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get breached SLAs")
		return
	}
	respondJSON(w, http.StatusOK, findings)
}

// DueSoon handles GET /sla/due-soon
func (h *SLAHandler) DueSoon(w http.ResponseWriter, r *http.Request) {
	findings, err := h.uc.GetDueSoon(r.Context(), 3) // 3 days warning
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get due-soon SLAs")
		return
	}
	respondJSON(w, http.StatusOK, findings)
}

// GetFindingSLA handles GET /findings/{id}/sla
func (h *SLAHandler) GetFindingSLA(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid finding ID")
		return
	}
	slaInfo, err := h.uc.GetSLAForFinding(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "SLA not found for finding")
		return
	}
	respondJSON(w, http.StatusOK, slaInfo)
}
```

### File 4: `services/finding-service/internal/delivery/http/router.go`

```go
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/usecase/finding"
	"github.com/osv/finding-service/internal/usecase/sla"
)

// RouterDeps holds all handler dependencies.
type RouterDeps struct {
	FindingUC *finding.UseCases
	SLAUC     *sla.UseCases
	Log       zerolog.Logger
}

// NewRouter creates the HTTP router for finding-service.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(chi.Middleware(CommonMiddleware(deps.Log)))

	findingH := NewFindingHandler(deps.FindingUC, deps.Log)
	slaH := NewSLAHandler(deps.SLAUC, deps.Log)

	// Finding CRUD
	r.Get("/findings", findingH.List)
	r.Post("/findings", findingH.Create)
	r.Get("/findings/{id}", findingH.Get)
	r.Put("/findings/{id}", findingH.Update)

	// Finding state transitions
	r.Post("/findings/{id}/verify", findingH.Verify)
	r.Post("/findings/{id}/mitigate", findingH.Mitigate)
	r.Post("/findings/{id}/accept-risk", findingH.AcceptRisk)

	// SLA endpoints
	r.Get("/sla/breached", slaH.Breached)
	r.Get("/sla/due-soon", slaH.DueSoon)
	r.Get("/findings/{id}/sla", slaH.GetFindingSLA)

	// Health
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}
```

## Files to Extend

### Extend: `services/finding-service/cmd/server/main.go`

```go
// Thêm HTTP server sau gRPC server (giữ gRPC nguyên):

// Import thêm:
// http_delivery "github.com/osv/finding-service/internal/delivery/http"
// "net/http"

// Trong main():
httpRouter := http_delivery.NewRouter(http_delivery.RouterDeps{
    FindingUC: findingUC,  // existing use case
    SLAUC:     slaUC,      // existing use case
    Log:       logger,
})

// Start HTTP server (non-blocking):
go func() {
    log.Info().Str("addr", ":8085").Msg("starting HTTP server")
    if err := http.ListenAndServe(":8085", httpRouter); err != nil {
        log.Fatal().Err(err).Msg("HTTP server failed")
    }
}()

// Existing gRPC server.Serve() continues as before
```

### Add to `go.mod` (nếu chi chưa có):

```bash
cd services/finding-service && go get github.com/go-chi/chi/v5
```

## Verification

```bash
cd services/finding-service && go build ./...

# Start server và test:
curl http://localhost:8085/health/live
# Expected: 200 OK

curl http://localhost:8085/findings
# Expected: JSON array (có thể rỗng)

curl http://localhost:8085/sla/breached
# Expected: JSON array

# gRPC vẫn hoạt động:
grpcurl -plaintext localhost:50055 list
```

## Notes

- Đọc actual method names trong `usecase/finding/use_cases.go` và điều chỉnh cho khớp
- Nếu use case methods khác signatures, adapt handler code accordingly
- Port 8085 — confirm trong config không bị conflict
- Thêm `"encoding/json"` và `"strconv"` imports khi cần
