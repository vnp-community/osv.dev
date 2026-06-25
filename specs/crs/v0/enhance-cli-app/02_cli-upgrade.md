# 02 — CLI Upgrade (`apps/cli`)

> **Nguyên tắc**: GIỮ tất cả code cũ (GCP Datastore + Pub/Sub). THÊM backend microservices
> via config selector `CLI_BACKEND=gcp|microservices`.

---

## 1. Phân tích code hiện tại

### Các commands hiện có
| Command | File | Chức năng | Backend hiện tại |
|---------|------|-----------|-----------------|
| `importer` | `cmd/importer/main.go` | Import vuln từ Git/GCS/REST sources | GCP Datastore + Pub/Sub |
| `worker` | `cmd/worker/main.go` | Process enrichment tasks từ Pub/Sub | GCP Datastore + GCS |
| `exporter` | `cmd/exporter/exporter.go` | Export DB ra GCS buckets | GCP GCS |
| `relations` | `cmd/relations/relations.go` | Compute alias/related vuln | GCP Datastore |
| `recordchecker` | `cmd/recordchecker/recordchecker.go` | Validate records vs GCS | GCP Datastore + GCS |
| `gitter` | `cmd/gitter/gitter.go` | Git repo operations | Git |
| `generatesitemap` | `cmd/generatesitemap/generatesitemap.go` | Sitemap generation | GCP Datastore |
| `custommetrics` | `cmd/custommetrics/main.go` | Metrics export | GCP Monitoring |
| `osv-linter-worker` | `cmd/osv-linter-worker/main.go` | Linting worker | Pub/Sub |
| `extract_versions` | `cmd/extract_versions/main.go` | Version extraction | Inline |
| `first_package_finder` | `cmd/first_package_finder/main.go` | First package lookup | Inline |

---

## 2. Giải pháp nâng cấp từng command

### 2.1 `cmd/importer` — Dual Backend

**Vấn đề**: Hiện dùng GCP Pub/Sub để send tasks đến worker. Cần NATS alternative.

**Giải pháp — Thêm NATS publisher**:
```
internal/importer/
├── importer.go           ← GIỮ NGUYÊN (GCP Pub/Sub)
├── rest.go               ← GIỮ NGUYÊN
├── git.go                ← GIỮ NGUYÊN
├── bucket.go             ← GIỮ NGUYÊN
└── [NEW] nats_publisher.go  ← NATS alternative publisher
```

**Thêm vào `cmd/importer/main.go`** (không sửa existing logic):
```go
// Thêm NATS backend selector
backend := os.Getenv("CLI_BACKEND") // "gcp" (default) | "microservices"
if backend == "microservices" {
    natsURL := os.Getenv("NATS_URL")
    nc, _ := nats.Connect(natsURL)
    config.Publisher = importer.NewNATSPublisher(nc)  // NEW
    // Gửi event osv.vuln.imported thay vì Pub/Sub task
} else {
    // existing GCP Pub/Sub setup giữ nguyên
    config.Publisher = &clients.GCPPublisher{...}
}
```

**File mới**: `internal/importer/nats_publisher.go`
```go
// NATSPublisher publishes vuln import events to NATS JetStream.
// Thay thế GCPPublisher khi CLI_BACKEND=microservices.
// Event subject: "osv.vuln.imported"
// Payload: JSON-encoded vulnerability record (OSV schema)
type NATSPublisher struct {
    js nats.JetStreamContext
}

func (p *NATSPublisher) Publish(ctx context.Context, msg *pubsub.Message) error {
    // Convert GCP PubSub message format → NATS event
    return p.js.Publish("osv.vuln.imported", msg.Data)
}
```

---

### 2.2 `cmd/worker` — gRPC AI Enrichment

**Vấn đề**: Worker hiện dùng inline enrichment pipeline. Cần delegate đến ai-service.

**Giải pháp — Thêm AI gRPC enricher**:
```
internal/worker/pipeline/
├── enrich.go             ← GIỮ NGUYÊN (Enricher interface)
├── sourcelink/           ← GIỮ NGUYÊN
├── namenormalize/        ← GIỮ NGUYÊN
└── [NEW] ai_enricher.go  ← gRPC delegate đến ai-service
```

**File mới**: `internal/worker/pipeline/ai_enricher.go`
```go
// AIEnricher delegates vulnerability enrichment to ai-service via gRPC.
// Implements the existing Enricher interface — drop-in replacement.
//
// Kích hoạt: AI_ENRICHER_ADDR=ai-service:50052
type AIEnricher struct {
    client aiv1.AIEnrichmentServiceClient
}

func (e *AIEnricher) Enrich(ctx context.Context, vuln *osvschema.Vulnerability, params *EnrichParams) error {
    resp, err := e.client.EnrichCVE(ctx, &aiv1.EnrichCVERequest{
        CveId: vuln.ID,
    })
    if err != nil {
        return fmt.Errorf("ai-service.EnrichCVE: %w", err)
    }
    // Populate vuln với AI enrichment (non-destructive, only fills empty fields)
    if vuln.Summary == "" {
        vuln.Summary = resp.GetSummaryShort()
    }
    if vuln.Details == "" {
        vuln.Details = resp.GetSummaryLong()
    }
    return nil
}
```

**Cập nhật `cmd/worker/main.go`** (thêm pipeline stage):
```go
// Thêm AI enricher nếu addr được set
aiAddr := os.Getenv("AI_ENRICHER_ADDR")
if aiAddr != "" {
    conn, _ := grpc.NewClient(aiAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    aiClient := aiv1.NewAIEnrichmentServiceClient(conn)
    pipeline.RegisterEnricher("ai", worker.NewAIEnricher(aiClient))  // NEW
}
// existing pipeline stages giữ nguyên
```

---

### 2.3 `cmd/relations` — gRPC Alias Service

**Vấn đề**: Relations dùng GCP Datastore trực tiếp. Cần delegate đến data-service.

**Giải pháp — Thêm gRPC client wrapper**:
```
cmd/relations/
├── relations.go      ← GIỮ NGUYÊN entry point
├── alias.go          ← GIỮ NGUYÊN (GCP Datastore logic)
├── related.go        ← GIỮ NGUYÊN
├── upstream.go       ← GIỮ NGUYÊN
└── [NEW] grpc_backend.go  ← data-service gRPC alternative
```

**File mới**: `cmd/relations/grpc_backend.go`
```go
// GRPCRelationsBackend delegates alias computation to data-service.
// Activated when DATA_SERVICE_ADDR is set.
type GRPCRelationsBackend struct {
    addr string  // e.g. "data-service:50053"
}

func (b *GRPCRelationsBackend) ComputeAliases(ctx context.Context, vulnID string) ([]string, error) {
    conn, _ := grpc.NewClient(b.addr, ...)
    // Call data-service AliasGroup RPC
    // TODO: wire to data-service proto once cvedb.v1 AliasService is defined
    return nil, nil
}
```

---

### 2.4 `cmd/exporter` — REST API Export

**Vấn đề**: Exporter đọc trực tiếp từ GCS. Cần REST alternative từ data-service.

**Giải pháp — Thêm REST downloader**:
```
cmd/exporter/
├── exporter.go      ← GIỮ NGUYÊN
├── downloader.go    ← GIỮ NGUYÊN (GCS downloader)
├── worker.go        ← GIỮ NGUYÊN
├── writer.go        ← GIỮ NGUYÊN
└── [NEW] api_downloader.go  ← REST API alternative
```

**File mới**: `cmd/exporter/api_downloader.go`
```go
// APIDownloader downloads vulnerability records from data-service REST API.
// Replaces GCS downloader when EXPORTER_BACKEND=api.
//
// Endpoint: GET http://data-service:8082/api/v1/vulns?limit=1000&offset=N
type APIDownloader struct {
    baseURL    string
    httpClient *http.Client
}

func (d *APIDownloader) DownloadAll(ctx context.Context, fn func(*osvschema.Vulnerability) error) error {
    offset := 0
    for {
        url := fmt.Sprintf("%s/api/v1/vulns?limit=1000&offset=%d", d.baseURL, offset)
        resp, err := d.httpClient.Get(url)
        // ... decode JSON, call fn for each vuln
        offset += 1000
        if total <= offset { break }
    }
    return nil
}
```

---

### 2.5 [NEW] `cmd/scan` — Scan Service CLI

**Chức năng mới theo PRD**: Security scanning cho container images và filesystems.

```
cmd/scan/         ← HOÀN TOÀN MỚI
└── main.go
```

```go
// cmd/scan/main.go — Trigger scan qua scan-service gRPC
//
// Usage:
//   osv-cli scan image nginx:latest
//   osv-cli scan dir /path/to/project
//   osv-cli scan sbom /path/to/sbom.json
package main

import (
    "flag"
    "fmt"
    "os"

    "google.golang.org/grpc"
    scanv1 "github.com/osv/shared/proto/gen/go/scan/v1"
)

func main() {
    addr := os.Getenv("SCAN_SERVICE_ADDR") // "scan-service:50055"
    if addr == "" {
        addr = "localhost:8087"
    }

    target := flag.String("target", "", "Container image or directory path")
    scanType := flag.String("type", "image", "image|dir|sbom")
    flag.Parse()

    conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    // ...
    client := scanv1.NewScanServiceClient(conn)

    // Submit scan job
    resp, err := client.SubmitScan(ctx, &scanv1.SubmitScanRequest{
        Target:   *target,
        ScanType: *scanType,
    })
    fmt.Printf("Scan job submitted: %s\n", resp.GetJobId())
    // Poll for results
}
```

---

### 2.6 [NEW] `cmd/query` — Query Gateway CLI

**Chức năng**: Query vulnerability theo package/version/commit qua gateway-service.

```
cmd/query/        ← HOÀN TOÀN MỚI
└── main.go
```

```go
// cmd/query/main.go — Query vulnerabilities via gateway REST API
//
// Usage:
//   osv-cli query --package lodash --version 4.17.20 --ecosystem npm
//   osv-cli query --cve CVE-2021-44228
//   osv-cli query --commit abc123def456
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "net/http"
    "os"
)

func main() {
    gatewayURL := os.Getenv("GATEWAY_URL") // "http://localhost:8080"
    if gatewayURL == "" {
        gatewayURL = "http://localhost:8080"
    }

    pkg := flag.String("package", "", "Package name")
    version := flag.String("version", "", "Package version")
    ecosystem := flag.String("ecosystem", "", "Ecosystem (npm, PyPI, Maven, etc.)")
    cveID := flag.String("cve", "", "CVE ID")
    commit := flag.String("commit", "", "Git commit hash")
    flag.Parse()

    if *cveID != "" {
        // GET /v1/vulns/{id}
        resp, _ := http.Get(fmt.Sprintf("%s/v1/vulns/%s", gatewayURL, *cveID))
        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        printJSON(result)
        return
    }

    // POST /v1/query
    body := buildQueryBody(*pkg, *version, *ecosystem, *commit)
    resp, _ := http.Post(gatewayURL+"/v1/query", "application/json", body)
    // ... decode and print
}
```

---

### 2.7 [NEW] `cmd/enrich` — Manual AI Enrichment

```
cmd/enrich/       ← HOÀN TOÀN MỚI
└── main.go
```

```go
// cmd/enrich/main.go — Trigger AI enrichment for specific CVEs
//
// Usage:
//   osv-cli enrich --cve CVE-2021-44228
//   osv-cli enrich --batch --file cve-list.txt
package main
// Calls ai-service gRPC: EnrichCVE or BatchEnrich
```

---

## 3. go.mod thay đổi cho `apps/cli`

```go
// apps/cli/go.mod — Thêm (không xóa existing):

require (
    // ... existing deps ...
    
    // NEW — microservices clients
    github.com/nats-io/nats.go v1.42.0          // NATS publisher
    github.com/osv/shared/proto v0.0.0           // gRPC proto types
)

replace (
    // ... existing replaces ...
    github.com/osv/shared/proto => ../../services/shared/proto  // NEW
)
```

---

## 4. Makefile thay đổi

```makefile
# Thêm vào apps/cli/Makefile (không xóa existing targets):

.PHONY: build-scan build-query build-enrich

build-scan:
	go build -o bin/osv-scan ./cmd/scan/...

build-query:
	go build -o bin/osv-query ./cmd/query/...

build-enrich:
	go build -o bin/osv-enrich ./cmd/enrich/...

build-all-new: build-scan build-query build-enrich

# Integration test với microservices backend
test-microservices:
	CLI_BACKEND=microservices \
	NATS_URL=nats://localhost:4222 \
	DATA_SERVICE_ADDR=localhost:50053 \
	go test ./... -tags=microservices
```
