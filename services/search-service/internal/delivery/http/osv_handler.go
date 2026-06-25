// Package http — osv_handler.go
// OSVHandler serves OSV v1 compatible REST endpoints.
//
// Routes registered in main.go (additive to existing mux):
//   POST /v1/query           → Handler.Query()
//   POST /v1/querybatch      → Handler.BatchQuery()
//   GET  /v1/vulns/{id}      → Handler.GetVulnByID()
//   GET  /v1/vulns/list      → Handler.ListVulns()
//
// Reference: https://osv.dev/docs/#section/OSV-API
package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	domainerrors "github.com/osv/search-service/internal/domain/errors"
	"github.com/osv/search-service/internal/usecase/getbyid"
	"github.com/osv/search-service/internal/usecase/osv_query"
)

// OSVHandler serves OSV v1 compatible endpoints.
type OSVHandler struct {
	queryUC    *osv_query.OSVQueryUseCase
	getByIDUC  *getbyid.UseCase
	log        zerolog.Logger
}

// NewOSVHandler creates an OSVHandler.
func NewOSVHandler(queryUC *osv_query.OSVQueryUseCase, getByIDUC *getbyid.UseCase, log zerolog.Logger) *OSVHandler {
	return &OSVHandler{queryUC: queryUC, getByIDUC: getByIDUC, log: log}
}

// Query handles POST /v1/query.
func (h *OSVHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req osv_query.OSVQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.queryUC.QueryByPackage(r.Context(), req)
	if err != nil {
		h.log.Error().Err(err).Msg("osv.Query")
		respondError(w, http.StatusInternalServerError, "query failed")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// BatchQuery handles POST /v1/querybatch.
func (h *OSVHandler) BatchQuery(w http.ResponseWriter, r *http.Request) {
	var req osv_query.OSVBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.queryUC.BatchQuery(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetVulnByID handles GET /v1/vulns/{id}.
// Delegates to existing getbyid.UseCase.
func (h *OSVHandler) GetVulnByID(w http.ResponseWriter, r *http.Request) {
	// Extract {id} from path — /v1/vulns/{id}
	id := strings.TrimPrefix(r.URL.Path, "/v1/vulns/")
	id = strings.TrimSuffix(id, "/")

	if id == "" || id == "list" {
		respondError(w, http.StatusBadRequest, "missing vuln id")
		return
	}

	cve, err := h.getByIDUC.Execute(r.Context(), id)
	if err != nil {
		if err == domainerrors.ErrInvalidCVEID {
			respondError(w, http.StatusBadRequest, "invalid CVE ID format")
		} else {
			respondError(w, http.StatusNotFound, "vulnerability not found")
		}
		return
	}

	respondJSON(w, http.StatusOK, cve)
}

// ListVulns handles GET /v1/vulns/list.
func (h *OSVHandler) ListVulns(w http.ResponseWriter, r *http.Request) {
	params := osv_query.OSVVulnListParams{
		PageToken:     r.URL.Query().Get("page_token"),
		ModifiedSince: r.URL.Query().Get("modified_since"),
		Ecosystem:     r.URL.Query().Get("ecosystem"),
	}

	result, err := h.queryUC.ListVulns(r.Context(), params)
	if err != nil {
		h.log.Error().Err(err).Msg("osv.ListVulns")
		respondError(w, http.StatusInternalServerError, "list failed")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// RegisterRoutes registers all OSV v1 routes on the given mux.
// Call this from main.go after constructing the handler.
func (h *OSVHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/query", h.Query)
	mux.HandleFunc("/v1/querybatch", h.BatchQuery)
	mux.HandleFunc("/v1/vulns/list", h.ListVulns)       // MUST be before /v1/vulns/
	mux.HandleFunc("/v1/vulns/", h.GetVulnByID)          // catch-all for /v1/vulns/{id}
}


