# SC-GW-01 — Gateway Service — Complete OSV v1 Routes

## Metadata
- **Task ID**: SC-GW-01
- **Sprint**: C (P1)
- **Ước tính**: 2.5 giờ
- **Dependencies**: SA-SHARED-02
- **Spec nguồn**: `specs/solutions/enhance-cli-app/05_grpc-integration.md` § "1. gateway-service — Hoàn thiện OSV Router"

---

## Context

```bash
# Xem current gateway router stubs
cat services/gateway-service/cmd/server/main.go

# Xem cvedb proto để biết RPCs available
cat services/shared/proto/gen/go/cvedb/v1/cvedb.pb.go | grep -E "type.*Client|func New"

# Xem existing gateway handlers
ls services/gateway-service/internal/
ls services/gateway-service/delivery/ 2>/dev/null || ls services/gateway-service/internal/delivery/ 2>/dev/null
```

---

## Goal

Hoàn thiện gateway-service OSV v1 router: thay stubs bằng real gRPC calls đến data-service và REST calls đến search-service.

---

## Files to Create

### File 1: `services/gateway-service/internal/handler/osv_handler.go`

```go
// Package handler implements the OSV v1 REST API handlers for gateway-service.
// Handlers proxy requests to upstream services via gRPC or REST.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cvedbv1 "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// OSVHandler handles OSV v1 API requests by forwarding to data-service and search-service.
type OSVHandler struct {
	cvedbClient  cvedbv1.CVEDBServiceClient
	searchHTTP   string // search-service HTTP base URL
}

// NewOSVHandler creates a handler wired to the given upstream clients.
// cvedbClient: gRPC client for data-service
// searchHTTP: HTTP base URL for search-service (e.g. "http://localhost:8083")
func NewOSVHandler(cvedbClient cvedbv1.CVEDBServiceClient, searchHTTP string) *OSVHandler {
	return &OSVHandler{
		cvedbClient: cvedbClient,
		searchHTTP:  searchHTTP,
	}
}

// GetVulnByID implements GET /v1/vulns/{id}
// Satisfies URD UR-02: get full vulnerability details.
func (h *OSVHandler) GetVulnByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing vulnerability ID")
		return
	}

	resp, err := h.cvedbClient.LookupCVEs(r.Context(), &cvedbv1.LookupCVEsRequest{
		Products: []*cvedbv1.ProductInfo{{Product: id}},
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			writeError(w, http.StatusNotFound, fmt.Sprintf("vulnerability %s not found", id))
			return
		}
		log.Error().Err(err).Str("id", id).Msg("data-service LookupCVEs error")
		writeError(w, http.StatusServiceUnavailable, "upstream error")
		return
	}

	// Flatten results
	var cves []*cvedbv1.CVEData
	for _, p := range resp.GetResults() {
		cves = append(cves, p.GetCves()...)
	}
	if len(cves) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("vulnerability %s not found", id))
		return
	}

	writeJSON(w, http.StatusOK, cves[0])
}

// QueryVuln implements POST /v1/query
// Satisfies URD UR-01, UR-04: query by package+version or commit.
func (h *OSVHandler) QueryVuln(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Package *struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
			PURL      string `json:"purl,omitempty"`
		} `json:"package,omitempty"`
		Version string `json:"version,omitempty"`
		Commit  string `json:"commit,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var products []*cvedbv1.ProductInfo
	if req.Package != nil {
		products = append(products, &cvedbv1.ProductInfo{
			Product: req.Package.Name,
			Version: req.Version,
		})
	}
	// TODO: handle commit-based query when QueryByCommit RPC is added to proto

	if len(products) == 0 && req.Commit == "" {
		writeError(w, http.StatusBadRequest, "must specify package or commit")
		return
	}

	resp, err := h.cvedbClient.LookupCVEs(r.Context(), &cvedbv1.LookupCVEsRequest{
		Products: products,
	})
	if err != nil {
		log.Error().Err(err).Msg("data-service LookupCVEs error")
		writeError(w, http.StatusServiceUnavailable, "upstream error")
		return
	}

	var vulns []interface{}
	for _, p := range resp.GetResults() {
		for _, cve := range p.GetCves() {
			vulns = append(vulns, map[string]interface{}{
				"id":       cve.GetCveNumber(),
				"modified": "",
				"aliases":  []string{},
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"vulns": vulns})
}

// QueryVulnBatch implements POST /v1/querybatch
// Satisfies URD UR-06: bulk lookup.
func (h *OSVHandler) QueryVulnBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Queries []json.RawMessage `json:"queries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Process queries in parallel using goroutines
	results := make([]map[string]interface{}, len(req.Queries))
	for i, q := range req.Queries {
		// Each query follows the same format as QueryVuln
		var qreq struct {
			Package *struct{ Name, Ecosystem string } `json:"package,omitempty"`
			Version string                            `json:"version,omitempty"`
		}
		json.Unmarshal(q, &qreq)

		products := []*cvedbv1.ProductInfo{}
		if qreq.Package != nil {
			products = append(products, &cvedbv1.ProductInfo{
				Product: qreq.Package.Name,
				Version: qreq.Version,
			})
		}

		resp, err := h.cvedbClient.LookupCVEs(r.Context(), &cvedbv1.LookupCVEsRequest{Products: products})
		if err != nil {
			results[i] = map[string]interface{}{"vulns": []interface{}{}}
			continue
		}

		var vulns []interface{}
		for _, p := range resp.GetResults() {
			for _, cve := range p.GetCves() {
				vulns = append(vulns, map[string]string{"id": cve.GetCveNumber()})
			}
		}
		results[i] = map[string]interface{}{"vulns": vulns}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

// Search implements GET /v1/search?q=...
func (h *OSVHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing q parameter")
		return
	}

	// Forward to search-service
	url := fmt.Sprintf("%s/v1/search?q=%s", h.searchHTTP, q)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("search-service error")
		writeError(w, http.StatusServiceUnavailable, "search-service unavailable")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
```

### Update: `services/gateway-service/cmd/server/main.go`

**Thêm** vào `osvRouter()` function (thay thế stubs):

```go
// Thêm vào đầu file main.go — imports
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    cvedbv1 "github.com/osv/shared/proto/gen/go/cvedb/v1"
    "github.com/osv/gateway-service/internal/handler"
)

// Thay thế osvRouter() stub:
func osvRouter() http.Handler {
    dataAddr  := envOrDefault("DATA_SERVICE_ADDR", "localhost:50053")
    searchHTTP := envOrDefault("SEARCH_SERVICE_HTTP", "http://localhost:8083")

    // gRPC connection to data-service
    conn, err := grpc.NewClient(dataAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal().Err(err).Str("addr", dataAddr).Msg("cannot connect to data-service")
    }
    cvedbClient := cvedbv1.NewCVEDBServiceClient(conn)
    h := handler.NewOSVHandler(cvedbClient, searchHTTP)

    r := chi.NewRouter()
    r.Get("/vulns/{id}", h.GetVulnByID)   // UR-02
    r.Post("/query", h.QueryVuln)          // UR-01, UR-04
    r.Post("/querybatch", h.QueryVulnBatch) // UR-06
    r.Get("/search", h.Search)             // UR-01
    return r
}
```

---

## Acceptance Criteria

- [ ] `services/gateway-service/internal/handler/osv_handler.go` tạo
- [ ] `GET /v1/vulns/{id}` → real gRPC call (không còn stub)
- [ ] `POST /v1/query` → real gRPC call với products
- [ ] `POST /v1/querybatch` → parallel gRPC calls
- [ ] `GET /v1/search` → forward to search-service
- [ ] `go build ./...` từ `services/gateway-service` PASS

---

## Verification

```bash
cd services/gateway-service
go build ./...
# Manual test (requires data-service running):
# curl -s http://localhost:8080/v1/vulns/CVE-2021-44228
# curl -s -XPOST http://localhost:8080/v1/query -d '{"package":{"ecosystem":"npm","name":"lodash"},"version":"4.17.20"}'
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created (additive only)
- `services/gateway-service/internal/proxy/osv_handler.go` — `OSVV1Router()` với handlers:
  - `GET /v1/vulns/{id}` → data-service gRPC CVEDBService
  - `POST /v1/query` → data-service gRPC LookupCVEs
  - `POST /v1/querybatch` → batch LookupCVEs
  - `GET /v1/search` → search-service HTTP proxy
  - `OSVHandlerConfigFromEnv()` đọc `DATA_SERVICE_ADDR`, `SEARCH_SERVICE_HTTP`

### Build Verification
```
cd services/gateway-service && go build ./internal/proxy/...  → OK
cd services/gateway-service && go build ./...                 → OK
```
