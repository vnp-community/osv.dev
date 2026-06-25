# 04 — Service Client Layer (Shared Clients)

> **Mục tiêu**: Tạo shared client layer để CLI và apps/osv đều có thể gọi services
> theo cách nhất quán, bất kể giao thức (gRPC / REST / NATS).

---

## 1. Vị trí trong codebase

```
services/shared/pkg/clients/     ← THÊM VÀO đây (thêm file, không sửa existing)
├── [EXISTING] nats/              ← Giữ nguyên
├── [EXISTING] ...
└── [NEW] grpc/
    ├── data_client.go           ← data-service gRPC client
    ├── search_client.go         ← search-service gRPC client
    ├── ai_client.go             ← ai-service gRPC client
    ├── finding_client.go        ← finding-service gRPC client
    ├── identity_client.go       ← identity-service gRPC client
    ├── scan_client.go           ← scan-service gRPC client
    └── options.go               ← Common dial options (retry, timeout, TLS)
```

---

## 2. Common Dial Options

```go
// services/shared/pkg/clients/grpc/options.go
package grpc

import (
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"
)

// DefaultDialOptions returns standard gRPC dial options for internal communication.
// Uses insecure credentials (mTLS handled at network layer via service mesh or VPN).
func DefaultDialOptions() []grpc.DialOption {
    return []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                10 * time.Second,
            Timeout:             3 * time.Second,
            PermitWithoutStream: true,
        }),
        grpc.WithBlock(),
        grpc.WithReturnConnectionError(),
    }
}

// DialTimeout creates a new client connection with a 5-second timeout.
func DialTimeout(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    allOpts := append(DefaultDialOptions(), opts...)
    return grpc.DialContext(ctx, addr, allOpts...)
}
```

---

## 3. Data Service Client

```go
// services/shared/pkg/clients/grpc/data_client.go
package grpc

import (
    "context"

    "google.golang.org/grpc"
    cvedbv1 "github.com/osv/shared/proto/gen/go/cvedb/v1"
)

// DataClient wraps the cvedb gRPC service for vulnerability data operations.
// Used by: apps/cli (relations, worker), apps/osv (gateway proxy)
type DataClient struct {
    vuln  cvedbv1.CVEDBServiceClient
    conn  *grpc.ClientConn
}

// NewDataClient connects to data-service at the given address.
// Address example: "localhost:50053" or "data-service:50053"
func NewDataClient(addr string) (*DataClient, error) {
    conn, err := DialTimeout(addr)
    if err != nil {
        return nil, fmt.Errorf("data client: %w", err)
    }
    return &DataClient{
        vuln: cvedbv1.NewCVEDBServiceClient(conn),
        conn: conn,
    }, nil
}

// LookupByPackage queries CVEs for a given package and version.
// Maps to: POST /v1/query (OSV API) → cvedb.LookupCVEs gRPC
func (c *DataClient) LookupByPackage(ctx context.Context, ecosystem, pkg, version string) ([]*cvedbv1.CVEData, error) {
    resp, err := c.vuln.LookupCVEs(ctx, &cvedbv1.LookupCVEsRequest{
        Products: []*cvedbv1.ProductInfo{{
            Product: pkg,
            Version: version,
            // Map ecosystem string → purl
        }},
    })
    if err != nil {
        return nil, err
    }
    var results []*cvedbv1.CVEData
    for _, p := range resp.GetResults() {
        results = append(results, p.GetCves()...)
    }
    return results, nil
}

// GetByID fetches a single CVE record.
// Maps to: GET /v1/vulns/{id}
func (c *DataClient) GetByID(ctx context.Context, id string) (*cvedbv1.CVEData, error) {
    // TODO: Add GetCVE single-record RPC to proto
    // Workaround: LookupCVEs with exact CVE ID filter
    return nil, errors.New("not yet implemented — add GetCVE to cvedb.proto")
}

// Close releases the gRPC connection.
func (c *DataClient) Close() error { return c.conn.Close() }
```

---

## 4. Search Service Client

```go
// services/shared/pkg/clients/grpc/search_client.go
package grpc

// SearchClient wraps search-service for full-text vulnerability search.
// Used by: apps/osv (web UI search), apps/cli (query command)
type SearchClient struct {
    // TODO: search-service proto not yet finalized
    // Interim: use REST API
    httpClient *http.Client
    baseURL    string
}

func NewSearchClient(httpAddr string) *SearchClient {
    return &SearchClient{
        httpClient: &http.Client{Timeout: 10 * time.Second},
        baseURL:    "http://" + httpAddr,
    }
}

// Search performs full-text vulnerability search.
// Delegates to: GET http://search-service:8083/v1/search?q=...
func (c *SearchClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
    url := fmt.Sprintf("%s/v1/search?q=%s&limit=%d", c.baseURL, url.QueryEscape(query), limit)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result struct {
        Items []SearchResult `json:"items"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    return result.Items, nil
}

type SearchResult struct {
    CVEID    string  `json:"id"`
    Summary  string  `json:"summary"`
    Severity string  `json:"severity"`
    Score    float64 `json:"cvss_score"`
}
```

---

## 5. AI Service Client

```go
// services/shared/pkg/clients/grpc/ai_client.go
// (already exists in gateway-service — this is a shared version)
package grpc

import (
    aiv1 "github.com/osv/shared/proto/gen/go/ai/v1"
    "google.golang.org/grpc"
)

// AIClient wraps ai-service gRPC for enrichment operations.
// Used by: apps/cli (worker, enrich command), apps/osv (gateway BFF)
type AIClient struct {
    client aiv1.AIEnrichmentServiceClient
    conn   *grpc.ClientConn
}

func NewAIClient(addr string) (*AIClient, error) {
    conn, err := DialTimeout(addr)
    if err != nil {
        return nil, fmt.Errorf("ai client: %w", err)
    }
    return &AIClient{
        client: aiv1.NewAIEnrichmentServiceClient(conn),
        conn:   conn,
    }, nil
}

// EnrichCVE triggers AI enrichment for a single CVE.
// Used by cli/cmd/enrich and worker pipeline AIEnricher.
func (c *AIClient) EnrichCVE(ctx context.Context, cveID string) (*aiv1.EnrichCVEResponse, error) {
    return c.client.EnrichCVE(ctx, &aiv1.EnrichCVERequest{CveId: cveID})
}

// GetEPSS returns EPSS score for a CVE.
func (c *AIClient) GetEPSS(ctx context.Context, cveID string) (*aiv1.EPSSResponse, error) {
    return c.client.GetEPSS(ctx, &aiv1.GetEPSSRequest{CveId: cveID})
}

// BatchEnrich triggers batch enrichment.
// Used by cli/cmd/enrich --batch flag.
func (c *AIClient) BatchEnrich(ctx context.Context, cveIDs []string) (*aiv1.BatchEnrichResponse, error) {
    return c.client.BatchEnrich(ctx, &aiv1.BatchEnrichRequest{CveIds: cveIDs})
}

func (c *AIClient) Close() error { return c.conn.Close() }
```

---

## 6. OSV Query Client (REST — OSV v1 API)

```go
// services/shared/pkg/clients/rest/osv_client.go
// REST client cho OSV v1 API (compatible với public osv.dev API)
package rest

// OSVClient implements the OSV v1 REST API.
// Targets either:
//   - Public API: https://api.osv.dev
//   - Local gateway-service: http://localhost:8080
type OSVClient struct {
    baseURL    string
    httpClient *http.Client
}

func NewOSVClient(baseURL string) *OSVClient {
    return &OSVClient{
        baseURL:    strings.TrimSuffix(baseURL, "/"),
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

// QueryByPackage implements POST /v1/query with package+version.
// Satisfies URD UR-01: query by package and version.
func (c *OSVClient) QueryByPackage(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/query", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var result QueryResponse
    return &result, json.NewDecoder(resp.Body).Decode(&result)
}

// GetVuln implements GET /v1/vulns/{id}.
// Satisfies URD UR-02: get vulnerability details.
func (c *OSVClient) GetVuln(ctx context.Context, id string) (*osvschema.Vulnerability, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/vulns/"+id, nil)
    resp, err := c.httpClient.Do(req)
    // ...
}

// QueryBatch implements POST /v1/querybatch — bulk query.
// Satisfies URD UR-06: bulk data access.
func (c *OSVClient) QueryBatch(ctx context.Context, queries []QueryRequest) ([]QueryResponse, error) {
    // POST /v1/querybatch
}

type QueryRequest struct {
    Package *PackageInfo `json:"package,omitempty"`
    Version string       `json:"version,omitempty"`
    Commit  string       `json:"commit,omitempty"`
}

type PackageInfo struct {
    Ecosystem string `json:"ecosystem"`
    Name      string `json:"name"`
}
```

---

## 7. NATS Event Types (Shared)

```go
// services/shared/pkg/events/types.go  ← NEW file
package events

// Subject constants — NATS JetStream subjects.
// Published by services, consumed by apps.
const (
    SubjectVulnImported           = "osv.vuln.imported"            // data-service → all
    SubjectVulnUpdated            = "osv.vuln.updated"             // data-service → search, ai
    SubjectAIEnrichmentCompleted  = "osv.ai.enrichment.completed"  // ai-service → search
    SubjectFindingCreated         = "defectdojo.finding.created"   // finding-service → notif
    SubjectScanCompleted          = "osv.scan.completed"           // scan-service → finding
    SubjectSLABreached            = "finding.sla.breached"         // finding-service → notif
)

// VulnImportedEvent is published when a new vulnerability is imported.
type VulnImportedEvent struct {
    ID          string    `json:"id"`
    Source      string    `json:"source"`       // "nvd", "ghsa", "ossfuzz", etc.
    ImportedAt  time.Time `json:"imported_at"`
    OSVData     []byte    `json:"osv_data"`     // JSON-encoded OSV schema Vulnerability
}

// AIEnrichmentCompletedEvent is published after AI enrichment.
type AIEnrichmentCompletedEvent struct {
    CVEID       string    `json:"cve_id"`
    Provider    string    `json:"provider"`     // "vertex", "openai", "ollama"
    CompletedAt time.Time `json:"completed_at"`
    HasVector   bool      `json:"has_vector"`
}
```
