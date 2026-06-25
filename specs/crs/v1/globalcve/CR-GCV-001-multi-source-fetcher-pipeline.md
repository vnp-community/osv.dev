# CR-GCV-001 — Multi-Source CVE Aggregation: Extended Fetcher Pipeline

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-001 |
| **Tiêu đề** | CVE Sync Service — Extended Multi-Source Fetcher Pipeline (NVD/CIRCL/JVN/ExploitDB/CVE.org/CNNVD/Vendor) |
| **Nguồn tham chiếu** | `globalcve/specs/services/03-cve-sync-service.md`, `globalcve/docs/PRD.md §5.1` |
| **Target Service** | `cve-sync-service` (extend) |
| **Ưu tiên** | 🔴 High |
| **Loại** | Feature Enhancement |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

OSV hiện có `cve-sync-service` với một số fetchers cơ bản. GlobalCVE định nghĩa **full fetcher pipeline** gồm 9+ nguồn chính và 40+ vendor sources, mỗi nguồn với lịch sync riêng. CR này chuẩn hóa và mở rộng toàn bộ fetcher pipeline theo chuẩn GlobalCVE v3.0.

### Vấn đề hiện tại

```
OSV cve-sync-service:
  ✅ NVD CVE fetcher (basic)
  ⚠️ CIRCL fetcher (partial)
  ❌ JVN RSS fetcher
  ❌ ExploitDB CSV fetcher
  ❌ CVE.org GitHub release fetcher
  ❌ CNNVD (Chinese NVD) fetcher
  ❌ Android Security Bulletins fetcher
  ❌ CERT-FR fetcher
  ❌ 40+ Vendor sources (Cisco, VMware, Oracle, Red Hat, Ubuntu...)
  ❌ Fetcher registry (auto-registration pattern)
```

---

## 2. Gap Analysis

| Fetcher | GlobalCVE Status | OSV Status | Gap |
|---------|-----------------|-----------|-----|
| NVD CVE v2 | ✅ Production | ✅ Basic | Extend: page_size=2000, rate limit |
| NVD CPE | ✅ Production | ❌ | Missing |
| CIRCL API | ✅ Production | ⚠️ Partial | Full search + single CVE |
| JVN RSS | ✅ Production | ❌ | Missing |
| ExploitDB CSV | ✅ Production | ❌ | Missing |
| CVE.org GitHub | ✅ Production | ❌ | Missing |
| EPSS CSV.GZ | ✅ Production | ❌ | Missing (→ CR-GCV-002) |
| MITRE CAPEC | ✅ Production | ❌ | Missing (→ CR-GCV-003) |
| MITRE CWE | ✅ Production | ❌ | Missing (→ CR-GCV-003) |
| CNNVD (China) | 🧪 Beta | ❌ | Missing |
| Android Bulletins | 🧪 Beta | ❌ | Missing |
| CERT-FR | 🧪 Beta | ❌ | Missing |
| Cisco advisories | 🧪 Beta | ❌ | Missing |
| Red Hat CVE | 🧪 Beta | ❌ | Missing |
| Ubuntu USN | 🧪 Beta | ❌ | Missing |

---

## 3. Fetcher Architecture

### 3.1 Fetcher Interface (từ GlobalCVE spec)

```go
// cve-sync-service/internal/fetcher/fetcher.go

// Fetcher — interface mà tất cả data source adapters phải implement
type Fetcher interface {
    Fetch(ctx context.Context) error
    Source() SourceName
}

// IncrementalFetcher — fetchers hỗ trợ sync incremental (chỉ dữ liệu mới)
type IncrementalFetcher interface {
    Fetcher
    FetchSince(ctx context.Context, since time.Time) error
}

type FetchOptions struct {
    Since *time.Time // nil = full sync
    Force bool       // bypass idempotency check
}

// FetchResult — thống kê sau khi fetch
type FetchResult struct {
    Source    SourceName
    Fetched   int
    Upserted  int
    Skipped   int
    Errors    int
    Duration  time.Duration
    StartedAt time.Time
}
```

### 3.2 Fetcher Registry

```go
// cve-sync-service/internal/fetcher/registry.go
// Auto-registration pattern — tất cả fetchers tự đăng ký vào registry

type Registry struct {
    fetchers map[SourceName]Fetcher
    mu       sync.RWMutex
}

func (r *Registry) Register(f Fetcher) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.fetchers[f.Source()] = f
}

func (r *Registry) Get(source SourceName) (Fetcher, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    f, ok := r.fetchers[source]
    return f, ok
}

func (r *Registry) All() []Fetcher {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Fetcher, 0, len(r.fetchers))
    for _, f := range r.fetchers {
        result = append(result, f)
    }
    return result
}
```

---

## 4. Core Fetchers Implementation

### 4.1 NVD CVE Fetcher (Enhanced)

```go
// cve-sync-service/internal/fetcher/nvd_cve.go
// Source: https://api.nvd.nist.gov/rest/json/cves/2.0

type NVDCVEFetcher struct {
    apiKey   string
    client   *http.Client
    cveRepo  repository.CVEWriteRepository
    pageSize int  // default: 2000 (max allowed)
    logger   zerolog.Logger
}

func (f *NVDCVEFetcher) Source() SourceName { return SourceNameNVD }

// FetchPage — download một page từ NVD
// GET /rest/json/cves/2.0?resultsPerPage=2000&startIndex=N&lastModStartDate=...
func (f *NVDCVEFetcher) FetchPage(ctx context.Context, startIndex int, since *time.Time) (*NVDResponse, error) {
    url := fmt.Sprintf("%s?resultsPerPage=%d&startIndex=%d",
        nvdBaseURL, f.pageSize, startIndex)

    if since != nil {
        url += fmt.Sprintf("&lastModStartDate=%s&lastModEndDate=%s",
            since.Format(time.RFC3339),
            time.Now().Format(time.RFC3339))
    }

    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    if f.apiKey != "" {
        req.Header.Set("apiKey", f.apiKey)
    }

    resp, err := f.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("NVD returned %d", resp.StatusCode)
    }

    var result NVDResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

// Fetch — full sync: loop tất cả pages
func (f *NVDCVEFetcher) Fetch(ctx context.Context) error {
    page, err := f.FetchPage(ctx, 0, nil)
    if err != nil { return err }

    total := page.TotalResults
    f.processPage(ctx, page)

    for startIndex := f.pageSize; startIndex < total; startIndex += f.pageSize {
        select {
        case <-ctx.Done(): return ctx.Err()
        default:
        }

        page, err = f.FetchPage(ctx, startIndex, nil)
        if err != nil {
            f.logger.Error().Err(err).Int("startIndex", startIndex).Msg("NVD page fetch failed, skipping")
            continue
        }

        f.processPage(ctx, page)

        // Rate limit: 100ms with API key, 6s without
        delay := 6 * time.Second
        if f.apiKey != "" {
            delay = 100 * time.Millisecond
        }
        time.Sleep(delay)
    }
    return nil
}

// FetchSince — incremental sync (recent changes)
func (f *NVDCVEFetcher) FetchSince(ctx context.Context, since time.Time) error {
    page, err := f.FetchPage(ctx, 0, &since)
    if err != nil { return err }
    return f.processPage(ctx, page)
}

// processPage — convert NVD items → CVE entities → upsert batch
func (f *NVDCVEFetcher) processPage(ctx context.Context, page *NVDResponse) error {
    cves := make([]*entity.CVE, 0, len(page.Vulnerabilities))
    for _, item := range page.Vulnerabilities {
        cve := f.mapToEntity(item)
        cves = append(cves, cve)
    }
    _, _, err := f.cveRepo.UpsertBatch(ctx, cves)
    return err
}

func (f *NVDCVEFetcher) mapToEntity(item NVDItem) *entity.CVE {
    cve := &entity.CVE{
        ID:     item.CVE.ID,
        Source: entity.SourceNVD,
    }

    // Description (first English desc)
    for _, d := range item.CVE.Descriptions {
        if d.Lang == "en" {
            cve.Description = d.Value
            break
        }
    }

    // Published
    if t, err := time.Parse(time.RFC3339, item.CVE.Published); err == nil {
        cve.Published = t
    }

    // CVSS v3.1
    if len(item.CVE.Metrics.CVSSMetricV31) > 0 {
        m := item.CVE.Metrics.CVSSMetricV31[0]
        score := m.CVSSData.BaseScore
        cve.CVSS3Score = &score
        cve.CVSS3Vector = m.CVSSData.VectorString
        cve.Severity = entity.SeverityFromCVSS3(score)
    }

    // CWE
    for _, w := range item.CVE.Weaknesses {
        for _, d := range w.Description {
            cve.CWE = append(cve.CWE, d.Value)
        }
    }

    // Vendors/Products (from CPE match)
    // ...

    return cve
}
```

### 4.2 CIRCL Fetcher

```go
// cve-sync-service/internal/fetcher/circl.go
// Source: https://cve.circl.lu/api

type CIRCLFetcher struct {
    baseURL string
    client  *http.Client
    cveRepo repository.CVEWriteRepository
    logger  zerolog.Logger
}

func (f *CIRCLFetcher) Source() SourceName { return SourceNameCIRCL }

// CIRCL hỗ trợ query theo keyword hoặc exact CVE ID
// GET /api/search/{keyword}   → returns array of CVEs
// GET /api/cve/{cve-id}       → returns single CVE object

func (f *CIRCLFetcher) FetchRecent(ctx context.Context) ([]*entity.CVE, error) {
    // CIRCL không có "recent" endpoint → fetch last 7 days của NVD published dates
    // hoặc query các từ khóa phổ biến
    return f.FetchByKeyword(ctx, "2026")  // current year
}

func (f *CIRCLFetcher) FetchByKeyword(ctx context.Context, keyword string) ([]*entity.CVE, error) {
    url := fmt.Sprintf("%s/search/%s", f.baseURL, keyword)
    resp, err := f.client.Get(url)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    var rawData interface{}
    json.NewDecoder(resp.Body).Decode(&rawData)

    // CIRCL response có thể là object hoặc array
    var items []map[string]interface{}
    switch v := rawData.(type) {
    case []interface{}:
        for _, item := range v {
            if m, ok := item.(map[string]interface{}); ok {
                items = append(items, m)
            }
        }
    case map[string]interface{}:
        items = []map[string]interface{}{v}
    }

    cves := make([]*entity.CVE, 0, len(items))
    for _, item := range items {
        cve := f.mapToEntity(item)
        if cve != nil {
            cves = append(cves, cve)
        }
    }
    return cves, nil
}

func (f *CIRCLFetcher) mapToEntity(item map[string]interface{}) *entity.CVE {
    id := extractString(item, "cveMetadata.cveId", "id")
    if id == "" { return nil }

    cve := &entity.CVE{
        ID:          id,
        Source:      entity.SourceCIRCL,
        Description: extractString(item, "summary", "containers.cna.descriptions.0.value"),
    }

    if published := extractString(item, "Published"); published != "" {
        if t, err := time.Parse(time.RFC3339, published); err == nil {
            cve.Published = t
        }
    }

    // Use inferSeverity for CIRCL (no standard severity field)
    // Check multiple CVSS fields
    cve.Severity = inferSeverity(item)
    return cve
}
```

### 4.3 JVN RSS Fetcher

```go
// cve-sync-service/internal/fetcher/jvn.go
// Source: https://jvndb.jvn.jp/en/rss/jvndb.rdf

type JVNFetcher struct {
    feedURL string
    client  *http.Client
    cveRepo repository.CVEWriteRepository
    logger  zerolog.Logger
}

func (f *JVNFetcher) Source() SourceName { return SourceNameJVN }

// RSS feed với custom fields: dc:identifier, dc:date, dc:subject
func (f *JVNFetcher) Fetch(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", f.feedURL, nil)
    resp, err := f.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()

    // Parse RSS/RDF feed
    decoder := xml.NewDecoder(resp.Body)
    var feed JVNFeed
    if err := decoder.Decode(&feed); err != nil {
        return fmt.Errorf("jvn: decode rss: %w", err)
    }

    cves := make([]*entity.CVE, 0, len(feed.Items))
    for _, item := range feed.Items {
        // Extract CVE ID from URL: .../JVNDB-2026-001234 or dc:identifier
        cveID := extractCVEFromURL(item.Link)
        if cveID == "" {
            cveID = item.DCIdentifier
        }

        cve := &entity.CVE{
            ID:          cveID,
            Source:      entity.SourceJVN,
            Description: item.Description,
            Link:        item.Link,
        }

        // Date from dc:date or pubDate
        if item.DCDate != "" {
            cve.Published, _ = time.Parse(time.RFC3339, item.DCDate)
        }

        // Severity from dc:subject text
        cve.Severity = inferSeverityFromText(item.DCSubject + " " + item.Description)

        cves = append(cves, cve)
    }

    _, _, err = f.cveRepo.UpsertBatch(ctx, cves)
    return err
}

// Infer severity from text (for JVN and other text-only sources)
// Mirrors: globalcve/src/lib/jvn.ts::inferSeverityFromText()
func inferSeverityFromText(text string) entity.Severity {
    lower := strings.ToLower(text)
    switch {
    case strings.Contains(lower, "critical"):                   return entity.SeverityCritical
    case strings.Contains(lower, "high"):                       return entity.SeverityHigh
    case strings.Contains(lower, "medium") ||
         strings.Contains(lower, "moderate"):                   return entity.SeverityMedium
    case strings.Contains(lower, "low"):                        return entity.SeverityLow
    default:                                                     return entity.SeverityUnknown
    }
}
```

### 4.4 ExploitDB CSV Fetcher

```go
// cve-sync-service/internal/fetcher/exploitdb.go
// Source: https://gitlab.com/exploit-database/exploitdb/-/raw/main/files_exploits.csv
// Format: CSV with columns: id,file,description,date_published,author,type,platform,port,codes

type ExploitDBFetcher struct {
    csvURL  string
    client  *http.Client
    cveRepo repository.CVEWriteRepository
    logger  zerolog.Logger
}

var cveRegex = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)

func (f *ExploitDBFetcher) Source() SourceName { return SourceNameExploitDB }

func (f *ExploitDBFetcher) Fetch(ctx context.Context) error {
    resp, err := f.client.Get(f.csvURL)
    if err != nil { return fmt.Errorf("exploitdb: download csv: %w", err) }
    defer resp.Body.Close()

    // Stream-based parsing (file có thể > 100MB)
    reader := csv.NewReader(resp.Body)
    reader.LazyQuotes = true

    // Skip header
    if _, err := reader.Read(); err != nil { return err }

    var batch []*entity.CVE
    batchSize := 500

    for {
        record, err := reader.Read()
        if err == io.EOF { break }
        if err != nil {
            f.logger.Warn().Err(err).Msg("ExploitDB: skip malformed row")
            continue
        }

        // Extract CVE IDs from description + codes columns
        // description = record[2], codes = record[7]
        text := record[2] + " " + record[7]
        cveIDs := cveRegex.FindAllString(text, -1)

        for _, cveID := range cveIDs {
            cveID = strings.ToUpper(cveID)
            severity := inferSeverityFromExploit(record[2])

            cve := &entity.CVE{
                ID:          cveID,
                Source:      entity.SourceExploitDB,
                Description: record[2],
                Severity:    severity,
                IsExploit:   true,
            }

            // Date
            if len(record) > 3 && record[3] != "" {
                if t, err := time.Parse("2006-01-02", record[3]); err == nil {
                    cve.Published = t
                }
            }

            batch = append(batch, cve)
            if len(batch) >= batchSize {
                f.cveRepo.UpsertBatch(ctx, batch)
                batch = batch[:0]
            }
        }
    }

    if len(batch) > 0 {
        f.cveRepo.UpsertBatch(ctx, batch)
    }
    return nil
}

// CVSS heuristic từ exploit description
// Mirrors: globalcve/src/lib/exploitdb.ts
func inferSeverityFromExploit(description string) entity.Severity {
    lower := strings.ToLower(description)
    switch {
    case strings.Contains(lower, "remote code execution") ||
         strings.Contains(lower, "rce") ||
         strings.Contains(lower, "critical") ||
         strings.Contains(lower, "buffer overflow"):
        return entity.SeverityCritical
    case strings.Contains(lower, "denial of service") ||
         strings.Contains(lower, "dos") ||
         strings.Contains(lower, "xss") ||
         strings.Contains(lower, "csrf") ||
         strings.Contains(lower, "bypass"):
        return entity.SeverityMedium
    case strings.Contains(lower, "information disclosure") ||
         strings.Contains(lower, "info leak") ||
         strings.Contains(lower, "minor"):
        return entity.SeverityLow
    default:
        return entity.SeverityUnknown
    }
}
```

### 4.5 CVE.org GitHub Fetcher

```go
// cve-sync-service/internal/fetcher/cveorg.go
// Source: CVEProject/cvelistV5 GitHub releases
// URL: https://github.com/CVEProject/cvelistV5/releases/latest/download/deltaLog.json

type CVEOrgFetcher struct {
    releaseURL string  // deltaLog.json (incremental) or baseVulnerabilities.json.gz (full)
    client     *http.Client
    cveRepo    repository.CVEWriteRepository
    logger     zerolog.Logger
}

func (f *CVEOrgFetcher) Source() SourceName { return SourceNameCVEOrg }

// deltaLog.json contains list of recently changed CVEs
// Format: {"changed": ["CVE-2026-12345.json", ...], "new": [...], "deleted": [...]}
func (f *CVEOrgFetcher) FetchSince(ctx context.Context, since time.Time) error {
    resp, err := f.client.Get(f.releaseURL)
    if err != nil { return err }
    defer resp.Body.Close()

    var deltaLog struct {
        Changed []string `json:"changed"`
        New     []string `json:"new"`
        Deleted []string `json:"deleted"`
    }
    json.NewDecoder(resp.Body).Decode(&deltaLog)

    // Fetch each changed/new CVE individually from GitHub API
    filesToFetch := append(deltaLog.Changed, deltaLog.New...)
    for _, filename := range filesToFetch {
        cveID := strings.TrimSuffix(filename, ".json")
        if !entity.IsValidID(cveID) { continue }

        cveURL := fmt.Sprintf("https://raw.githubusercontent.com/CVEProject/cvelistV5/main/cves/%s/%s/%s.json",
            cveID[4:8], cveID, cveID)

        cve, err := f.fetchCVEDetail(ctx, cveID, cveURL)
        if err != nil {
            f.logger.Warn().Err(err).Str("cve", cveID).Msg("CVE.org: fetch detail failed")
            continue
        }

        f.cveRepo.UpsertBatch(ctx, []*entity.CVE{cve})
    }
    return nil
}

func (f *CVEOrgFetcher) fetchCVEDetail(ctx context.Context, id, url string) (*entity.CVE, error) {
    resp, err := f.client.Get(url)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    var cveData map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&cveData)

    cve := &entity.CVE{
        ID:     id,
        Source: entity.SourceCVEOrg,
    }

    // Extract description from containers.cna.descriptions[0].value
    if containers, ok := cveData["containers"].(map[string]interface{}); ok {
        if cna, ok := containers["cna"].(map[string]interface{}); ok {
            if descs, ok := cna["descriptions"].([]interface{}); ok && len(descs) > 0 {
                if d, ok := descs[0].(map[string]interface{}); ok {
                    cve.Description = d["value"].(string)
                }
            }
        }
    }

    cve.Severity = inferSeverity(cveData)
    return cve, nil
}
```

### 4.6 CNNVD Fetcher (Chinese NVD)

```go
// cve-sync-service/internal/fetcher/cnnvd.go
// Source: cnnvd.org.cn (Chinese National Vulnerability Database)
// 🧪 Beta — testing branch

type CNNVDFetcher struct {
    baseURL string
    client  *http.Client
    cveRepo repository.CVEWriteRepository
    logger  zerolog.Logger
}

func (f *CNNVDFetcher) Source() SourceName { return SourceNameCNNVD }

// CNNVD API (unofficial, may need scraping or third-party mirror)
// https://www.cnnvd.org.cn/web/vulnerability/querylist.tag
// Returns vulnerabilities with CNVD ID + CVE ID cross-reference

func (f *CNNVDFetcher) Fetch(ctx context.Context) error {
    // Request recent vulnerabilities
    reqBody := `{"pageIndex":1,"pageSize":100,"language":"en"}`
    req, _ := http.NewRequestWithContext(ctx, "POST", f.baseURL+"/web/vulnerability/querylist.tag",
        strings.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")

    resp, err := f.client.Do(req)
    if err != nil { return fmt.Errorf("cnnvd: query failed: %w", err) }
    defer resp.Body.Close()

    var result struct {
        Data struct {
            Records []struct {
                CVEID   string `json:"cveNumber"`
                Name    string `json:"cnvdNumber"`
                Summary string `json:"vulDesc"`
                Level   string `json:"hazardLevel"`  // 高(High), 中(Medium), 低(Low)
                PubDate string `json:"publishTime"`
            } `json:"records"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    cves := make([]*entity.CVE, 0)
    for _, r := range result.Data.Records {
        if r.CVEID == "" { continue }

        cve := &entity.CVE{
            ID:          r.CVEID,
            Source:      entity.SourceCNNVD,
            Description: r.Summary,
            Severity:    mapCNNVDLevel(r.Level),
        }
        if t, err := time.Parse("2006-01-02", r.PubDate); err == nil {
            cve.Published = t
        }
        cves = append(cves, cve)
    }

    _, _, err = f.cveRepo.UpsertBatch(ctx, cves)
    return err
}

func mapCNNVDLevel(level string) entity.Severity {
    switch {
    case strings.Contains(level, "超危") || strings.Contains(level, "critical"): return entity.SeverityCritical
    case strings.Contains(level, "高") || strings.Contains(level, "high"):       return entity.SeverityHigh
    case strings.Contains(level, "中") || strings.Contains(level, "medium"):     return entity.SeverityMedium
    case strings.Contains(level, "低") || strings.Contains(level, "low"):        return entity.SeverityLow
    default:                                                                       return entity.SeverityUnknown
    }
}
```

---

## 5. CVE Entity Extension

```go
// cve-sync-service/internal/domain/entity/cve.go
// Thêm các source mới + fields từ GlobalCVE

type Source string
const (
    SourceNVD       Source = "NVD"
    SourceCIRCL     Source = "CIRCL"
    SourceJVN       Source = "JVN"
    SourceExploitDB Source = "EXPLOITDB"
    SourceCVEOrg    Source = "CVE.ORG"
    SourceArchive   Source = "ARCHIVE"
    // NEW — from GlobalCVE
    SourceCNNVD     Source = "CNNVD"      // Chinese NVD
    SourceAndroid   Source = "ANDROID"    // Android Security Bulletins
    SourceCERTFR    Source = "CERT-FR"    // French CERT
    SourceCisco     Source = "CISCO"      // Cisco Security Advisories
    SourceRedHat    Source = "RED-HAT"    // Red Hat CVE Database
    SourceUbuntu    Source = "UBUNTU"     // Ubuntu Security Notices
    SourceOracle    Source = "ORACLE"     // Oracle Critical Patch Update
    SourceMicrosoft Source = "MICROSOFT"  // Microsoft Security Response Center
    SourceGitHub    Source = "GITHUB"     // GitHub Advisory Database (GHSA)
    SourceVMware    Source = "VMWARE"     // VMware Security Advisories
)

// CVE entity — extended với fields từ GlobalCVE
type CVE struct {
    ID          string
    Description string
    Summary     string  // Short summary (từ NVD)
    Severity    Severity
    Published   time.Time
    Modified    time.Time
    Source      Source

    // CVSS scores
    CVSSScore   *float64  // CVSS v2
    CVSS3Score  *float64  // CVSS v3
    CVSS4Score  *float64  // CVSS v4 (NEW)
    CVSSVector  string
    CVSS3Vector string

    // EPSS (populated by EPSS fetcher → CR-GCV-002)
    EPSS        *float64
    EPSSPct     *float64

    // Enrichment
    IsKEV       bool
    IsExploit   bool  // from ExploitDB
    Vendors     []string
    Products    []string
    CWE         []string  // from MITRE CWE (→ CR-GCV-003)

    // Relations
    Link        string

    // AI (→ CR-GCV-004)
    Embedding   []float32  // pgvector, 1536 dims

    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

---

## 6. Sync Scheduler

```go
// cve-sync-service/internal/delivery/scheduler/scheduler.go
// Mirrors: globalcve/specs/services/03-cve-sync-service.md §7

// Sync schedule per source:
// NVD CVE:   mỗi 2 giờ   → "0 */2 * * *"
// JVN:       mỗi 1 giờ   → "0 * * * *"
// CIRCL:     mỗi 6 giờ   → "0 */6 * * *"
// ExploitDB: mỗi 24 giờ  → "0 2 * * *" (2am)
// CVE.org:   mỗi 12 giờ  → "0 */12 * * *"
// EPSS:      mỗi 24 giờ  → "0 3 * * *" (3am)
// NVD CPE:   mỗi 7 ngày  → "0 4 * * 0" (Sunday 4am)
// CAPEC/CWE: mỗi 7 ngày  → "0 5 * * 0" (Sunday 5am)
// KEV:       mỗi 6 giờ   → "0 */6 * * *" (→ kev-service)
// CNNVD:     mỗi 12 giờ  → "0 */12 * * *"
// Android:   mỗi 24 giờ  → "0 6 * * *"

func SetupScheduler(reg *fetcher.Registry) *cron.Cron {
    c := cron.New(cron.WithSeconds())

    schedules := map[fetcher.SourceName]string{
        fetcher.SourceNameNVD:       "0 0 */2 * * *",
        fetcher.SourceNameJVN:       "0 0 * * * *",
        fetcher.SourceNameCIRCL:     "0 0 */6 * * *",
        fetcher.SourceNameExploitDB: "0 0 2 * * *",
        fetcher.SourceNameCVEOrg:    "0 0 */12 * * *",
        fetcher.SourceNameCNNVD:     "0 0 */12 * * *",
    }

    for source, schedule := range schedules {
        src := source
        c.AddFunc(schedule, func() {
            f, ok := reg.Get(src)
            if !ok { return }

            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
            defer cancel()

            if err := f.Fetch(ctx); err != nil {
                log.Error().Err(err).Str("source", string(src)).Msg("Sync failed")
            }
        })
    }

    return c
}
```

---

## 7. Admin API Endpoints

```go
// Mirrors: globalcve/specs/services/03-cve-sync-service.md §2

GET  /health                           → Health check
GET  /api/v2/sync/status               → Trạng thái sync tất cả sources
GET  /api/v2/sync/status/:source       → Trạng thái sync source cụ thể
POST /api/v2/sync/trigger              → Trigger manual sync tất cả (admin)
POST /api/v2/sync/trigger/:source      → Trigger sync source cụ thể
GET  /api/v2/sync/history              → Lịch sử sync jobs (phân trang)
GET  /api/v2/sync/history/:source      → Lịch sử sync theo source
```

### Status Response

```json
GET /api/v2/sync/status

{
  "sources": [
    {
      "name": "NVD",
      "last_sync": "2026-06-14T02:00:00Z",
      "status": "COMPLETED",
      "synced": 15234,
      "skipped": 892,
      "errors": 0,
      "duration_seconds": 45
    },
    {
      "name": "JVN",
      "last_sync": "2026-06-14T03:00:00Z",
      "status": "COMPLETED",
      "synced": 234,
      "skipped": 12,
      "errors": 0,
      "duration_seconds": 8
    },
    {
      "name": "CIRCL",
      "last_sync": null,
      "status": "PENDING",
      "synced": 0
    }
  ],
  "total_cves": 258341
}
```

---

## 8. Database Schema Extensions

```sql
-- Thêm sources mới vào CHECK constraint
ALTER TABLE cves DROP CONSTRAINT IF EXISTS cves_source_check;
ALTER TABLE cves ADD CONSTRAINT cves_source_check
    CHECK (source IN (
        'NVD', 'CIRCL', 'JVN', 'EXPLOITDB', 'CVE.ORG', 'ARCHIVE',
        -- NEW sources
        'CNNVD', 'ANDROID', 'CERT-FR', 'CISCO', 'RED-HAT',
        'UBUNTU', 'ORACLE', 'MICROSOFT', 'GITHUB', 'VMWARE'
    ));

-- Add is_exploit flag
ALTER TABLE cves ADD COLUMN IF NOT EXISTS is_exploit BOOLEAN DEFAULT FALSE;

-- Add modified timestamp
ALTER TABLE cves ADD COLUMN IF NOT EXISTS modified TIMESTAMPTZ;

-- Add CVSS v4
ALTER TABLE cves ADD COLUMN IF NOT EXISTS cvss4_score NUMERIC(4,1);

-- Add summary (short version)
ALTER TABLE cves ADD COLUMN IF NOT EXISTS summary TEXT DEFAULT '';

-- New index for exploit flag
CREATE INDEX IF NOT EXISTS idx_cves_exploit ON cves(is_exploit) WHERE is_exploit = TRUE;
CREATE INDEX IF NOT EXISTS idx_cves_modified ON cves(modified DESC NULLS LAST);
```

---

## 9. Severity Inference Logic

```go
// pkg/severity/infer.go
// Centralized severity inference (mirrors globalcve/src/app/api/cves/route.ts::inferSeverity)

// SeverityFromCVSS3 — compute severity label from CVSS v3 base score
// Mirrors CVSS v3.1 specification
func SeverityFromCVSS3(score float64) Severity {
    switch {
    case score >= 9.0: return SeverityCritical
    case score >= 7.0: return SeverityHigh
    case score >= 4.0: return SeverityMedium
    case score > 0.0:  return SeverityLow
    default:           return SeverityUnknown
    }
}

// InferFromMap — try multiple CVSS fields in priority order
// Priority: cvss_v31_score > cvss_v3_score > cvss_v2_score > label
func InferFromMap(data map[string]interface{}) Severity {
    // Try CVSS numeric scores
    for _, field := range []string{"cvss", "cvssScore", "cvssv3", "cvssv2", "cvss3", "base_score"} {
        if v, ok := data[field]; ok {
            if score, ok := toFloat64(v); ok && score > 0 {
                return SeverityFromCVSS3(score)
            }
        }
    }

    // Try severity label
    for _, field := range []string{"severity", "level", "risk"} {
        if v, ok := data[field].(string); ok {
            switch strings.ToUpper(v) {
            case "CRITICAL": return SeverityCritical
            case "HIGH":     return SeverityHigh
            case "MEDIUM", "MODERATE": return SeverityMedium
            case "LOW":      return SeverityLow
            }
        }
    }

    return SeverityUnknown
}
```

---

## 10. Acceptance Criteria

- [x] `GET /api/v2/sync/status` trả về status của tất cả 8+ sources
- [x] `POST /api/v2/sync/trigger/NVD` → NVD sync bắt đầu async
- [x] NVD full sync: fetch tất cả pages, upsert với ON CONFLICT DO UPDATE
- [x] NVD rate limit: 100ms với API key, 6000ms không có key
- [x] JVN RSS sync: parse feed, extract CVE IDs từ URLs
- [x] ExploitDB CSV: stream parse, extract CVE IDs bằng regex, mark `is_exploit=true`
- [x] CVE.org: download deltaLog.json, fetch chỉ changed/new CVEs
- [x] CIRCL: search và normalize response (object hoặc array)
- [x] CNNVD: fetch Chinese CVEs, map level → severity (高/中/低)
- [x] Scheduler: mỗi source tự động sync theo đúng schedule
- [x] Graceful degradation: một source fail không ảnh hưởng sources còn lại
- [x] SyncJob record được tạo cho mỗi sync run với stats (synced/skipped/errors)
- [x] Severity inference sử dụng CVSS score trước, label sau, UNKNOWN cuối cùng
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Service: `data-service` | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| Fetcher interface + Registry | `internal/fetcher/fetcher.go`, `registry.go` | ✅ DONE |
| NVD CVE fetcher (page=2000, rate limit) | `internal/fetcher/nvd_cve.go` | ✅ DONE |
| NVD CPE fetcher (weekly sync) | `internal/fetcher/nvd_cpe.go` | ✅ DONE |
| CIRCL fetcher (search + single CVE) | `internal/fetcher/circl.go` | ✅ DONE |
| JVN RSS fetcher (hourly) | `internal/fetcher/jvn.go` | ✅ DONE |
| ExploitDB CSV fetcher (stream parse) | `internal/fetcher/exploitdb.go` | ✅ DONE |
| CVE.org deltaLog fetcher | `internal/fetcher/cveorg.go` | ✅ DONE |
| CNNVD fetcher (Chinese NVD) | `internal/fetcher/cnnvd.go` | ✅ DONE |
| EPSS fetcher | `internal/fetcher/epss.go` | ✅ DONE |
| CAPEC/CWE fetchers | `internal/fetcher/mitre_capec.go`, `mitre_cwe.go` | ✅ DONE |
| VIA4 fetcher | `internal/fetcher/via4.go` | ✅ DONE |
| Publisher hook (NATS) | `internal/fetcher/publisher_hook.go` | ✅ DONE |
| Redis CPE cache | `internal/fetcher/redis_cpe_cache.go` | ✅ DONE |
| Cron scheduler (all sources) | `internal/delivery/scheduler/scheduler.go` | ✅ DONE |
| CVE entity extensions (Source, IsKEV, IsExploit, EPSS) | `internal/domain/entity/cve.go` | ✅ DONE |
| Source constants (NVD/CIRCL/JVN/EXPLOITDB/CVE.ORG/CNNVD/ANDROID/CERT-FR) | `internal/fetcher/fetcher.go` | ✅ DONE |
| SyncAll use case | `internal/usecase/syncall/sync_all.go` | ✅ DONE |
| Sync source use case | `internal/usecase/syncsource/sync_source.go` | ✅ DONE |
| GET /api/v2/sync/status | `internal/delivery/http/` | ✅ DONE |

### Acceptance Criteria: 13/13 ✅
