# SA-SHARED-02 — Shared gRPC Client Layer

## Metadata
- **Task ID**: SA-SHARED-02
- **Sprint**: A (P0 — Foundation)
- **Ước tính**: 2 giờ
- **Dependencies**: SA-SHARED-01 (needs shared/pkg/events)
- **Spec nguồn**: `specs/solutions/enhance-cli-app/04_service-client-layer.md`

---

## Context

```bash
# Xem existing proto gen files
ls services/shared/proto/gen/go/
ls services/shared/proto/gen/go/ai/v1/
ls services/shared/proto/gen/go/cvedb/v1/

# Check go.mod của shared
cat services/shared/go.mod 2>/dev/null || echo "no go.mod in shared root"

# Xem gateway service's existing grpc client patterns (để tham khảo style)
cat services/gateway-service/adapter/grpcclient/ai_client.go | head -50
cat services/gateway-service/adapter/grpcclient/identity_client.go | head -50
```

---

## Goal

Tạo shared gRPC client layer trong `services/shared/pkg/clients/grpc/` để CLI và apps/osv dùng chung, thay vì mỗi app tự tạo client riêng.

---

## Files to Create

### File 1: `services/shared/pkg/clients/grpc/options.go`

```go
// Package grpc provides shared gRPC client utilities for OSV platform services.
// All internal service-to-service communication should use these helpers
// for consistent retry, timeout, and connection behavior.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const defaultDialTimeout = 5 * time.Second

// DefaultDialOptions returns standard gRPC options for internal (in-cluster) communication.
// Uses insecure credentials — TLS/mTLS is handled at the network layer (service mesh, VPN).
func DefaultDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}
}

// DialTimeout creates a gRPC client connection with the default 5-second dial timeout.
// Returns error if the connection cannot be established within the timeout.
func DialTimeout(addr string, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()
	opts := append(DefaultDialOptions(), extraOpts...)
	return grpc.DialContext(ctx, addr, opts...) //nolint:staticcheck
}
```

### File 2: `services/shared/pkg/clients/grpc/data_client.go`

```go
package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	cvedbv1 "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// DataClient wraps the cvedb gRPC service (data-service) for vulnerability operations.
// Used by: apps/cli (relations, worker, query commands), apps/osv (gateway proxy)
type DataClient struct {
	conn   *grpc.ClientConn
	cvedb  cvedbv1.CVEDBServiceClient
}

// NewDataClient connects to data-service at the given address.
// Example: NewDataClient("localhost:50053") or NewDataClient("data-service:50053")
func NewDataClient(addr string) (*DataClient, error) {
	conn, err := DialTimeout(addr)
	if err != nil {
		return nil, fmt.Errorf("data client connect %s: %w", addr, err)
	}
	return &DataClient{
		conn:  conn,
		cvedb: cvedbv1.NewCVEDBServiceClient(conn),
	}, nil
}

// LookupByPackage queries CVEs affecting a given package + version.
// Maps to OSV v1 POST /query with package+version.
func (c *DataClient) LookupByPackage(ctx context.Context, ecosystem, pkg, version string) ([]*cvedbv1.CVEData, error) {
	resp, err := c.cvedb.LookupCVEs(ctx, &cvedbv1.LookupCVEsRequest{
		Products: []*cvedbv1.ProductInfo{{
			Product: pkg,
			Version: version,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("LookupCVEs: %w", err)
	}
	var results []*cvedbv1.CVEData
	for _, p := range resp.GetResults() {
		results = append(results, p.GetCves()...)
	}
	return results, nil
}

// LookupByCVEID returns vulnerabilities matching the given CVE/OSV ID.
func (c *DataClient) LookupByCVEID(ctx context.Context, id string) ([]*cvedbv1.CVEData, error) {
	resp, err := c.cvedb.LookupCVEs(ctx, &cvedbv1.LookupCVEsRequest{
		Products: []*cvedbv1.ProductInfo{{
			// Use product field with CVE ID for direct lookup
			// TODO: add dedicated GetCVE RPC to proto
			Product: id,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("LookupCVEs by ID: %w", err)
	}
	var results []*cvedbv1.CVEData
	for _, p := range resp.GetResults() {
		results = append(results, p.GetCves()...)
	}
	return results, nil
}

// Raw returns the underlying gRPC client for advanced usage.
func (c *DataClient) Raw() cvedbv1.CVEDBServiceClient { return c.cvedb }

// Close releases the gRPC connection.
func (c *DataClient) Close() error { return c.conn.Close() }
```

### File 3: `services/shared/pkg/clients/grpc/ai_client.go`

```go
package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
)

// AIClient wraps ai-service gRPC for enrichment and EPSS operations.
// Used by: apps/cli (worker ai_enricher, cmd/enrich), apps/osv (gateway BFF)
type AIClient struct {
	conn   *grpc.ClientConn
	ai     aiv1.AIEnrichmentServiceClient
}

// NewAIClient connects to ai-service at the given address.
// Example: NewAIClient("localhost:50052")
func NewAIClient(addr string) (*AIClient, error) {
	conn, err := DialTimeout(addr)
	if err != nil {
		return nil, fmt.Errorf("ai client connect %s: %w", addr, err)
	}
	return &AIClient{
		conn: conn,
		ai:   aiv1.NewAIEnrichmentServiceClient(conn),
	}, nil
}

// EnrichCVE triggers AI enrichment for a single CVE.
// Used by: apps/cli/cmd/enrich, worker pipeline AIEnricher
func (c *AIClient) EnrichCVE(ctx context.Context, cveID string) (*aiv1.EnrichCVEResponse, error) {
	resp, err := c.ai.EnrichCVE(ctx, &aiv1.EnrichCVERequest{CveId: cveID})
	if err != nil {
		return nil, fmt.Errorf("EnrichCVE %s: %w", cveID, err)
	}
	return resp, nil
}

// GetEPSS returns the EPSS probability score for a CVE.
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aiv1.EPSSResponse, error) {
	resp, err := c.ai.GetEPSS(ctx, &aiv1.GetEPSSRequest{CveId: cveID})
	if err != nil {
		return nil, fmt.Errorf("GetEPSS %s: %w", cveID, err)
	}
	return resp, nil
}

// BatchEnrich triggers enrichment for multiple CVEs.
// Used by: apps/cli/cmd/enrich --batch
func (c *AIClient) BatchEnrich(ctx context.Context, cveIDs []string) (*aiv1.BatchEnrichResponse, error) {
	resp, err := c.ai.BatchEnrich(ctx, &aiv1.BatchEnrichRequest{CveIds: cveIDs})
	if err != nil {
		return nil, fmt.Errorf("BatchEnrich (%d CVEs): %w", len(cveIDs), err)
	}
	return resp, nil
}

// Raw returns the underlying gRPC client.
func (c *AIClient) Raw() aiv1.AIEnrichmentServiceClient { return c.ai }

// Close releases the connection.
func (c *AIClient) Close() error { return c.conn.Close() }
```

### File 4: `services/shared/pkg/clients/grpc/search_client.go`

```go
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SearchClient wraps search-service for full-text vulnerability search.
// Uses REST HTTP since the search-service proto is not yet finalized.
// Used by: apps/cli/cmd/query, apps/osv (gateway /v1/search)
type SearchClient struct {
	httpClient *http.Client
	baseURL    string // e.g. "http://localhost:8083"
}

// SearchResult represents a single vulnerability search result.
type SearchResult struct {
	CVEID    string  `json:"id"`
	Summary  string  `json:"summary"`
	Severity string  `json:"severity"`
	Score    float64 `json:"cvss_score"`
}

// NewSearchClient creates a client pointing to search-service HTTP API.
// addr format: "localhost:8083" or "search-service:8083"
func NewSearchClient(addr string) *SearchClient {
	return &SearchClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "http://" + addr,
	}
}

// Search performs a full-text vulnerability search.
// Calls: GET /v1/search?q=<query>&limit=<limit>
func (c *SearchClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	u := fmt.Sprintf("%s/v1/search?q=%s&limit=%d",
		c.baseURL, url.QueryEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned %d", resp.StatusCode)
	}

	var result struct {
		Items []SearchResult `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return result.Items, nil
}
```

---

## Acceptance Criteria

- [ ] `services/shared/pkg/clients/grpc/options.go` — DialTimeout + DefaultDialOptions
- [ ] `services/shared/pkg/clients/grpc/data_client.go` — DataClient với LookupByPackage, LookupByCVEID, Close
- [ ] `services/shared/pkg/clients/grpc/ai_client.go` — AIClient với EnrichCVE, GetEPSS, BatchEnrich, Close
- [ ] `services/shared/pkg/clients/grpc/search_client.go` — SearchClient (REST) với Search
- [ ] `go build ./...` từ `services/shared/` PASS
- [ ] Không xóa code cũ

---

## Verification

```bash
cd services/shared
go build ./pkg/clients/...
# Output: no errors
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created
- `services/shared/pkg/clients/grpc/options.go` — `DefaultDialOptions()` + `DialTimeout()`
- `services/shared/pkg/clients/grpc/data_client.go` — `DataClient` với `LookupByPackage()`, `LookupByPURL()`, `Raw()`, `Close()`
- `services/shared/pkg/clients/grpc/ai_client.go` — `AIClient` với `EnrichCVE()`, `GetEPSS()`, `BatchEnrich()`, `Raw()`, `Close()`
- `services/shared/pkg/clients/grpc/search_client.go` — `SearchClient` REST với `Search()`
- `services/shared/pkg/go.mod` — thêm `github.com/osv/shared/proto v0.0.0` + replace directive

### Build Verification
```
go build ./clients/grpc/...  → OK
```
