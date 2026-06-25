package http

import (
    "encoding/json"
    "net/http"

    "github.com/osv/search-service/internal/domain/repository"
    "github.com/osv/search-service/internal/infra/opensearch"
)

// InternalHandler handles internal API calls (not exposed via gateway).
type InternalHandler struct {
    osClient *opensearch.Client
    cveRepo  repository.CVERepository
}

// NewInternalHandler creates a new InternalHandler.
func NewInternalHandler(osClient *opensearch.Client, cveRepo repository.CVERepository) *InternalHandler {
    return &InternalHandler{osClient: osClient, cveRepo: cveRepo}
}

// POST /internal/opensearch/index
// Body: {"cve_id": "CVE-2021-44228"} — index single CVE
func (h *InternalHandler) IndexCVE(w http.ResponseWriter, r *http.Request) {
    if h.osClient == nil {
        respondError(w, 503, "opensearch not configured")
        return
    }

    var req struct {
        CVEID string `json:"cve_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid json")
        return
    }

    cve, err := h.cveRepo.FindByID(r.Context(), req.CVEID)
    if err != nil {
        respondError(w, 404, "CVE not found")
        return
    }

    if err := h.osClient.IndexCVE(r.Context(), cve); err != nil {
        respondError(w, 500, "indexing failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]string{"status": "indexed"})
}

// POST /internal/opensearch/bulk
// Body: {"cve_ids": [...]} — bulk re-index
func (h *InternalHandler) BulkIndex(w http.ResponseWriter, r *http.Request) {
    if h.osClient == nil {
        respondError(w, 503, "opensearch not configured")
        return
    }

    var req struct {
        CVEIDs []string `json:"cve_ids"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, 400, "invalid json")
        return
    }

    if len(req.CVEIDs) > 1000 {
        req.CVEIDs = req.CVEIDs[:1000] // cap at 1000
    }

    cves, err := h.cveRepo.FindByIDs(r.Context(), req.CVEIDs)
    if err != nil {
        respondError(w, 500, "fetch CVEs failed")
        return
    }

    if err := h.osClient.BulkIndex(r.Context(), cves); err != nil {
        respondError(w, 500, "bulk index failed: "+err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{
        "indexed": len(cves),
        "status":  "ok",
    })
}
