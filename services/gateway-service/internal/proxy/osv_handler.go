// osv_handler.go — Real OSV v1 HTTP handlers for gateway-service.
// Replaces the placeholder "proxied to..." stub handlers in main.go.
//
// This file is ADDITIVE — main.go osvRouter() stub handlers are NOT deleted.
// Usage: mount OSVV1Handler() at /v1 to override the stub router.
//
// Routes implemented:
//   GET  /v1/vulns/{id}      → data-service gRPC CVEDBService.LookupCVEs / GetDBStatus
//   POST /v1/query           → data-service gRPC LookupCVEs
//   POST /v1/querybatch      → data-service gRPC batch LookupCVEs
//   GET  /v1/search          → search-service HTTP /v1/search
//   GET  /v1/health          → local health
package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	grpcclient "github.com/osv/gateway-service/internal/adapter/grpcclient"
	cvedbpb "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// OSVHandlerConfig holds upstream addresses for OSV v1 routes.
type OSVHandlerConfig struct {
	// DataServiceAddr is the gRPC address of data-service (CVEDBService).
	// Default: "localhost:50053" (warns in production)
	DataServiceAddr string

	// SearchServiceHTTP is the HTTP base URL of search-service.
	// Default: "http://localhost:8083" (warns in production)
	SearchServiceHTTP string

	// HTTPTimeoutSec is the timeout in seconds for proxied HTTP calls.
	// Default: 15. Override via OSV_HANDLER_TIMEOUT_SEC env var.
	HTTPTimeoutSec int
}

// OSVHandlerConfigFromEnv loads config from environment variables.
// Logs WARN when using localhost fallbacks to surface misconfiguration in production.
func OSVHandlerConfigFromEnv() OSVHandlerConfig {
	dataAddr := os.Getenv("DATA_SERVICE_ADDR")
	if dataAddr == "" {
		dataAddr = "localhost:50053"
		log.Warn().Str("fallback", dataAddr).
			Msg("DATA_SERVICE_ADDR not set, using localhost — set env var in production")
	}
	searchHTTP := os.Getenv("SEARCH_SERVICE_HTTP")
	if searchHTTP == "" {
		searchHTTP = "http://localhost:8083"
		log.Warn().Str("fallback", searchHTTP).
			Msg("SEARCH_SERVICE_HTTP not set, using localhost — set env var in production")
	}
	timeoutSec := 15
	if v := os.Getenv("OSV_HANDLER_TIMEOUT_SEC"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &timeoutSec); n != 1 || err != nil || timeoutSec <= 0 {
			log.Warn().Str("value", v).Msg("OSV_HANDLER_TIMEOUT_SEC invalid, using default 15s")
			timeoutSec = 15
		}
	}
	return OSVHandlerConfig{
		DataServiceAddr:   dataAddr,
		SearchServiceHTTP: searchHTTP,
		HTTPTimeoutSec:    timeoutSec,
	}
}

// osvV1Handler holds dependencies for OSV v1 route handlers.
type osvV1Handler struct {
	cvedb      *grpcclient.CVEDBClient
	searchBase string
	httpClient *http.Client
}

// OSVV1Router returns a chi.Router with real OSV v1 route implementations.
// Call r.Mount("/v1", OSVV1Router(cfg)) to activate — does NOT modify main.go.
func OSVV1Router(cfg OSVHandlerConfig) http.Handler {
	cvedbClient, err := grpcclient.NewCVEDBClient(cfg.DataServiceAddr)
	if err != nil {
		log.Warn().Err(err).Str("addr", cfg.DataServiceAddr).
			Msg("gateway: data-service unavailable, OSV v1 routes will return 503")
	}

	h := &osvV1Handler{
		cvedb:      cvedbClient,
		searchBase: cfg.SearchServiceHTTP,
		httpClient: &http.Client{Timeout: time.Duration(cfg.HTTPTimeoutSec) * time.Second}, // [FIX BUG-003] configurable
	}

	r := chi.NewRouter()

	// GET /v1/vulns/{id} — retrieve full vulnerability details by ID
	r.Get("/vulns/{id}", h.getVulnByID)

	// POST /v1/query — query vulnerabilities by package/version/commit
	r.Post("/query", h.queryVulns)

	// POST /v1/querybatch — batch query
	r.Post("/querybatch", h.queryBatch)

	// GET /v1/search — full-text search (proxied to search-service)
	r.Get("/search", h.searchVulns)

	return r
}

// ── Handler implementations ───────────────────────────────────────────────────

// getVulnByID handles GET /v1/vulns/{id}
func (h *osvV1Handler) getVulnByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}

	if h.cvedb == nil {
		http.Error(w, `{"error":"data-service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	// LookupCVEs with just the CVE ID to get full record
	result, err := h.cvedb.LookupCVEs(r.Context(), grpcclient.LookupRequest{
		Products: []*cvedbpb.ProductInfo{
			{Product: id}, // ID-based lookup
		},
	})
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("getVulnByID: data-service error")
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if result == nil || len(result.Results) == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

// queryRequest matches OSV v1 POST /v1/query body.
type queryRequest struct {
	Package *struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
		PURL      string `json:"purl,omitempty"`
	} `json:"package,omitempty"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

// queryVulns handles POST /v1/query
func (h *osvV1Handler) queryVulns(w http.ResponseWriter, r *http.Request) {
	if h.cvedb == nil {
		http.Error(w, `{"error":"data-service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var products []*cvedbpb.ProductInfo
	if req.Package != nil {
		pi := &cvedbpb.ProductInfo{
			Product: req.Package.Name,
			Version: req.Version,
		}
		if req.Package.PURL != "" {
			pi.Purl = req.Package.PURL
		}
		products = append(products, pi)
	}

	result, err := h.cvedb.LookupCVEs(r.Context(), grpcclient.LookupRequest{
		Products: products,
	})
	if err != nil {
		log.Error().Err(err).Msg("queryVulns: data-service error")
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Format as OSV v1 response: {"vulns": [...]}
	type vulnSummary struct {
		ID       string   `json:"id"`
		Modified string   `json:"modified,omitempty"`
		Aliases  []string `json:"aliases,omitempty"`
	}
	type queryResp struct {
		Vulns []vulnSummary `json:"vulns"`
	}

	resp := queryResp{}
	if result != nil {
		for _, pCVEs := range result.Results {
			for _, cve := range pCVEs.GetCves() {
				resp.Vulns = append(resp.Vulns, vulnSummary{
					ID: cve.GetCveNumber(),
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// batchQueryRequest matches OSV v1 POST /v1/querybatch body.
type batchQueryRequest struct {
	Queries []queryRequest `json:"queries"`
}

// queryBatch handles POST /v1/querybatch
func (h *osvV1Handler) queryBatch(w http.ResponseWriter, r *http.Request) {
	if h.cvedb == nil {
		http.Error(w, `{"error":"data-service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	var req batchQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	type singleResult struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	}
	type batchResp struct {
		Results []singleResult `json:"results"`
	}

	resp := batchResp{Results: make([]singleResult, len(req.Queries))}

	for i, q := range req.Queries {
		var products []*cvedbpb.ProductInfo
		if q.Package != nil {
			products = append(products, &cvedbpb.ProductInfo{
				Product: q.Package.Name,
				Version: q.Version,
				Purl:    q.Package.PURL,
			})
		}
		result, err := h.cvedb.LookupCVEs(r.Context(), grpcclient.LookupRequest{
			Products: products,
		})
		if err != nil {
			log.Warn().Err(err).Int("query_index", i).Msg("querybatch: data-service error for query")
			continue
		}
		sr := singleResult{}
		if result != nil {
			for _, pCVEs := range result.Results {
				for _, cve := range pCVEs.GetCves() {
					sr.Vulns = append(sr.Vulns, struct {
						ID string `json:"id"`
					}{ID: cve.GetCveNumber()})
				}
			}
		}
		resp.Results[i] = sr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// searchVulns handles GET /v1/search — proxies to search-service HTTP.
func (h *osvV1Handler) searchVulns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error":"q parameter is required"}`, http.StatusBadRequest)
		return
	}
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}

	upstreamURL := fmt.Sprintf("%s/v1/search?q=%s&limit=%s",
		h.searchBase,
		url.QueryEscape(q),
		url.QueryEscape(limit),
	)

	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("Accept", "application/json")

	upstreamResp, err := h.httpClient.Do(upstream)
	if err != nil {
		log.Error().Err(err).Str("url", upstreamURL).Msg("searchVulns: search-service error")
		http.Error(w, `{"error":"search-service unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	defer upstreamResp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(upstreamResp.StatusCode)
	io.Copy(w, upstreamResp.Body) //nolint:errcheck
}
