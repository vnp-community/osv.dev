# 05 — gRPC Integration Details

> **Mục đích**: Chi tiết cách gateway-service và apps/cli gọi các services qua gRPC.

---

## 1. gateway-service — Hoàn thiện OSV Router

Hiện tại `cmd/server/main.go` có stub routes. Cần wire gRPC calls:

### Thêm vào `gateway-service/cmd/server/main.go`:

```go
// gateway-service/cmd/server/main.go — Cập nhật osvRouter()
func osvRouter(dataAddr, searchAddr, aiAddr string) http.Handler {
    // gRPC clients
    dataConn, _ := grpc.NewClient(dataAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    cveClient := cvedbv1.NewCVEDBServiceClient(dataConn)
    
    searchClient := grpcclient.NewSearchHTTPClient("http://" + searchAddr)
    aiClient := grpcclient.NewAIClient(aiAddr)

    r := chi.NewRouter()
    
    // GET /v1/vulns/{id} — Get vuln by ID (UR-02, UR-04)
    r.Get("/vulns/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        // Call data-service via gRPC LookupCVEs
        resp, err := cveClient.LookupCVEs(r.Context(), &cvedbv1.LookupCVEsRequest{
            Products: []*cvedbv1.ProductInfo{{Purl: "pkg:generic/" + id}},
        })
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    })
    
    // POST /v1/query — Query by package+version or commit (UR-01, UR-04)
    r.Post("/query", handleOSVQuery(cveClient))
    
    // POST /v1/querybatch — Batch query (UR-06)
    r.Post("/querybatch", handleOSVQueryBatch(cveClient))
    
    // GET /v1/search — Full-text search (UR-01)
    r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
        q := r.URL.Query().Get("q")
        results, err := searchClient.Search(r.Context(), q, 20)
        // ...
    })
    
    return r
}

func handleOSVQuery(client cvedbv1.CVEDBServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Package *struct {
                Ecosystem string `json:"ecosystem"`
                Name      string `json:"name"`
            } `json:"package,omitempty"`
            Version string `json:"version,omitempty"`
            Commit  string `json:"commit,omitempty"`
        }
        json.NewDecoder(r.Body).Decode(&req)
        
        products := []*cvedbv1.ProductInfo{}
        if req.Package != nil {
            products = append(products, &cvedbv1.ProductInfo{
                Product: req.Package.Name,
                Version: req.Version,
                Purl:    buildPURL(req.Package.Ecosystem, req.Package.Name, req.Version),
            })
        }
        
        resp, err := client.LookupCVEs(r.Context(), &cvedbv1.LookupCVEsRequest{
            Products: products,
        })
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        // Convert gRPC response → OSV JSON schema
        vulns := convertToOSVSchema(resp)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{"vulns": vulns})
    }
}
```

---

## 2. CLI cmd/importer — Config Extension

```go
// apps/cli/internal/importer/importer.go — Thêm NATSPublisher interface

// Publisher interface abstracts GCP Pub/Sub and NATS.
// (Extends existing clients.Publisher if compatible, otherwise parallel type)
type VulnPublisher interface {
    PublishVuln(ctx context.Context, id string, osvData []byte) error
}

// GCPPublisherAdapter wraps existing GCP publisher.
type GCPPublisherAdapter struct {
    inner clients.Publisher
}

func (a *GCPPublisherAdapter) PublishVuln(ctx context.Context, id string, osvData []byte) error {
    return a.inner.Publish(ctx, &pubsub.Message{Data: osvData})
}
```

---

## 3. CLI cmd/worker — Pipeline Extension

```go
// apps/cli/internal/worker/worker.go — Thêm AI enricher registration

// RegisteredEnrichers holds all pipeline enrichers.
// New enrichers are added via RegisterEnricher — existing ones not removed.
var RegisteredEnrichers = map[string]pipeline.Enricher{
    // existing enrichers giữ nguyên...
}

// RegisterEnricher adds a new enricher to the pipeline (additive).
func RegisterEnricher(name string, e pipeline.Enricher) {
    RegisteredEnrichers[name] = e
}
```

---

## 4. Proto Requirements — Services cần expose

### data-service cần thêm RPC:

```protobuf
// cần thêm vào shared/proto/cvedb/v1/cvedb.proto:
service CVEDBService {
    // existing...
    rpc LookupCVEs(LookupCVEsRequest) returns (LookupCVEsResponse);
    
    // NEW — needed by gateway OSV v1 API:
    rpc GetCVE(GetCVERequest) returns (CVEData);
    rpc ListCVEs(ListCVEsRequest) returns (stream CVEData);  // for export
    rpc QueryByCommit(QueryByCommitRequest) returns (LookupCVEsResponse);
    
    // NEW — needed by cli/relations:
    rpc GetAliases(GetAliasesRequest) returns (AliasGroup);
    rpc ComputeAliases(ComputeAliasesRequest) returns (AliasGroup);
}
```

### search-service cần expose:

```protobuf
// cần tạo shared/proto/search/v1/search.proto:
service SearchService {
    rpc Search(SearchRequest) returns (SearchResponse);
    rpc IndexVuln(IndexVulnRequest) returns (google.protobuf.Empty);
    rpc BulkIndex(stream IndexVulnRequest) returns (BulkIndexResponse);
}
```

### scan-service cần expose:

```protobuf
// cần tạo shared/proto/scan/v1/scan.proto:
service ScanService {
    rpc SubmitScan(SubmitScanRequest) returns (ScanJob);
    rpc GetScanResult(GetScanResultRequest) returns (ScanResult);
    rpc ListScanJobs(ListScanJobsRequest) returns (ListScanJobsResponse);
}
```

---

## 5. Backward Compatibility Matrix

| CLI Command | GCP Backend | Microservices Backend | Selector |
|-------------|-------------|----------------------|----------|
| `importer` | Pub/Sub → GCP Datastore | NATS → data-service | `CLI_BACKEND=microservices` |
| `worker` | GCS + Datastore (inline) | gRPC → ai-service | `AI_ENRICHER_ADDR=...` |
| `exporter` | GCS bucket read | REST → data-service | `EXPORTER_BACKEND=api` |
| `relations` | GCP Datastore | gRPC → data-service | `DATA_SERVICE_ADDR=...` |
| `recordchecker` | GCP Datastore | REST → data-service | `DATA_SERVICE_ADDR=...` |
| `generatesitemap` | GCP Datastore | REST → gateway /sitemap | `SITEMAP_BACKEND=api` |

---

## 6. grpc ClientConn Reuse Pattern

Để tránh tạo nhiều connections, dùng connection pool:

```go
// apps/osv/internal/orchestrator/grpc_pool.go  ← NEW
package orchestrator

// GRPCPool manages shared gRPC connections to all services.
// Used when apps/osv runs embedded — all services are localhost.
type GRPCPool struct {
    DataConn     *grpc.ClientConn
    SearchConn   *grpc.ClientConn
    AIConn       *grpc.ClientConn
    FindingConn  *grpc.ClientConn
    IdentityConn *grpc.ClientConn
    ScanConn     *grpc.ClientConn
}

func NewGRPCPool(cfg *Config) (*GRPCPool, error) {
    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    }
    
    data, err := grpc.NewClient(cfg.Gateway.DataAddr, opts...)
    if err != nil {
        return nil, fmt.Errorf("grpc pool: data-service: %w", err)
    }
    
    // ... other connections
    
    return &GRPCPool{
        DataConn:    data,
        // ...
    }, nil
}

func (p *GRPCPool) Close() {
    p.DataConn.Close()
    p.SearchConn.Close()
    // ...
}
```
