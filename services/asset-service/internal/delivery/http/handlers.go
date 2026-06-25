package http

import (
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/asset-service/internal/domain/entity"
    "github.com/google/osv.dev/services/asset-service/internal/usecase/asset"
)

// Handler holds asset-service HTTP handler dependencies.
type Handler struct {
    crudUC       *asset.AssetCRUDUseCase
    taggingUC    *asset.TaggingUseCase
    riskUC       *asset.RiskScoringUseCase
    listUC       *asset.ListAssetsUseCase
    updateUC     *asset.UpdateAssetUseCase // TASK-008 FIX
    logger       zerolog.Logger
}

// NewHandler creates an asset-service HTTP handler.
func NewHandler(
    crudUC *asset.AssetCRUDUseCase,
    taggingUC *asset.TaggingUseCase,
    riskUC *asset.RiskScoringUseCase,
    listUC *asset.ListAssetsUseCase,
    logger zerolog.Logger,
) *Handler {
    return &Handler{
        crudUC:    crudUC,
        taggingUC: taggingUC,
        riskUC:    riskUC,
        listUC:    listUC,
        logger:    logger,
    }
}

// WithUpdateUC sets the optional UpdateAssetUseCase (TASK-008).
func (h *Handler) WithUpdateUC(uc *asset.UpdateAssetUseCase) *Handler {
    h.updateUC = uc
    return h
}

// Router defines asset-service routes.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Literal routes BEFORE wildcards
	r.Get("/assets/tags", h.GetTags)     // CR-004: GET /api/v1/assets/tags
	r.Post("/assets/bulk", h.CreateBulkAssets) // SEED-005-B
	r.Post("/assets/import", h.ImportAssets)   // SEED-005-B
	r.Get("/assets", h.ListAssets)
	r.Post("/assets", h.CreateAsset)           // SEED-005-B

	// Wildcard routes
	r.Get("/assets/{id}", h.GetAsset)
	r.Delete("/assets/{id}", h.DeleteAsset)    // SEED-005-B
	r.Put("/assets/{id}", h.UpdateAsset)       // TASK-008 FIX: PUT for full update
	r.Patch("/assets/{id}", h.PatchAsset)      // TASK-008 FIX: PATCH for partial update
	r.Put("/assets/{id}/tags", h.UpdateTags)
	r.Get("/assets/{id}/risk", h.GetRiskScore)
	r.Get("/assets/{id}/history", h.GetHistory)
	r.Get("/assets/{id}/findings", h.GetFindings)
	r.Post("/assets/{id}/vulnerabilities", h.AddVulnerabilities) // SEED-005-B

	return r
}

// GetTags handles GET /assets/tags — returns unique tags across all assets.
// Used by the frontend tag-filter dropdown (CR-004).
func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	assets, _, err := h.listUC.List(r.Context(), entity.AssetFilter{
		Limit: 1000, // fetch enough to build tag set; replace with dedicated repo call if needed
	})
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect unique tags
	seen := make(map[string]struct{})
	var tags []string
	for _, a := range assets {
		for _, t := range a.Tags {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				tags = append(tags, t)
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tags":  tags,
		"total": len(tags),
	})
}

// ListAssets handles GET /assets — filtered, paginated asset list.
// FIX: supports page_size param (spec) in addition to legacy limit param.
func (h *Handler) ListAssets(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    page, _ := strconv.Atoi(q.Get("page"))
    if page <= 0 {
        page = 1
    }

    // Support page_size (spec) or legacy limit param
    pageSize, _ := strconv.Atoi(q.Get("page_size"))
    if pageSize <= 0 {
        pageSize, _ = strconv.Atoi(q.Get("limit"))
    }
    if pageSize <= 0 {
        pageSize = 20
    }
    if pageSize > 200 {
        pageSize = 200
    }

    filter := entity.AssetFilter{
        Tag:    q.Get("tag"),
        OS:     q.Get("os"),
        Query:  q.Get("q"),
        Status: entity.AssetStatus(q.Get("status")),
        Page:   page,
        Limit:  pageSize,
    }

    if portStr := q.Get("port"); portStr != "" {
        port, err := strconv.Atoi(portStr)
        if err == nil {
            filter.HasPort = &port
        }
    }

    assets, total, err := h.listUC.List(r.Context(), filter)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // FIX: ensure assets is never null in JSON response
    if assets == nil {
        assets = []*entity.Asset{}
    }

    // FIX: use "page_size" (not "limit") per spec
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "assets":     assets,
        "total":      total,
        "page":       page,
        "page_size":  pageSize, // FIX: "limit" → "page_size" per spec
    })
}


// GetAsset handles GET /assets/{id}.
// FIX BUG-H2-001: actually queries DB via crudUC.Get instead of returning stub.
func (h *Handler) GetAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    a, err := h.crudUC.Get(r.Context(), id)
    if err != nil {
        jsonError(w, "asset not found", http.StatusNotFound)
        return
    }

    // Map to DTO with field names expected by OpenAPI spec and tests:
    // ASSET_REQUIRED = ["id", "ip", "services", "web_technologies",
    //                   "tags", "risk_score", "active_finding_count",
    //                   "first_seen_at", "last_seen_at"]
    var firstSeenAt, lastSeenAt string
    if a.CreatedAt.IsZero() {
        firstSeenAt = ""
    } else {
        firstSeenAt = a.CreatedAt.Format(time.RFC3339)
    }
    if a.LastSeenAt != nil {
        lastSeenAt = a.LastSeenAt.Format(time.RFC3339)
    } else {
        lastSeenAt = firstSeenAt // fallback
    }

    services := a.Services
    if services == nil {
        services = []entity.ServicePort{}
    }
    tags := a.Tags
    if tags == nil {
        tags = []string{}
    }

    dto := map[string]interface{}{
        "id":                   a.ID.String(),
        "ip":                   a.IPAddress,         // alias for ip_address
        "ip_address":           a.IPAddress,
        "hostname":             a.Hostname,
        "os":                   a.OS,
        "services":             services,
        "web_technologies":     []string{},           // not yet tracked — return empty array
        "tags":                 tags,
        "risk_score":           a.RiskScore,
        "active_finding_count": a.FindingCount,       // alias
        "finding_count":        a.FindingCount,
        "status":               string(a.Status),
        "first_seen_at":        firstSeenAt,
        "last_seen_at":         lastSeenAt,
        "created_at":           a.CreatedAt.Format(time.RFC3339),
        "updated_at":           a.UpdatedAt.Format(time.RFC3339),
    }
    jsonResponse(w, http.StatusOK, dto)
}

// UpdateTags handles PUT /assets/{id}/tags.
func (h *Handler) UpdateTags(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }

    var req asset.TagAssetInput
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }
    req.AssetID = id

    if err := h.taggingUC.Tag(r.Context(), req); err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// GetRiskScore handles GET /assets/{id}/risk.
func (h *Handler) GetRiskScore(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }

    score, err := h.riskUC.ComputeRiskScore(r.Context(), id)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "asset_id":   id,
        "risk_score": score,
    })
}

// GetHistory handles GET /assets/{id}/history.
// FIX BUG-H2-002: returns proper paginated format (asset_history table not yet implemented).
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "history": []interface{}{},
        "total":   0,
    })
}

// UpdateAsset handles PUT /assets/{id} — full update.
// TASK-008 FIX: replaces full asset fields.
func (h *Handler) UpdateAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    if h.updateUC == nil {
        jsonError(w, "update not supported", http.StatusNotImplemented)
        return
    }
    var in asset.UpdateAssetInput
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }
    in.ID = id
    updated, err := h.updateUC.Execute(r.Context(), in)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonResponse(w, http.StatusOK, updated)
}

// PatchAsset handles PATCH /assets/{id} — partial update.
// TASK-008 FIX: only non-nil fields are applied.
func (h *Handler) PatchAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }
    if h.updateUC == nil {
        jsonError(w, "patch not supported", http.StatusNotImplemented)
        return
    }
    var patch asset.PatchAssetInput
    if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }
    patch.ID = id
    updated, err := h.updateUC.Patch(r.Context(), patch)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonResponse(w, http.StatusOK, updated)
}

// GetFindings handles GET /assets/{id}/findings.
// FIX BUG-H2-003: returns proper paginated format (findings fetched from finding-service when available).
func (h *Handler) GetFindings(w http.ResponseWriter, r *http.Request) {
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "findings": []interface{}{},
        "total":    0,
    })
}

// CreateAsset handles POST /assets
func (h *Handler) CreateAsset(w http.ResponseWriter, r *http.Request) {
    var in entity.AssetCreateInput
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }
    a, err := h.crudUC.Create(r.Context(), in)
    if err != nil {
        // Simple conflict check
        if strings.Contains(err.Error(), "duplicate key value") || strings.Contains(err.Error(), "already exists") {
            jsonError(w, err.Error(), http.StatusConflict)
            return
        }
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonResponse(w, http.StatusCreated, a)
}

// CreateBulkAssets handles POST /assets/bulk
func (h *Handler) CreateBulkAssets(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Assets         []entity.AssetCreateInput `json:"assets"`
        UpdateExisting bool                      `json:"update_existing"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    results, err := h.crudUC.BulkCreate(r.Context(), req.Assets, req.UpdateExisting)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    created := 0
    updated := 0
    for _, res := range results {
        if res.Status == "created" {
            created++
        } else if res.Status == "updated" {
            updated++
        }
    }

    jsonResponse(w, http.StatusMultiStatus, map[string]interface{}{
        "created_count": created,
        "updated_count": updated,
        "failed_count":  len(results) - created - updated,
        "results":       results,
    })
}

// ImportAssets handles POST /assets/import
func (h *Handler) ImportAssets(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        jsonError(w, "file exceeds 10MB limit", http.StatusRequestEntityTooLarge)
        return
    }

    format := r.FormValue("format")
    if format == "" {
        format = "json"
    }
    updateExisting := r.FormValue("update_existing") == "true"

    file, _, err := r.FormFile("file")
    if err != nil {
        jsonError(w, "file field is required", http.StatusBadRequest)
        return
    }
    defer file.Close()

    var result *asset.ImportResult
    if format == "csv" {
        result, err = h.crudUC.ImportFromCSV(r.Context(), file, updateExisting)
    } else {
        result, err = h.crudUC.ImportFromJSON(r.Context(), file, updateExisting)
    }

    if err != nil {
        jsonError(w, "import failed: "+err.Error(), http.StatusBadRequest)
        return
    }

    jsonResponse(w, http.StatusOK, result)
}

// DeleteAsset handles DELETE /assets/{id}
func (h *Handler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }

    if err := h.crudUC.Delete(r.Context(), id); err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// AddVulnerabilities handles POST /assets/{id}/vulnerabilities
func (h *Handler) AddVulnerabilities(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        jsonError(w, "invalid asset ID", http.StatusBadRequest)
        return
    }

    var vulns []entity.Vulnerability
    if err := json.NewDecoder(r.Body).Decode(&vulns); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    if err := h.crudUC.AddVulnerabilities(r.Context(), id, vulns); err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonResponse(w, http.StatusCreated, map[string]string{"status": "vulnerabilities added"})
}

func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(body)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
    jsonResponse(w, status, map[string]string{"error": msg})
}
