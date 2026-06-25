package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/osv/search-service/internal/domain/repository"
)

type TaxonomyHandler struct {
	cweRepo   repository.CWERepository
	capecRepo repository.CAPECRepository
}

func NewTaxonomyHandler(cweRepo repository.CWERepository, capecRepo repository.CAPECRepository) *TaxonomyHandler {
	return &TaxonomyHandler{cweRepo: cweRepo, capecRepo: capecRepo}
}

// GET /api/v2/cwe?q=injection&page=0&limit=50
func (h *TaxonomyHandler) ListCWE(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page := parseInt(r.URL.Query().Get("page"), 0)
	limit := parseInt(r.URL.Query().Get("limit"), 50)

	entries, total, err := h.cweRepo.List(r.Context(), q, page, limit)
	if err != nil {
		respondError(w, 500, "failed to list CWE")
		return
	}
	
	for _, e := range entries {
		if e.Mitigations == nil {
			e.Mitigations = []string{}
		}
		if e.CAPECIDs == nil {
			e.CAPECIDs = []string{}
		}
	}

	respondJSON(w, 200, map[string]interface{}{
		"CweList":  entries,
		"Page":     page,
		"PageSize": limit,
		"total":    total,
	})
}

// GET /api/v2/cwe/{id} (e.g. CWE-89)
func (h *TaxonomyHandler) GetCWE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.cweRepo.FindByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, 404, "CWE not found")
		return
	}
	if err != nil {
		respondError(w, 500, "failed to get CWE")
		return
	}
	if entry.Mitigations == nil {
		entry.Mitigations = []string{}
	}
	if entry.CAPECIDs == nil {
		entry.CAPECIDs = []string{}
	}
	respondJSON(w, 200, entry)
}

// GET /api/v2/capec?q=injection&cwe_id=CWE-89&page=0&limit=50
func (h *TaxonomyHandler) ListCAPEC(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	cweID := r.URL.Query().Get("cwe_id")
	page := parseInt(r.URL.Query().Get("page"), 0)
	limit := parseInt(r.URL.Query().Get("limit"), 50)

	entries, total, err := h.capecRepo.List(r.Context(), q, cweID, page, limit)
	if err != nil {
		respondError(w, 500, "failed to list CAPEC")
		return
	}
	
	for _, e := range entries {
		if e.Mitigations == nil {
			e.Mitigations = []string{}
		}
		if e.CWEIDs == nil {
			e.CWEIDs = []string{}
		}
	}

	respondJSON(w, 200, map[string]interface{}{
		"CapecList": entries,
		"Page":      page,
		"PageSize":  limit,
		"total":     total,
	})
}

// GET /api/v2/capec/{id}
func (h *TaxonomyHandler) GetCAPEC(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.capecRepo.FindByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(w, 404, "CAPEC not found")
		return
	}
	if err != nil {
		respondError(w, 500, "failed to get CAPEC")
		return
	}
	if entry.Mitigations == nil {
		entry.Mitigations = []string{}
	}
	if entry.CWEIDs == nil {
		entry.CWEIDs = []string{}
	}
	respondJSON(w, 200, entry)
}
