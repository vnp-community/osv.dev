package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/osv/asset-service/internal/domain/entity"
	"github.com/osv/asset-service/internal/domain/repository"
	upsertasset "github.com/osv/asset-service/internal/usecase/upsert_asset"
	"github.com/rs/zerolog"
)

// AssetHandler handles HTTP requests for asset operations.
type AssetHandler struct {
	upsertUC  *upsertasset.UseCase
	assetRepo repository.AssetRepository
	vulnRepo  repository.VulnerabilityRepository
	log       zerolog.Logger
}

func NewAssetHandler(
	upsertUC *upsertasset.UseCase,
	assetRepo repository.AssetRepository,
	vulnRepo repository.VulnerabilityRepository,
	log zerolog.Logger,
) *AssetHandler {
	return &AssetHandler{upsertUC: upsertUC, assetRepo: assetRepo, vulnRepo: vulnRepo, log: log}
}

// ListAssets GET /assets
func (h *AssetHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	ps, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	filter := repository.AssetFilter{
		Search:   r.URL.Query().Get("search"),
		OS:       r.URL.Query().Get("os"),
		Page:     page,
		PageSize: ps,
	}
	assets, total, err := h.assetRepo.List(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets, "total": total})
}

// GetAsset GET /assets/{id}
func (h *AssetHandler) GetAsset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	asset, err := h.assetRepo.FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("not_found", "asset not found"))
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

// CreateAsset POST /assets (manual creation)
func (h *AssetHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	var req upsertasset.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	resp, err := h.upsertUC.Execute(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("create_failed", err.Error()))
		return
	}
	status := http.StatusOK
	if resp.Created { status = http.StatusCreated }
	writeJSON(w, status, resp)
}

// DeleteAsset DELETE /assets/{id}
func (h *AssetHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "id must be a UUID"))
		return
	}
	if err := h.assetRepo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("delete_failed", err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListVulnerabilities GET /assets/{id}/vulnerabilities
func (h *AssetHandler) ListVulnerabilities(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	ps, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	vulns, total, err := h.vulnRepo.FindByAssetID(r.Context(), id, page, ps)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("query_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vulnerabilities": vulns, "total": total})
}

// MarkRemediated POST /assets/{id}/vulnerabilities/{cve_id}/remediate
func (h *AssetHandler) MarkRemediated(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	cveID := chi.URLParam(r, "cve_id")
	if err := h.vulnRepo.MarkRemediated(r.Context(), id, cveID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("remediate_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "marked as remediated"})
}

// LinkVulnerability POST /assets/{id}/vulnerabilities (internal, called by cve-service event)
func (h *AssetHandler) LinkVulnerability(w http.ResponseWriter, r *http.Request) {
	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	var v entity.Vulnerability
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_request", err.Error()))
		return
	}
	v.AssetID = id
	if err := h.vulnRepo.Upsert(r.Context(), &v); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("link_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "vulnerability linked"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func errResp(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}

// NewRouter builds the asset service chi router.
func NewRouter(h *AssetHandler, log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	}))

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/assets", h.ListAssets)
		r.Post("/assets", h.CreateAsset)
		r.Get("/assets/{id}", h.GetAsset)
		r.Delete("/assets/{id}", h.DeleteAsset)
		r.Get("/assets/{id}/vulnerabilities", h.ListVulnerabilities)
		r.Post("/assets/{id}/vulnerabilities", h.LinkVulnerability)
		r.Post("/assets/{id}/vulnerabilities/{cve_id}/remediate", h.MarkRemediated)
	})
	return r
}
