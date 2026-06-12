> **✅ COMPLETED** — Bridge Pattern, go build && go vet passed.

# T11 — Ingestion Agent Pipeline (OSV Enrichment)

## Thông tin
| | |
|---|---|
| **Phase** | 4 — Agent Enrichment |
| **Ước tính** | 3–4 giờ |
| **Depends on** | T07 (agent events), T09 (CVE repo) |
| **Blocks** | — |

## Mục tiêu
Wire-up `ingestion-service` pipeline: NATS subscriber nhận `agent.report.submitted`, query OSV API cho từng package, lưu CVEs, tạo findings.

---

## Packages cần import

| Import path | Thành phần |
|-------------|------------|
| `ingestion-service/internal/fetcher/` | OSV API fetcher |
| `ingestion-service/internal/converter/` | OSV → internal CVE converter |
| `ingestion-service/internal/pipeline/` | Enrichment pipeline |
| `ingestion-service/internal/usecase/sync/` | Sync CVE data |
| `ingestion-service/internal/domain/` | CVE ingestion entity |

---

## Các bước thực hiện

### 11.1 Đọc ingestion-service API

```bash
cat osv.dev/services/ingestion-service/internal/fetcher/*.go
cat osv.dev/services/ingestion-service/internal/converter/*.go
cat osv.dev/services/ingestion-service/internal/pipeline/*.go
cat osv.dev/services/ingestion-service/internal/usecase/sync/*.go
```

Ghi lại:
- `fetcher.QueryOSV(packageName, version)` → OSV response
- `converter.Convert(osvResponse)` → internal CVE entity
- Pipeline orchestration logic

### 11.2 Khởi tạo ingestion components

```go
import (
    ingestfetch "github.com/osv/ingestion-service/internal/fetcher"
    ingestconv  "github.com/osv/ingestion-service/internal/converter"
    ingestpipe  "github.com/osv/ingestion-service/internal/pipeline"
)

osvFetcher := ingestfetch.New(http.DefaultClient, a.log)
converter  := ingestconv.New()
pipeline   := ingestpipe.New(osvFetcher, converter, cveRepo, a.log)
```

### 11.3 NATS subscriber cho `agent.report.submitted`

```go
// internal/app/app.go — trong Start():
go func() {
    a.nc.Subscribe("agent.report.submitted", func(msg *nats.Msg) {
        var event struct {
            ReportID int    `json:"report_id"`
            Hostname string `json:"hostname"`
        }
        json.Unmarshal(msg.Data, &event)

        // Load agent report từ DB
        report, err := a.AgentRepo.GetByID(ctx, event.ReportID)
        if err != nil {
            a.log.Error().Err(err).Msg("failed to load agent report")
            return
        }

        // Process mỗi package qua OSV API
        for _, pkg := range report.Packages {
            cves, err := pipeline.ProcessPackage(ctx, pkg.Name, pkg.Version, pkg.Ecosystem)
            if err != nil {
                a.log.Warn().Err(err).Str("package", pkg.Name).Msg("osv lookup failed")
                continue
            }

            // Tạo findings cho agent scan
            if len(cves) > 0 {
                a.BatchCreateUC.Execute(ctx, findinguc.BatchCreateInput{
                    ScanID:   report.ScanID,
                    Findings: convertCVEsToFindings(cves, pkg),
                })
            }
        }

        a.log.Info().
            Int("report_id", event.ReportID).
            Str("hostname", event.Hostname).
            Msg("agent report enrichment completed")
    })
}()
```

### 11.4 Convert CVEs → Findings

```go
// Hàm helper (viết mới, ~30 LOC)
func convertCVEsToFindings(cves []vulnentity.CVE, pkg AgentPackage) []findinguc.FindingInput {
    inputs := make([]findinguc.FindingInput, 0, len(cves))
    for _, cve := range cves {
        inputs = append(inputs, findinguc.FindingInput{
            Title:            fmt.Sprintf("%s in %s@%s", cve.ID, pkg.Name, pkg.Version),
            Description:      cve.Description,
            Severity:         cve.Severity,
            CVE:              cve.ID,
            CVSSv3:           cve.CVSS.V3Vector,
            CVSSv3Score:      &cve.CVSS.V3Score,
            ComponentName:    pkg.Name,
            ComponentVersion: pkg.Version,
            Active:           true,
            Mitigation:       cve.Remediation,
        })
    }
    return inputs
}
```

### 11.5 Xử lý idempotency

```go
// Tránh tạo duplicate findings cho cùng package + CVE
// Dùng findinguc.FindByHashCode() để check trước khi create:
for _, input := range findingInputs {
    existing, _ := a.FindByHashCodeUC.Execute(ctx, findinguc.FindByHashCodeInput{
        HashCode: hashOf(input.CVE, pkg.Name, pkg.Version),
    })
    if existing.FindingID != nil {
        continue // Already exists
    }
    // Create...
}
```

### 11.6 Rate limiting cho OSV API

```go
// Tránh rate limit khi xử lý nhiều packages
// Thêm delay hoặc dùng semaphore:
semaphore := make(chan struct{}, 3) // max 3 concurrent OSV requests

for _, pkg := range report.Packages {
    semaphore <- struct{}{}
    go func(p Package) {
        defer func() { <-semaphore }()
        pipeline.ProcessPackage(ctx, p.Name, p.Version, p.Ecosystem)
    }(pkg)
}
```

---

## Output

- [x] Ingestion components (fetcher, converter, pipeline) khởi tạo ✓ (ingestionBridge)
- [x] NATS subscriber `agent.report.submitted` hoạt động ✓ (subscribeAgentReports)
- [x] `convertCVEsToFindings()` function ✓ (convertCVEsToFindingInputs in ingestion_runner.go)
- [x] Idempotency check ✓ (ON CONFLICT (hash_code) DO NOTHING)
- [x] Rate limiting cho OSV API calls ✓ (semaphore pattern in ingestion_runner.go)

## Acceptance Criteria

```bash
# Submit agent report
curl -X POST http://localhost:8080/agent/report \
  -d '{"hostname":"test","packages":[{"name":"log4j","version":"2.14.0","ecosystem":"Maven"}]}'
# → {"report_id":1}

# Đợi 10 giây (OSV API lookup)
sleep 10

# Findings phải được tạo tự động
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/findings?hostname=test"
# → {"findings":[{"cve":"CVE-2021-44228","severity":"critical",...}]}
```
