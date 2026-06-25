# SA-SHARED-03 — OSV REST Client (v1 API)

## Metadata
- **Task ID**: SA-SHARED-03
- **Sprint**: A (P0 — Foundation)
- **Ước tính**: 1.5 giờ
- **Dependencies**: SA-SHARED-01
- **Spec nguồn**: `specs/solutions/enhance-cli-app/04_service-client-layer.md` § "6. OSV Query Client (REST)"

---

## Context

```bash
# Xem OSV v1 handler đã có trong search-service
cat services/search-service/internal/delivery/http/osv_handler.go | head -60

# Xem proto để biết query format
cat services/shared/proto/gen/go/cvedb/v1/cvedb.pb.go | grep -A5 "LookupCVEsRequest"

# Xem apps/osv existing API client nếu có
ls apps/osv/internal/api/
```

---

## Goal

Tạo OSV v1 REST API client để CLI `cmd/query` và apps/osv có thể gọi gateway-service theo OSV schema chuẩn (`POST /v1/query`, `GET /v1/vulns/{id}`, `POST /v1/querybatch`).

---

## Files to Create

### File 1: `services/shared/pkg/clients/rest/osv_client.go`

```go
// Package rest provides HTTP clients for OSV platform REST APIs.
// OSVClient implements the OSV v1 public API spec, targeting either:
//   - Local gateway-service: http://localhost:8080
//   - Public API: https://api.osv.dev
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OSVClient is an HTTP client for the OSV v1 REST API.
// All query methods return OSV-schema-compatible responses.
type OSVClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOSVClient creates a client pointing to the given base URL.
// baseURL example: "http://localhost:8080" or "https://api.osv.dev"
func NewOSVClient(baseURL string) *OSVClient {
	return &OSVClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── Request/Response types (OSV v1 schema) ───────────────────────────────────

// PackageInfo identifies a package in a specific ecosystem.
type PackageInfo struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	// PURL is the Package URL (alternative to ecosystem+name).
	PURL string `json:"purl,omitempty"`
}

// QueryRequest represents a POST /v1/query payload.
// Exactly one of Package+Version, Commit, or PURL must be set.
type QueryRequest struct {
	Package *PackageInfo `json:"package,omitempty"`
	Version string       `json:"version,omitempty"`
	// Commit is the full git commit hash for commit-based queries.
	Commit string `json:"commit,omitempty"`
	// PURL is a Package URL for direct PURL-based queries.
	PURL string `json:"purl,omitempty"`
}

// QueryResponse wraps the list of matching vulnerabilities.
type QueryResponse struct {
	Vulns []VulnSummary `json:"vulns"`
}

// VulnSummary is a brief vulnerability record (as returned by /v1/query).
type VulnSummary struct {
	ID       string `json:"id"`
	Modified string `json:"modified"`
	// Aliases contains related CVE/GHSA IDs.
	Aliases []string `json:"aliases,omitempty"`
}

// BatchQueryRequest is the payload for POST /v1/querybatch.
type BatchQueryRequest struct {
	Queries []QueryRequest `json:"queries"`
}

// BatchQueryResponse is the response from POST /v1/querybatch.
type BatchQueryResponse struct {
	Results []QueryResponse `json:"results"`
}

// ── API Methods ──────────────────────────────────────────────────────────────

// QueryByPackage queries for vulnerabilities affecting a package+version.
// Satisfies URD UR-01: search by package and version.
//
// Example:
//
//	resp, err := client.QueryByPackage(ctx, "npm", "lodash", "4.17.20")
func (c *OSVClient) QueryByPackage(ctx context.Context, ecosystem, name, version string) (*QueryResponse, error) {
	req := QueryRequest{
		Package: &PackageInfo{
			Ecosystem: ecosystem,
			Name:      name,
		},
		Version: version,
	}
	return c.query(ctx, req)
}

// QueryByCommit queries for vulnerabilities affecting a git commit.
// Satisfies URD UR-04: commit-based lookup.
func (c *OSVClient) QueryByCommit(ctx context.Context, commit string) (*QueryResponse, error) {
	return c.query(ctx, QueryRequest{Commit: commit})
}

// QueryByCVEID looks up a specific vulnerability by ID.
// Calls GET /v1/vulns/{id}.
// Satisfies URD UR-02: get full vulnerability details.
func (c *OSVClient) QueryByCVEID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/vulns/%s", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /v1/vulns/%s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("vulnerability %s not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET /v1/vulns/%s returned %d: %s", id, resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode vuln response: %w", err)
	}
	return result, nil
}

// QueryBatch submits multiple queries in a single request.
// Satisfies URD UR-06: bulk/batch lookup.
func (c *OSVClient) QueryBatch(ctx context.Context, queries []QueryRequest) (*BatchQueryResponse, error) {
	body, err := json.Marshal(BatchQueryRequest{Queries: queries})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /v1/querybatch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("querybatch returned %d: %s", resp.StatusCode, b)
	}

	var result BatchQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode querybatch response: %w", err)
	}
	return &result, nil
}

// ── internal helpers ─────────────────────────────────────────────────────────

func (c *OSVClient) query(ctx context.Context, q QueryRequest) (*QueryResponse, error) {
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /v1/query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query returned %d: %s", resp.StatusCode, b)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}
	return &result, nil
}
```

---

## Acceptance Criteria

- [ ] `services/shared/pkg/clients/rest/osv_client.go` tồn tại
- [ ] `QueryByPackage(ctx, ecosystem, name, version)` → calls `POST /v1/query`
- [ ] `QueryByCVEID(ctx, id)` → calls `GET /v1/vulns/{id}`
- [ ] `QueryBatch(ctx, queries)` → calls `POST /v1/querybatch`
- [ ] `go build ./pkg/clients/rest/...` PASS

---

## Verification

```bash
cd services/shared
go build ./pkg/clients/rest/...
go vet ./pkg/clients/rest/...
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created
- `services/shared/pkg/clients/rest/osv_client.go` — `OSVClient` với `QueryByPackage()`, `QueryByCommit()`, `QueryByCVEID()`, `QueryBatch()`

### Build Verification
```
go build ./clients/rest/...  → OK
```
