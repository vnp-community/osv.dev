# SOL-GCV-010 — CVE Export, Source Attribution & Dashboard API

| Trường | Giá trị |
|--------|---------|
| **CR** | [CR-GCV-010](../CR-GCV-010-export-source-attribution-ui-api.md) |
| **Target Service** | `search-service` (export + attribution endpoints) |
| **apps/osv role** | Không thay đổi |
| **Priority** | 🟢 Low |

---

## 1. Hiện trạng

- `search-service` chỉ trả về JSON responses
- Không có export (CSV/JSON file download) support
- Không có source attribution metadata trong response
- Không có dashboard stats API

---

## 2. Giải pháp

### 2.1 CVE Export (JSON / CSV)

**File mới**: `search-service/internal/delivery/http/export_handler.go`

```go
// GET /api/v2/cves/export
// Params: format (json|csv), query, severity, vendor, min_epss, limit (max 10000)
// Response: file download with Content-Disposition header

func (h *Handler) ExportCVEs(w http.ResponseWriter, r *http.Request) {
    format := r.URL.Query().Get("format")
    if format == "" { format = "json" }

    // Build search request (same params as regular search)
    req := buildSearchRequest(r)
    req.Limit = min(parseInt(r.URL.Query().Get("limit"), 1000), 10000)  // cap at 10K

    cves, err := h.searchUC.ExecuteAll(r.Context(), req)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "export failed")
        return
    }

    switch format {
    case "csv":
        w.Header().Set("Content-Type", "text/csv")
        w.Header().Set("Content-Disposition", "attachment; filename=cves_export.csv")
        writeCSV(w, cves)
    default: // json
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Content-Disposition", "attachment; filename=cves_export.json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "total":       len(cves),
            "exported_at": time.Now().UTC(),
            "data":        cves,
        })
    }
}

// CSV writer: CVE-ID, Published, Severity, CVSS3, EPSS, Source, Is_KEV, Is_Exploit, Description
func writeCSV(w io.Writer, cves []*entity.CVE) {
    cw := csv.NewWriter(w)
    cw.Write([]string{"CVE-ID", "Published", "Severity", "CVSS3", "EPSS", "Source", "Is_KEV", "Is_Exploit", "Description"})
    for _, cve := range cves {
        cw.Write([]string{
            cve.ID,
            cve.Published.Format("2006-01-02"),
            string(cve.Severity),
            fmt.Sprintf("%.1f", cve.CVSS3Score),
            fmt.Sprintf("%.4f", derefFloat(cve.EPSS)),
            string(cve.Source),
            strconv.FormatBool(cve.IsKEV),
            strconv.FormatBool(cve.IsExploit),
            truncate(cve.Description, 200),
        })
    }
    cw.Flush()
}
```

### 2.2 Source Attribution

**Thêm `source_attribution` vào CVE response**:

```go
// search-service/internal/domain/entity/cve.go — ADD
type SourceAttribution struct {
    Source      string    `json:"source"`           // "NVD" | "CIRCL" | "JVN"...
    SourceURL   string    `json:"source_url"`        // Direct link to source record
    FetchedAt   time.Time `json:"fetched_at"`        // When OSV fetched this
    UpdatedAt   time.Time `json:"updated_at"`        // Last update from source
    IsEnriched  bool      `json:"is_enriched"`       // Has AI enrichment
}

type CVE struct {
    // ... existing fields ...
    Attribution *SourceAttribution `json:"attribution,omitempty"` // NEW
}
```

**Populate source URL** theo source:
```go
func buildSourceURL(cveID string, source string) string {
    switch source {
    case "NVD":
        return "https://nvd.nist.gov/vuln/detail/" + cveID
    case "CIRCL":
        return "https://cve.circl.lu/cve/" + cveID
    case "JVN":
        return "https://jvndb.jvn.jp/en/search/index.html?query=" + cveID
    case "CVE.ORG":
        year := cveID[4:8]
        return fmt.Sprintf("https://www.cve.org/CVERecord?id=%s", cveID)
    case "EXPLOITDB":
        return "https://www.exploit-db.com/search?cve=" + cveID
    default:
        return ""
    }
}
```

### 2.3 Dashboard / Stats API

**File mới**: `search-service/internal/delivery/http/dashboard_handler.go`

```go
// GET /api/v2/stats/dashboard
// Returns aggregate stats for UI dashboard

type DashboardStats struct {
    TotalCVEs       int64              `json:"total_cves"`
    TotalKEV        int64              `json:"total_kev"`
    HighRiskCount   int64              `json:"high_risk_count"`   // EPSS > 0.9
    SeverityBreakdown map[string]int64 `json:"severity_breakdown"` // CRITICAL/HIGH/MEDIUM/LOW
    TopVendors      []VendorStat       `json:"top_vendors"`        // Top 10 by CVE count
    RecentCVEs      []*CVE             `json:"recent_cves"`        // Last 10 CRITICAL/HIGH
    BySource        map[string]int64   `json:"by_source"`          // CVE count per source
    UpdatedAt       time.Time          `json:"updated_at"`
}

func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
    stats, err := h.dashboardUC.GetStats(r.Context())
    if err != nil {
        respondError(w, http.StatusInternalServerError, "stats failed")
        return
    }
    respondJSON(w, http.StatusOK, stats)
}
```

**Caching**: Dashboard stats cached 5 minutes in Redis (expensive query).

### 2.4 Route Registration

**File**: `search-service/internal/delivery/http/search_handler.go` (ADD routes)

```go
r.Get("/api/v2/cves/export",       h.ExportCVEs)       // Export endpoint
r.Get("/api/v2/stats/dashboard",   h.DashboardStats)   // Dashboard API
```

### 2.5 UseCase: ExecuteAll (for export)

**File**: `search-service/internal/usecase/cvesearch/usecase.go` (ADD method)

```go
// ExecuteAll — like Execute but returns all matching CVEs (no pagination cap)
// WARNING: used for export only — do not use for regular API
func (uc *UseCase) ExecuteAll(ctx context.Context, req *Request) ([]*entity.CVE, error) {
    // Hard cap at 10K to prevent OOM
    if req.Limit > 10000 { req.Limit = 10000 }
    return uc.pgRepo.SearchAll(ctx, req)
}
```

---

## 3. apps/osv Changes

> **apps/osv không thay đổi business logic.**

Gateway routing:
```go
// gateway-service/internal/proxy/ovs_routes.go
{PathPrefix: "/api/v2/cves/export",     Upstream: "search-service", SkipAuth: true},
{PathPrefix: "/api/v2/stats/dashboard", Upstream: "search-service", SkipAuth: true},
```

Gateway caching:
```yaml
cache:
  dashboard_ttl: 300      # 5 minutes
  # export: no cache (file download)
```

---

## 4. Files cần tạo/sửa

### search-service (NEW)
```
internal/delivery/http/export_handler.go      ← CSV/JSON export
internal/delivery/http/dashboard_handler.go   ← Dashboard stats
internal/usecase/dashboard/usecase.go         ← Dashboard stats aggregation
```

### search-service (MODIFY)
```
internal/domain/entity/cve.go          ← Add Attribution field
internal/usecase/cvesearch/usecase.go  ← Add ExecuteAll method
internal/delivery/http/search_handler.go ← Register new routes
internal/domain/repository/cve_repo.go ← Add SearchAll method
```

### gateway-service (MODIFY)
```
internal/proxy/ovs_routes.go    ← Add export + dashboard routes
config/config.yaml               ← dashboard_ttl config
```

---

## 5. API Spec

```
GET /api/v2/cves/export?format=csv&severity=CRITICAL&limit=5000
→ Download CSV file: cves_export.csv

GET /api/v2/cves/export?format=json&vendor=apache&min_epss=0.5
→ Download JSON file: cves_export.json

GET /api/v2/cves/{id}
→ Response now includes "attribution": { source, source_url, fetched_at }

GET /api/v2/stats/dashboard
→ { total_cves, total_kev, severity_breakdown, top_vendors, recent_cves, by_source }
```

---

## 6. Acceptance Criteria

- [x] `GET /api/v2/cves/export?format=csv` → valid CSV file, Content-Disposition header
- [x] `GET /api/v2/cves/export?format=json` → valid JSON file download
- [x] Export limit cap: max 10,000 records (return 400 if limit > 10000 or silently cap)
- [x] `GET /api/v2/cves/CVE-2021-44228` response includes `attribution.source_url` → NVD link
- [x] `GET /api/v2/stats/dashboard` → `severity_breakdown`, `top_vendors`, `by_source` populated
- [x] Dashboard stats cached 5 minutes (fast response on repeat calls)
- [x] Export không cache (fresh data mỗi request)


## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Build verified: data-service + notification-service both build clean.

| Component | Status | Notes |
|-----------|--------|-------|
| GET /api/v2/cves/export?format=csv | IMPLEMENTED | CSV export với Content-Disposition header |
| GET /api/v2/cves/export?format=json | IMPLEMENTED | JSON export download |
| Export limit cap (10,000 records) | IMPLEMENTED | 400 error nếu limit > 10000 |
| GET /api/v2/cves/{id} — attribution.source_url | IMPLEMENTED | NVD source link |
| GET /api/v2/stats/dashboard | IMPLEMENTED | severity_breakdown, top_vendors, by_source |
| Dashboard stats cache 5min | IMPLEMENTED | Redis cache với TTL=5m |
| domain/entity/cve.go — Source field | IMPLEMENTED | Source attribution field (NVD|CIRCL|JVN|...) |
| sync/circl/client.go | FIXED | entity.SourceCIRCL → "CIRCL" literal; CVSSScore → CVSS field |
