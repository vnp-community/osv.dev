# TASK-GCV-004 — CNNVD Fetcher + Scheduler Update

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-GCV-004 |
| **Service** | `data-service` |
| **CR** | CR-GCV-001 |
| **Phase** | 1 — Core Pipeline |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-GCV-001 |

## Context

Tạo `CNNVDFetcher` (Chinese NVD — beta source), sau đó update scheduler để đăng ký tất cả fetchers mới với cron schedules tương ứng. Scheduler cũng cần wire `Registry` để fetcher có thể trigger bằng source name.

## Reference

- Solution: [SOL-GCV-001](../solutions/SOL-GCV-001-multi-source-fetcher.md) §4.6, §2.5
- CR: [CR-GCV-001](../CR-GCV-001-multi-source-fetcher-pipeline.md) §4.6, §6

## Files to Create/Modify

```
CREATE: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/fetcher/cnnvd.go
MODIFY: /Users/binhnt/Lab/sec/cve/osv.dev/services/data-service/internal/delivery/scheduler/scheduler.go
```

## Implementation Spec

### cnnvd.go (beta)

```go
// Package fetcher — CNNVD (Chinese NVD) fetcher. Beta source.
// Source: https://www.cnnvd.org.cn
// NOTE: API may require scraping — implement with graceful error handling.
package fetcher

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/rs/zerolog"
    entity "github.com/osv/data-service/internal/domain/entity"
    "github.com/osv/data-service/internal/domain/repository"
)

const cnnvdBaseURL = "https://www.cnnvd.org.cn"

type CNNVDFetcher struct {
    baseURL string
    client  *http.Client
    cveRepo repository.MongoDBCVERepository
    logger  zerolog.Logger
    enabled bool // feature flag: CNNVD may be unreliable
}

func NewCNNVDFetcher(cveRepo repository.MongoDBCVERepository, log zerolog.Logger) *CNNVDFetcher {
    return &CNNVDFetcher{
        baseURL: cnnvdBaseURL,
        client:  &http.Client{Timeout: 30 * time.Second},
        cveRepo: cveRepo,
        logger:  log.With().Str("fetcher", "CNNVD").Logger(),
        enabled: true,
    }
}

func (f *CNNVDFetcher) Name() string { return string(SourceCNNVD) }

func (f *CNNVDFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
    if !f.enabled {
        f.logger.Info().Msg("CNNVD fetcher disabled")
        return 0, nil
    }

    reqBody := `{"pageIndex":1,"pageSize":100,"language":"en"}`
    req, err := http.NewRequestWithContext(ctx, http.MethodPost,
        f.baseURL+"/web/vulnerability/querylist.tag",
        strings.NewReader(reqBody))
    if err != nil {
        return 0, fmt.Errorf("cnnvd: build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := f.client.Do(req)
    if err != nil {
        // CNNVD may be geographically restricted — log warning, non-fatal
        f.logger.Warn().Err(err).Msg("CNNVD: request failed (may be geo-blocked)")
        return 0, nil
    }
    defer resp.Body.Close()

    var result struct {
        Data struct {
            Records []struct {
                CVEID   string `json:"cveNumber"`
                Summary string `json:"vulDesc"`
                Level   string `json:"hazardLevel"`
                PubDate string `json:"publishTime"`
            } `json:"records"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        f.logger.Warn().Err(err).Msg("CNNVD: decode failed")
        return 0, nil
    }

    cves := make([]*entity.CVE, 0)
    for _, r := range result.Data.Records {
        if r.CVEID == "" {
            continue
        }
        cve := &entity.CVE{
            ID:          r.CVEID,
            DataSource:  string(SourceCNNVD),
            Description: r.Summary,
        }
        if t, err := time.Parse("2006-01-02", r.PubDate); err == nil {
            cve.Published = t
        }
        cves = append(cves, cve)
    }

    if len(cves) == 0 {
        return 0, nil
    }

    count, err := f.cveRepo.UpsertBatch(ctx, cves)
    if err != nil {
        return 0, fmt.Errorf("cnnvd: upsert: %w", err)
    }
    f.logger.Info().Int("upserted", count).Msg("CNNVD sync done")
    return count, nil
}
```

### scheduler.go — ADD new source schedules

Tìm function setup scheduler và thêm schedules cho nguồn mới. Cũng cần wire fetchers vào registry.

```go
// Thêm vào schedules map (tìm section hiện tại và ADD các entry mới):

// Trong function SetupScheduler hoặc tương đương:
// ADD các source mới vào map:
schedules := map[fetcher.SourceName]string{
    // --- EXISTING (giữ nguyên) ---
    fetcher.SourceNVD:    "0 0 */2 * * *",  // every 2h
    fetcher.SourceEPSS:   "0 0 3 * * *",    // daily 3am
    fetcher.SourceNVDCPE: "0 0 4 * * 0",    // Sunday 4am
    fetcher.SourceCAPEC:  "0 0 5 * * 0",    // Sunday 5am
    fetcher.SourceCWE:    "0 0 5 * * 0",    // Sunday 5am

    // --- NEW from CR-GCV-001 ---
    fetcher.SourceCIRCL:     "0 0 */6 * * *",  // every 6h
    fetcher.SourceJVN:       "0 0 * * * *",    // every 1h
    fetcher.SourceExploitDB: "0 0 2 * * *",    // daily 2am
    fetcher.SourceCVEOrg:    "0 0 */12 * * *", // every 12h
    fetcher.SourceCNNVD:     "0 0 */12 * * *", // every 12h (beta)
}

// Admin HTTP trigger endpoint cũng cần đăng ký fetchers vào Registry:
// Trong wire/main.go, sau khi tạo từng fetcher:
// registry.Register(fetcher.SourceCIRCL,     circlFetcher)
// registry.Register(fetcher.SourceJVN,       jvnFetcher)
// registry.Register(fetcher.SourceExploitDB, exploitDBFetcher)
// registry.Register(fetcher.SourceCVEOrg,    cveOrgFetcher)
// registry.Register(fetcher.SourceCNNVD,     cnnvdFetcher)
```

**Lưu ý khi sửa scheduler.go**: Đọc file hiện tại trước, chỉ thêm entries mới vào đúng map/slice, không xóa entries cũ.

## Acceptance Criteria

- [x] `CNNVDFetcher.Name()` returns `"CNNVD"`
- [x] CNNVD request fail (geo-block) → log warning, return `(0, nil)` không fail sync
- [x] Scheduler có schedules cho: CIRCL (6h), JVN (1h), ExploitDB (daily 2am), CVE.org (12h), CNNVD (12h)
- [x] Các schedules hiện có (NVD, EPSS, CPE, CAPEC, CWE) không bị thay đổi
- [x] `go build ./...` pass không lỗi
