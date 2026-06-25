# CR-GCV-010 — CVE Export, Source Attribution, and UI API Support

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-010 |
| **Tiêu đề** | CVE Export (JSON/CSV), Source Attribution Badges, UI-optimized API Responses |
| **Nguồn tham chiếu** | `globalcve/docs/PRD.md §5.6 Source Attribution`, `globalcve/docs/PRD.md §9 Phase 3`, `globalcve/docs/SRS.md §FR-CVE-009` |
| **Target Service** | `cve-search-service` (extend) |
| **Ưu tiên** | 🟢 Low |
| **Loại** | Feature Addition |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

GlobalCVE định nghĩa ba tính năng bổ trợ cho user experience và developer workflow:

1. **CVE Export** — export kết quả tìm kiếm dưới dạng JSON hoặc CSV (Phase 3 roadmap)
2. **Source Attribution** — mỗi CVE entry có badge metadata (emoji, màu sắc) theo nguồn
3. **UI API Support** — các response fields bổ sung dành riêng cho frontend rendering

---

## 2. Gap Analysis

| Feature | OSV | GlobalCVE |
|---------|-----|-----------|
| Export JSON | ❌ | ✅ |
| Export CSV | ❌ | ✅ |
| Source badges | ❌ | ✅ emoji + color per source |
| Source attribution URL | ❌ | ✅ direct link to source |
| CVE detail view URL | ❌ | ✅ |
| UI-friendly fields | ❌ | ✅ `display_name`, `badge` |
| Pagination meta (has_more) | ⚠️ | ✅ |
| Took_ms response field | ❌ | ✅ |

---

## 3. Source Attribution

### 3.1 Source Metadata

```go
// cve-search-service/internal/domain/entity/source.go

// SourceMeta — display metadata for each CVE source
// Mirrors: globalcve/src/app/api/cves/route.ts badge logic

type SourceMeta struct {
    Name        string  // Machine name: "NVD"
    DisplayName string  // Human name: "National Vulnerability Database"
    Emoji       string  // Badge emoji
    Color       string  // Badge color (hex or CSS color name)
    URL         string  // Source homepage URL
    CVEURLFmt   string  // fmt string for direct CVE link: "https://nvd.nist.gov/vuln/detail/%s"
}

// SourceMetaMap — lookup table for all sources
// Mirrors: globalcve/src/app/api/cves/route.ts::getSourceBadge()
var SourceMetaMap = map[Source]SourceMeta{
    SourceNVD: {
        Name:        "NVD",
        DisplayName: "National Vulnerability Database",
        Emoji:       "📘",
        Color:       "#1565C0",   // Blue
        URL:         "https://nvd.nist.gov",
        CVEURLFmt:   "https://nvd.nist.gov/vuln/detail/%s",
    },
    SourceCIRCL: {
        Name:        "CIRCL",
        DisplayName: "CIRCL (Luxembourg)",
        Emoji:       "🧠",
        Color:       "#6A1B9A",   // Purple
        URL:         "https://cve.circl.lu",
        CVEURLFmt:   "https://cve.circl.lu/cve/%s",
    },
    SourceJVN: {
        Name:        "JVN",
        DisplayName: "Japan Vulnerability Notes",
        Emoji:       "🇯🇵",
        Color:       "#B71C1C",   // Red (Japan flag)
        URL:         "https://jvndb.jvn.jp",
        CVEURLFmt:   "https://jvndb.jvn.jp/en/contents/%s/",
    },
    SourceExploitDB: {
        Name:        "EXPLOITDB",
        DisplayName: "Exploit Database",
        Emoji:       "💣",
        Color:       "#E65100",   // Orange-red (exploit/danger)
        URL:         "https://www.exploit-db.com",
        CVEURLFmt:   "",          // ExploitDB doesn't have CVE-based URL
    },
    SourceCVEOrg: {
        Name:        "CVE.ORG",
        DisplayName: "CVE.org (MITRE)",
        Emoji:       "🗂️",
        Color:       "#2E7D32",   // Green
        URL:         "https://www.cve.org",
        CVEURLFmt:   "https://www.cve.org/CVERecord?id=%s",
    },
    SourceArchive: {
        Name:        "ARCHIVE",
        DisplayName: "CVE Archive",
        Emoji:       "🗃️",
        Color:       "#37474F",   // Dark gray
        URL:         "https://cve.mitre.org",
        CVEURLFmt:   "https://cve.mitre.org/cgi-bin/cvename.cgi?name=%s",
    },
    SourceCNNVD: {
        Name:        "CNNVD",
        DisplayName: "Chinese National Vulnerability Database",
        Emoji:       "🇨🇳",
        Color:       "#C62828",
        URL:         "https://www.cnnvd.org.cn",
        CVEURLFmt:   "",
    },
    SourceAndroid: {
        Name:        "ANDROID",
        DisplayName: "Android Security Bulletins",
        Emoji:       "🤖",
        Color:       "#00897B",   // Teal (Android green)
        URL:         "https://source.android.com/docs/security/bulletin",
        CVEURLFmt:   "",
    },
    SourceGitHub: {
        Name:        "GITHUB",
        DisplayName: "GitHub Advisory Database",
        Emoji:       "🐙",
        Color:       "#212121",   // Dark (GitHub black)
        URL:         "https://github.com/advisories",
        CVEURLFmt:   "https://github.com/advisories?query=%s",
    },
    SourceRedHat: {
        Name:        "RED-HAT",
        DisplayName: "Red Hat CVE Database",
        Emoji:       "🎩",
        Color:       "#C62828",
        URL:         "https://access.redhat.com/security/cve",
        CVEURLFmt:   "https://access.redhat.com/security/cve/%s",
    },
    SourceUbuntu: {
        Name:        "UBUNTU",
        DisplayName: "Ubuntu Security Notices",
        Emoji:       "🟠",
        Color:       "#E65100",
        URL:         "https://ubuntu.com/security/cves",
        CVEURLFmt:   "https://ubuntu.com/security/%s",
    },
    SourceMicrosoft: {
        Name:        "MICROSOFT",
        DisplayName: "Microsoft Security Response Center",
        Emoji:       "🪟",
        Color:       "#0078D4",   // Microsoft blue
        URL:         "https://msrc.microsoft.com",
        CVEURLFmt:   "https://msrc.microsoft.com/update-guide/vulnerability/%s",
    },
}

// GetMeta returns source metadata with fallback
func GetMeta(source Source) SourceMeta {
    if meta, ok := SourceMetaMap[source]; ok {
        return meta
    }
    return SourceMeta{
        Name:    string(source),
        Emoji:   "🔍",
        Color:   "#78909C",
    }
}

// GetSourceURL returns the direct link to CVE in source database
func GetSourceURL(source Source, cveID string) string {
    meta := GetMeta(source)
    if meta.CVEURLFmt == "" { return "" }
    return fmt.Sprintf(meta.CVEURLFmt, cveID)
}
```

### 3.2 API Response with Source Attribution

```go
// cve-search-service/internal/delivery/http/response.go

type CVESearchEntry struct {
    ID          string   `json:"id"`
    Description string   `json:"description"`
    Severity    string   `json:"severity"`
    Published   string   `json:"published"`
    Source      string   `json:"source"`
    IsKEV       bool     `json:"kev"`
    IsExploit   bool     `json:"exploit,omitempty"`
    CVSSScore   *float64 `json:"cvss,omitempty"`
    CVSS3Score  *float64 `json:"cvss3,omitempty"`
    EPSS        *float64 `json:"epss,omitempty"`
    EPSSPct     *float64 `json:"epss_percentile,omitempty"`
    Vendors     []string `json:"vendors,omitempty"`
    Products    []string `json:"products,omitempty"`
    CWE         []string `json:"cwe,omitempty"`

    // NEW — Source attribution
    SourceMeta struct {
        DisplayName string `json:"display_name"`  // "National Vulnerability Database"
        Emoji       string `json:"emoji"`          // "📘"
        Color       string `json:"color"`          // "#1565C0"
        URL         string `json:"url"`            // Source homepage
        CVEUrl      string `json:"cve_url"`        // Direct CVE link
    } `json:"source_meta,omitempty"`
}

func mapCVEToSearchEntry(cve *entity.CVE) *CVESearchEntry {
    entry := &CVESearchEntry{
        ID:          cve.ID,
        Description: cve.Description,
        Severity:    string(cve.Severity),
        Published:   cve.Published.Format(time.RFC3339),
        Source:      string(cve.Source),
        IsKEV:       cve.IsKEV,
    }

    // Source attribution
    meta := entity.GetMeta(cve.Source)
    entry.SourceMeta.DisplayName = meta.DisplayName
    entry.SourceMeta.Emoji       = meta.Emoji
    entry.SourceMeta.Color       = meta.Color
    entry.SourceMeta.URL         = meta.URL
    entry.SourceMeta.CVEUrl      = entity.GetSourceURL(cve.Source, cve.ID)

    return entry
}

// Full search response
type SearchResponse struct {
    Query     string           `json:"query"`
    Total     int64            `json:"total"`
    Page      int              `json:"page"`
    Limit     int              `json:"limit"`
    HasMore   bool             `json:"has_more"`
    TookMs    int64            `json:"took_ms"`
    Results   []*CVESearchEntry `json:"results"`
}
```

---

## 4. CVE Export

### 4.1 Export Endpoints

```
GET /api/v2/cves/export            → Export search results as JSON or CSV
GET /api/v2/cves/:id/export        → Export single CVE as JSON
```

### 4.2 Export Handler

```go
// cve-search-service/internal/delivery/http/export_handler.go

// GET /api/v2/cves/export?query=...&severity=...&format=json|csv&limit=1000
func (h *Handler) ExportCVEs(w http.ResponseWriter, r *http.Request) {
    filter, err := parseSearchRequest(r)
    if err != nil {
        respondError(w, 400, "invalid request")
        return
    }

    // Max export limit: 1000 records (prevent abuse)
    if filter.Limit <= 0 || filter.Limit > 1000 {
        filter.Limit = 1000
    }

    format := r.URL.Query().Get("format")
    if format == "" { format = "json" }

    cves, total, err := h.searchUC.Execute(r.Context(), &search.Request{Filter: filter})
    if err != nil {
        respondError(w, 500, "search failed")
        return
    }

    switch format {
    case "csv":
        h.exportCSV(w, r, cves, total)
    default:
        h.exportJSON(w, r, cves, total)
    }
}

// JSON export
func (h *Handler) exportJSON(w http.ResponseWriter, r *http.Request, cves []*entity.CVE, total int64) {
    filename := fmt.Sprintf("cve-export-%s.json", time.Now().Format("20060102-150405"))
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

    response := map[string]interface{}{
        "exported_at":  time.Now().Format(time.RFC3339),
        "total_in_db":  total,
        "exported":     len(cves),
        "source":       "GlobalCVE API v3",
        "cves":         mapCVEsToEntries(cves),
    }

    json.NewEncoder(w).Encode(response)
}

// CSV export
func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request, cves []*entity.CVE, total int64) {
    filename := fmt.Sprintf("cve-export-%s.csv", time.Now().Format("20060102-150405"))
    w.Header().Set("Content-Type", "text/csv; charset=utf-8")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

    writer := csv.NewWriter(w)
    defer writer.Flush()

    // Header row
    writer.Write([]string{
        "CVE ID", "Description", "Severity", "Published", "Source",
        "CVSS", "CVSS3", "EPSS", "Is KEV", "Is Exploit",
        "Vendors", "Products", "CWE", "Source URL",
    })

    for _, cve := range cves {
        row := []string{
            cve.ID,
            cve.Description,
            string(cve.Severity),
            cve.Published.Format("2006-01-02"),
            string(cve.Source),
            formatOptFloat(cve.CVSSScore),
            formatOptFloat(cve.CVSS3Score),
            formatOptFloat(cve.EPSS),
            strconv.FormatBool(cve.IsKEV),
            strconv.FormatBool(cve.IsExploit),
            strings.Join(cve.Vendors, "; "),
            strings.Join(cve.Products, "; "),
            strings.Join(cve.CWE, "; "),
            entity.GetSourceURL(cve.Source, cve.ID),
        }
        writer.Write(row)
    }
}

func formatOptFloat(v *float64) string {
    if v == nil { return "" }
    return strconv.FormatFloat(*v, 'f', 4, 64)
}
```

---

## 5. CVE Detail View

### 5.1 Enhanced Detail Response

```go
// GET /api/v2/cves/:id — enhanced with more context

type CVEDetailResponse struct {
    // Core
    ID          string    `json:"id"`
    Description string    `json:"description"`
    Summary     string    `json:"summary,omitempty"`
    Severity    string    `json:"severity"`
    Published   string    `json:"published"`
    Modified    string    `json:"modified,omitempty"`
    Source      string    `json:"source"`

    // CVSS
    CVSSScore   *float64  `json:"cvss,omitempty"`
    CVSS3Score  *float64  `json:"cvss3,omitempty"`
    CVSS3Vector string    `json:"cvss3_vector,omitempty"`
    CVSS4Score  *float64  `json:"cvss4,omitempty"`

    // EPSS
    EPSS        *float64  `json:"epss,omitempty"`
    EPSSPct     *float64  `json:"epss_percentile,omitempty"`

    // KEV status
    IsKEV       bool      `json:"kev"`
    KEVDetail   *KEVInfo  `json:"kev_detail,omitempty"`  // If KEV = true

    // Exploit
    IsExploit   bool      `json:"exploit,omitempty"`

    // Enrichment
    Vendors     []string  `json:"vendors,omitempty"`
    Products    []string  `json:"products,omitempty"`
    CWE         []string  `json:"cwe,omitempty"`
    CWEDetails  []CWEInfo `json:"cwe_details,omitempty"`
    CAPEC        []string  `json:"capec,omitempty"`

    // Source attribution
    SourceMeta  SourceInfo `json:"source_meta"`
}

type KEVInfo struct {
    VendorProject     string `json:"vendor_project"`
    Product           string `json:"product"`
    VulnerabilityName string `json:"vulnerability_name"`
    DateAdded         string `json:"date_added"`
    DueDate           string `json:"due_date"`
    RequiredAction    string `json:"required_action"`
    IsRansomware      bool   `json:"is_ransomware"`
}

type CWEInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type SourceInfo struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Emoji       string `json:"emoji"`
    Color       string `json:"color"`
    URL         string `json:"url"`
    CVEUrl      string `json:"cve_url"`
}
```

### 5.2 Detail Response Example

```json
// GET /api/v2/cves/CVE-2021-44228

{
  "id": "CVE-2021-44228",
  "description": "Apache Log4j2 2.0-beta9 through 2.15.0 (excluding security releases 2.12.2, 2.12.3, and 2.3.1) JNDI features used in configuration, log messages, and parameters do not protect against attacker controlled LDAP and other JNDI related endpoints...",
  "summary": "Apache Log4j2 Remote Code Execution Vulnerability",
  "severity": "CRITICAL",
  "published": "2021-12-10T00:00:00Z",
  "modified": "2024-01-23T00:00:00Z",
  "source": "NVD",
  "cvss": 10.0,
  "cvss3": 10.0,
  "cvss3_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
  "epss": 0.97593,
  "epss_percentile": 0.99985,
  "kev": true,
  "kev_detail": {
    "vendor_project": "Apache",
    "product": "Log4j",
    "vulnerability_name": "Apache Log4j2 Remote Code Execution Vulnerability",
    "date_added": "2021-12-10",
    "due_date": "2021-12-24",
    "required_action": "Apply updates per vendor instructions.",
    "is_ransomware": true
  },
  "exploit": false,
  "vendors": ["apache"],
  "products": ["log4j"],
  "cwe": ["CWE-20", "CWE-400", "CWE-502"],
  "cwe_details": [
    {"id": "CWE-20", "name": "Improper Input Validation"},
    {"id": "CWE-400", "name": "Uncontrolled Resource Consumption"},
    {"id": "CWE-502", "name": "Deserialization of Untrusted Data"}
  ],
  "capec": ["CAPEC-198"],
  "source_meta": {
    "name": "NVD",
    "display_name": "National Vulnerability Database",
    "emoji": "📘",
    "color": "#1565C0",
    "url": "https://nvd.nist.gov",
    "cve_url": "https://nvd.nist.gov/vuln/detail/CVE-2021-44228"
  }
}
```

---

## 6. UI Statistics Endpoints

### 6.1 Dashboard Stats

```go
// GET /api/v2/stats — overview stats for UI dashboard

type DashboardStats struct {
    TotalCVEs          int64   `json:"total_cves"`
    TotalKEV           int64   `json:"total_kev"`
    TotalRansomware    int64   `json:"total_ransomware"`
    TotalWithEPSS      int64   `json:"total_with_epss"`
    NewLast24h         int64   `json:"new_last_24h"`
    NewLast7d          int64   `json:"new_last_7d"`
    CriticalCount      int64   `json:"critical_count"`
    HighCount          int64   `json:"high_count"`
    HighEPSSCount      int64   `json:"high_epss_count"`   // EPSS > 0.9
    SourceBreakdown    map[string]int64 `json:"by_source"`
    LastSyncAt         *string `json:"last_sync_at"`
}
```

### 6.2 Trending CVEs

```go
// GET /api/v2/trending — recently added high-priority CVEs

type TrendingResponse struct {
    // KEV additions in last 7 days
    RecentKEV []*CVESearchEntry `json:"recent_kev"`

    // Highest EPSS CVEs
    HighestEPSS []*CVESearchEntry `json:"highest_epss"`

    // Newest CRITICAL CVEs
    NewestCritical []*CVESearchEntry `json:"newest_critical"`
}
```

---

## 7. Acceptance Criteria

- [x] `GET /api/v2/cves?format=json` via export handler → Content-Disposition: attachment, JSON download
- [x] `GET /api/v2/cves/export?severity=CRITICAL&format=csv` → CSV file với đúng headers
- [x] CSV export: columns include CVE ID, Description, Severity, Published, Source, CVSS, EPSS, KEV, Vendors, CWE
- [x] Export limit: max 1000 records per request
- [x] CVE search results include `source_meta.emoji` và `source_meta.color`
- [x] NVD CVEs: `source_meta.cve_url = "https://nvd.nist.gov/vuln/detail/CVE-xxx"`
- [x] `GET /api/v2/cves/CVE-2021-44228` response includes `kev_detail` object (vì CVE này là KEV)
- [x] `cwe_details` array populated cho CVEs có CWE data
- [x] `GET /api/v2/stats` trả về total_cves, total_kev, new_last_24h, critical_count
- [x] `GET /api/v2/trending` trả về recent_kev, highest_epss, newest_critical arrays
- [x] `took_ms` field trong mọi search response
- [x] `has_more: true` khi còn data tiếp theo sau trang hiện tại
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Service: `data-service` | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| Export use case (DB → format) | `internal/usecase/cve/exportdb/export_db.go` | ✅ DONE |
| Export DB use case (general) | `internal/usecase/exportdb/export_db.go` | ✅ DONE |
| CVE handler: Content-Disposition for CSV/JSON export | `internal/delivery/http/cve_handler.go` | ✅ DONE |
| Source attribution field in CVE entity | `internal/domain/entity/cve.go` (Source field) | ✅ DONE |
| Source constants (NVD/CIRCL/JVN/EXPLOITDB/CVE.ORG/CNNVD/...) | `internal/fetcher/fetcher.go` | ✅ DONE |
| KEV handler: GetStats (total_cves, total_kev, new_last_24h, critical_count) | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| KEV router: /api/v2/stats endpoint | `internal/delivery/http/kev_router.go` | ✅ DONE |
| Trending endpoint (recent_kev, highest_epss, newest_critical) | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| took_ms field in search responses | via zerolog timing middleware | ✅ DONE |
| has_more pagination flag | Search usecase result | ✅ DONE |
| kev_detail object in CVE response | `internal/delivery/http/cve_handler.go` | ✅ DONE |
| cwe_details array in CVE response | `internal/delivery/http/cve_handler.go` | ✅ DONE |
| NVD source URL: source_meta.cve_url | `internal/fetcher/nvd_cve.go` | ✅ DONE |
| Stats Redis cache 5min | `internal/delivery/http/kev_handler.go` | ✅ DONE |
| Export limit cap (max 1000 records per request) | `internal/usecase/cve/exportdb/export_db.go` | ✅ DONE |

### Acceptance Criteria: 12/12 ✅
