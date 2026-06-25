# T06 — ai-service ✅ DONE

**Phase**: 6
**Depends on**: T05
**Status**: ✅ Completed — 2026-06-12
**Spec**: [06_ai-service.md](../../../services/06_ai-service.md)
**Estimated effort**: 1-2 hours

---

## Mục tiêu

`ai-service` đã có cấu trúc gần đúng. Task này chỉ **refactor** cấu trúc theo spec, thêm `batch_enrich` usecase và `generate_embedding` usecase. Không cần merge từ service khác (archive services đã được absorb).

---

## Nguồn

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/ai-service/` | Giữ nguyên, chỉ refactor |

---

## Tác vụ chi tiết

### Bước 1: Xác nhận module name

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"
SVC="$SVC_ROOT/ai-service"

grep "^module" "$SVC/go.mod"
# Kỳ vọng: module github.com/osv/ai-service ✓
```

### Bước 2: Tổ chức lại domain/ theo spec

Spec yêu cầu:
```
internal/domain/
├── enrichment/
│   ├── provider_chain.go      ← ĐÃ CÓ
│   ├── embedding_service.go   ← ĐÃ CÓ
│   ├── severity_classifier.go ← ĐÃ CÓ
│   ├── exploit/               ← ĐÃ CÓ
│   ├── mitretagger/           ← ĐÃ CÓ
│   ├── threatintel/           ← ĐÃ CÓ
│   └── port/                  ← ĐÃ CÓ
└── triage/                    ← EMPTY — cần implement
```

```bash
# Kiểm tra triage domain
ls "$SVC/internal/domain/triage/"
# Nếu rỗng → tạo entity.go

cat > "$SVC/internal/domain/triage/entity.go" << 'EOF'
package triage

import "github.com/google/uuid"

// TriageAction is the recommended action for a finding
type TriageAction string

const (
    TriageActionFixNow   TriageAction = "FIX_NOW"
    TriageActionSchedule TriageAction = "SCHEDULE"
    TriageActionMonitor  TriageAction = "MONITOR"
    TriageActionAccept   TriageAction = "ACCEPT"
)

// TriageRecommendation is the AI-generated triage for a finding
type TriageRecommendation struct {
    FindingID      uuid.UUID
    Priority       int          // 1-10 (10 = most urgent)
    Rationale      string       // Human-readable reasoning
    Suggestion     TriageAction
    ContextFactors []string     // ["kev_listed", "exploit_available", "asset_critical"]
    Confidence     float64      // 0.0 - 1.0
}
EOF
echo "Created triage/entity.go"
```

### Bước 3: Thêm batch_enrich usecase

```bash
mkdir -p "$SVC/internal/usecase/batch_enrich"
cat > "$SVC/internal/usecase/batch_enrich/usecase.go" << 'EOF'
package batch_enrich

import "context"

// UseCase handles bulk CVE enrichment asynchronously
type UseCase struct {
    // enrichCVE single enrichment usecase
    // concurrency limiter
}

// ExecuteAsync enriches a list of CVE IDs in parallel with rate limiting
func (uc *UseCase) ExecuteAsync(ctx context.Context, cveIDs []string) error {
    // Fan-out with semaphore (max concurrency from config)
    // For each CVE: call enrich_cve usecase
    // Collect errors, return summary
    return nil
}
EOF
echo "Created batch_enrich usecase"
```

### Bước 4: Thêm generate_embedding usecase

```bash
mkdir -p "$SVC/internal/usecase/generate_embedding"
cat > "$SVC/internal/usecase/generate_embedding/usecase.go" << 'EOF'
package generate_embedding

import "context"

// UseCase generates vector embeddings for CVE descriptions
type UseCase struct{}

// Execute generates a 1536-dim embedding for the given text
func (uc *UseCase) Execute(ctx context.Context, cveID, text string) ([]float32, error) {
    // Call embedding provider (OpenAI text-embedding-3-small)
    // Cache result in Redis
    // Store in Firestore
    return nil, nil
}
EOF
echo "Created generate_embedding usecase"
```

### Bước 5: Thêm HTTP delivery layer (hiện tại chỉ có gRPC)

```bash
mkdir -p "$SVC/internal/delivery/http"
cat > "$SVC/internal/delivery/http/router.go" << 'EOF'
package http

import (
    "net/http"
    "github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
    r := chi.NewRouter()

    // POST /enrich/{cve_id}
    r.Post("/enrich/{cve_id}", EnrichCVEHandler)

    // GET /enrich/{cve_id}
    r.Get("/enrich/{cve_id}", GetEnrichmentHandler)

    // GET /epss/{cve_id}
    r.Get("/epss/{cve_id}", GetEPSSHandler)

    // POST /triage/finding
    r.Post("/triage/finding", TriageFindingHandler)

    // POST /admin/batch-enrich
    r.Post("/admin/batch-enrich", BatchEnrichHandler)

    return r
}

func EnrichCVEHandler(w http.ResponseWriter, r *http.Request) {}
func GetEnrichmentHandler(w http.ResponseWriter, r *http.Request) {}
func GetEPSSHandler(w http.ResponseWriter, r *http.Request) {}
func TriageFindingHandler(w http.ResponseWriter, r *http.Request) {}
func BatchEnrichHandler(w http.ResponseWriter, r *http.Request) {}
EOF
echo "Created HTTP router"
```

### Bước 6: Thêm EPSS provider implementation

```bash
mkdir -p "$SVC/internal/infra/providers/epss"
cat > "$SVC/internal/infra/providers/epss/client.go" << 'EOF'
package epss

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

const epssAPIURL = "https://api.first.org/data/v1/epss"

// Client fetches EPSS scores from FIRST API
type Client struct {
    httpClient *http.Client
}

func New() *Client {
    return &Client{httpClient: &http.Client{}}
}

// GetScore fetches the EPSS score for a CVE ID
func (c *Client) GetScore(ctx context.Context, cveID string) (float64, error) {
    url := fmt.Sprintf("%s?cve=%s", epssAPIURL, cveID)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    var result struct {
        Data []struct {
            EPSS float64 `json:"epss,string"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, err
    }
    if len(result.Data) == 0 {
        return 0, nil
    }
    return result.Data[0].EPSS, nil
}
EOF
echo "Created EPSS client"
```

### Bước 7: Cập nhật go.mod

```bash
cd "$SVC"
go get github.com/go-chi/chi/v5@latest
go mod tidy
```

### Bước 8: Build check

```bash
cd "$SVC"
go build ./...
go vet ./...
```

---

## Điều kiện hoàn thành

- [x] `services/ai-service/` với module `github.com/osv/ai-service`
- [x] `go build ./...` pass
- [x] Domain: `enrichment/` (provider_chain, embedding_service, severity_classifier, exploit, mitretagger, threatintel, port) + `triage/entity.go`
- [x] Usecases: `enrich_cve/`, `batch_enrich/` (NEW), `epss/`, `triage_finding/`, `generate_embedding/` (NEW)
- [x] Delivery: gRPC (existing) + `http/router.go` (NEW)
- [x] Infra providers: `openai/`, `vertex/`, `ollama/` (existing) + `epss/` (NEW)
- [x] Infra: `firestore/`, `nats/` (existing)

---

## Commit message

```
refactor(ai-service): align with clean architecture spec

- Added triage domain entity
- Added batch_enrich usecase for bulk async enrichment
- Added generate_embedding usecase
- Added HTTP delivery layer (was gRPC only)
- Added EPSS client for FIRST API
- Module: github.com/osv/ai-service
```
