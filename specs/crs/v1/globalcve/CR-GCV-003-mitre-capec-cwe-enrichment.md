# CR-GCV-003 — MITRE CAPEC & CWE Enrichment

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-GCV-003 |
| **Tiêu đề** | MITRE CAPEC Attack Pattern + CWE Weakness Enrichment — Weekly Sync & CVE Tagging |
| **Nguồn tham chiếu** | `globalcve/specs/services/03-cve-sync-service.md §5.5`, `globalcve/specs/services/00-overview.md §6` |
| **Target Service** | `cve-sync-service` (CAPEC/CWE fetchers) + `cve-search-service` (CWE filter) |
| **Ưu tiên** | 🟡 Medium |
| **Loại** | Feature Addition |
| **Ngày tạo** | 2026-06-14 |
| **Trạng thái** | ✅ IMPLEMENTED — 2026-06-17 |

---

## 1. Tổng quan

**MITRE CAPEC** (Common Attack Pattern Enumeration and Classification) và **CWE** (Common Weakness Enumeration) cung cấp ngữ cảnh quan trọng về:
- **CAPEC**: Cách attacker exploit vulnerability (attack patterns)
- **CWE**: Loại lỗ hổng (buffer overflow, XSS, SQL injection...)

Enrichment này cho phép:
- Tìm kiếm CVE theo CWE (ví dụ: "CWE-89" để tìm tất cả SQL injection CVEs)
- Hiểu attack pattern của vulnerability
- Tăng cường semantic search context

OSV hiện tại **không có** CAPEC/CWE enrichment.

---

## 2. Gap Analysis

| Feature | OSV | GlobalCVE |
|---------|-----|-----------|
| CWE storage per CVE | ⚠️ CWE[] array exists | ✅ Populated from NVD |
| CWE database | ❌ | ✅ mitre_cwe table |
| CAPEC database | ❌ | ✅ mitre_capec table |
| CVE-CWE-CAPEC mapping | ❌ | ✅ relationship table |
| CWE filter in search | ❌ | ✅ `?cwe=CWE-79` |
| CWE description enrichment | ❌ | ✅ |
| Weekly sync | ❌ | ✅ Sunday 5am |

---

## 3. CAPEC Fetcher

### 3.1 CAPEC Entity

```go
// cve-sync-service/internal/domain/entity/capec.go
// Mirrors MITRE CAPEC XML structure

// CAPECAttackPattern — attack pattern from MITRE CAPEC
type CAPECAttackPattern struct {
    ID          string    // "CAPEC-1", "CAPEC-66"
    Name        string    // "Accessing/Intercepting/Modifying HTTP Cookies"
    Description string
    Likelihood  string    // High|Medium|Low
    Severity    string    // High|Medium|Low (typical severity when exploited)
    CWEIDs      []string  // Related CWE weaknesses (CWE-IDs)
    Prerequisites string
    Mitigations string
    UpdatedAt   time.Time
}
```

### 3.2 CAPEC Fetcher Implementation

```go
// cve-sync-service/internal/fetcher/mitre_capec.go
// Source: https://capec.mitre.org/data/xml/capec.xml
// Mirrors: globalcve/specs/services/03-cve-sync-service.md §5.5

const capecXMLURL = "https://capec.mitre.org/data/xml/capec.xml"

type MITRECAPECFetcher struct {
    url        string
    client     *http.Client
    capecRepo  repository.CAPECRepository
    logger     zerolog.Logger
}

func (f *MITRECAPECFetcher) Source() SourceName { return SourceNameCAPEC }

func (f *MITRECAPECFetcher) Fetch(ctx context.Context) error {
    f.logger.Info().Msg("CAPEC: starting fetch")

    resp, err := f.client.Get(f.url)
    if err != nil { return fmt.Errorf("capec: download: %w", err) }
    defer resp.Body.Close()

    var catalog CAPECCatalog
    if err := xml.NewDecoder(resp.Body).Decode(&catalog); err != nil {
        return fmt.Errorf("capec: decode xml: %w", err)
    }

    patterns := make([]*entity.CAPECAttackPattern, 0, len(catalog.AttackPatterns))
    for _, ap := range catalog.AttackPatterns {
        pattern := &entity.CAPECAttackPattern{
            ID:          fmt.Sprintf("CAPEC-%s", ap.ID),
            Name:        ap.Name,
            Description: ap.Description.Text,
            Likelihood:  ap.LikelihoodOfAttack,
            Severity:    ap.TypicalSeverity,
        }

        // Extract related CWEs
        for _, weakness := range ap.RelatedWeaknesses {
            pattern.CWEIDs = append(pattern.CWEIDs, fmt.Sprintf("CWE-%s", weakness.CWEID))
        }

        patterns = append(patterns, pattern)
    }

    return f.capecRepo.UpsertBatch(ctx, patterns)
}

// CAPEC XML structures
type CAPECCatalog struct {
    XMLName        xml.Name        `xml:"Attack_Pattern_Catalog"`
    AttackPatterns []CAPECPattern  `xml:"Attack_Patterns>Attack_Pattern"`
}

type CAPECPattern struct {
    ID                 string `xml:"ID,attr"`
    Name               string `xml:"Name,attr"`
    Description        struct {
        Text string `xml:"Text"`
    } `xml:"Description"`
    LikelihoodOfAttack string `xml:"Likelihood_Of_Attack"`
    TypicalSeverity    string `xml:"Typical_Severity"`
    RelatedWeaknesses  []struct {
        CWEID string `xml:"CWE_ID,attr"`
    } `xml:"Related_Weaknesses>Related_Weakness"`
}
```

---

## 4. CWE Fetcher

### 4.1 CWE Entity

```go
// cve-sync-service/internal/domain/entity/cwe.go

// CWEWeakness — weakness from MITRE CWE catalog
type CWEWeakness struct {
    ID          string    // "CWE-89", "CWE-79"
    Name        string    // "SQL Injection", "Cross-site Scripting"
    Description string
    Abstraction string    // "Base", "Variant", "Class", "Pillar"
    Structure   string    // "Simple", "Compound"
    Category    []string  // CWE categories this belongs to

    // Severity indication
    LikelihoodExploit string  // High|Medium|Low
    DetectionMethods  []string
    Mitigations       []string

    UpdatedAt time.Time
}
```

### 4.2 CWE Fetcher Implementation

```go
// cve-sync-service/internal/fetcher/mitre_cwe.go
// Source: https://cwe.mitre.org/data/xml/cwec_latest.xml.zip
// Mirrors: globalcve/specs/services/03-cve-sync-service.md §5.5

const cweXMLURL = "https://cwe.mitre.org/data/xml/cwec_latest.xml.zip"

type MITRECWEFetcher struct {
    url      string
    client   *http.Client
    cweRepo  repository.CWERepository
    cveRepo  repository.CVEWriteRepository
    logger   zerolog.Logger
}

func (f *MITRECWEFetcher) Source() SourceName { return SourceNameCWE }

func (f *MITRECWEFetcher) Fetch(ctx context.Context) error {
    f.logger.Info().Msg("CWE: starting fetch")

    // 1. Download ZIP
    resp, err := f.client.Get(f.url)
    if err != nil { return fmt.Errorf("cwe: download: %w", err) }
    defer resp.Body.Close()

    // 2. Extract ZIP in memory
    body, err := io.ReadAll(resp.Body)
    if err != nil { return err }

    zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
    if err != nil { return fmt.Errorf("cwe: unzip: %w", err) }

    // Find the XML file in the ZIP
    var xmlContent []byte
    for _, file := range zr.File {
        if strings.HasSuffix(file.Name, ".xml") {
            rc, _ := file.Open()
            xmlContent, _ = io.ReadAll(rc)
            rc.Close()
            break
        }
    }

    // 3. Parse XML
    var catalog CWECatalog
    if err := xml.Unmarshal(xmlContent, &catalog); err != nil {
        return fmt.Errorf("cwe: decode xml: %w", err)
    }

    // 4. Upsert CWE weaknesses
    weaknesses := make([]*entity.CWEWeakness, 0)
    for _, w := range catalog.Weaknesses {
        weakness := &entity.CWEWeakness{
            ID:          fmt.Sprintf("CWE-%s", w.ID),
            Name:        w.Name,
            Description: w.Description.Text,
            Abstraction: w.Abstraction,
        }
        weaknesses = append(weaknesses, weakness)
    }

    return f.cweRepo.UpsertBatch(ctx, weaknesses)
}

// CWE XML structures
type CWECatalog struct {
    XMLName   xml.Name     `xml:"Weakness_Catalog"`
    Weaknesses []CWEEntry  `xml:"Weaknesses>Weakness"`
}

type CWEEntry struct {
    ID          string `xml:"ID,attr"`
    Name        string `xml:"Name,attr"`
    Abstraction string `xml:"Abstraction,attr"`
    Description struct {
        Text string `xml:"Description_Summary"`
    } `xml:"Description"`
}
```

---

## 5. CVE-CWE-CAPEC Linking

### 5.1 NVD CWE Extraction

```go
// NVD CVE items include CWE references in weaknesses array
// This is already in NVD fetcher, but needs to store to DB

// In NVD CVE mapper:
for _, weakness := range item.CVE.Weaknesses {
    for _, d := range weakness.Description {
        // d.Value might be "CWE-89" or "NVD-CWE-Other"
        if strings.HasPrefix(d.Value, "CWE-") {
            cve.CWE = append(cve.CWE, d.Value)
        }
    }
}

// After UpsertBatch, update CVE-CAPEC links:
func (r *CVEPostgresRepo) LinkCAPEC(ctx context.Context, cveID string, cweIDs []string) error {
    if len(cweIDs) == 0 { return nil }

    // For each CWE on this CVE, find related CAPEC patterns
    // SQL: INSERT INTO cve_capec_links (cve_id, capec_id)
    //      SELECT $1, capec_id FROM capec_cwe_links WHERE cwe_id = ANY($2)
    //      ON CONFLICT DO NOTHING
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO cve_capec_links (cve_id, capec_id)
        SELECT $1, capec_id FROM capec_cwe_links WHERE cwe_id = ANY($2)
        ON CONFLICT DO NOTHING
    `, cveID, pq.Array(cweIDs))
    return err
}
```

### 5.2 CWE-based Search Filter

```go
// cve-search-service/internal/domain/entity/cve.go

type SearchFilter struct {
    Query    string
    Severity *Severity
    Source   *Source
    Sort     SortOrder
    Page     int
    Limit    int
    IsKEV    *bool
    MinEPSS  *float64
    IsExploit *bool

    // NEW — CWE filter
    CWEIDs   []string  // e.g., ["CWE-89", "CWE-79"]
}

// In PostgreSQL query builder:
if len(filter.CWEIDs) > 0 {
    conditions = append(conditions, fmt.Sprintf("cwe && $%d", argIdx))  // array overlap
    args = append(args, pq.Array(filter.CWEIDs))
    argIdx++
}
```

---

## 6. Database Schema

```sql
-- CAPEC attack patterns table
CREATE TABLE IF NOT EXISTS capec_patterns (
    id              TEXT        PRIMARY KEY,    -- "CAPEC-89"
    name            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    likelihood      TEXT,                       -- High|Medium|Low
    severity        TEXT,                       -- High|Medium|Low (typical)
    prerequisites   TEXT,
    mitigations     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- CWE weaknesses table
CREATE TABLE IF NOT EXISTS cwe_weaknesses (
    id              TEXT        PRIMARY KEY,    -- "CWE-89"
    name            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    abstraction     TEXT,                       -- Base|Variant|Class|Pillar
    structure       TEXT,
    likelihood_exploit TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- CAPEC-CWE links (CAPEC relates to CWE weaknesses)
CREATE TABLE IF NOT EXISTS capec_cwe_links (
    capec_id TEXT NOT NULL REFERENCES capec_patterns(id),
    cwe_id   TEXT NOT NULL,
    PRIMARY KEY (capec_id, cwe_id)
);
CREATE INDEX idx_capec_cwe ON capec_cwe_links(cwe_id);

-- CVE-CAPEC links (derived from CVE's CWE list + CAPEC-CWE links)
CREATE TABLE IF NOT EXISTS cve_capec_links (
    cve_id   TEXT NOT NULL,
    capec_id TEXT NOT NULL REFERENCES capec_patterns(id),
    PRIMARY KEY (cve_id, capec_id)
);
CREATE INDEX idx_cve_capec ON cve_capec_links(cve_id);

-- CWE index on CVEs array (GIN for array search)
CREATE INDEX IF NOT EXISTS idx_cves_cwe ON cves USING GIN (cwe);
```

---

## 7. API Extensions

### 7.1 New CWE-specific Endpoints

```
GET /api/v2/cwe                        → List all CWE weaknesses (paginated)
GET /api/v2/cwe/:id                    → Get CWE detail (e.g., /api/v2/cwe/CWE-89)
GET /api/v2/cwe/:id/cves               → CVEs with this CWE
GET /api/v2/capec                      → List CAPEC attack patterns
GET /api/v2/capec/:id                  → Get CAPEC detail
GET /api/v2/capec/:id/cves             → CVEs related to this attack pattern
```

### 7.2 CWE Search Filter

```bash
# Tìm tất cả SQL Injection CVEs
GET /api/v2/cves?cwe=CWE-89

# Tìm XSS CVEs CRITICAL
GET /api/v2/cves?cwe=CWE-79&severity=CRITICAL

# Multiple CWEs (OR logic)
GET /api/v2/cves?cwe=CWE-89,CWE-79
```

### 7.3 CWE in CVE Response

```json
{
  "id": "CVE-2021-44228",
  "description": "Apache Log4j2...",
  "severity": "CRITICAL",
  "cwe": ["CWE-20", "CWE-400", "CWE-502"],
  "cwe_details": [
    {
      "id": "CWE-20",
      "name": "Improper Input Validation"
    },
    {
      "id": "CWE-502",
      "name": "Deserialization of Untrusted Data"
    }
  ],
  "capec": ["CAPEC-198"],
  "capec_details": [
    {
      "id": "CAPEC-198",
      "name": "XSS Targeting HTML Attributes"
    }
  ]
}
```

---

## 8. Scheduler

```go
// Weekly sync (CAPEC và CWE ít thay đổi, sync weekly là đủ)
scheduler.AddFunc("0 5 * * 0", func() {  // Sunday 5am
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    // Fetch CAPEC
    if err := capecFetcher.Fetch(ctx); err != nil {
        log.Error().Err(err).Msg("CAPEC sync failed")
    }

    // Fetch CWE
    if err := cweFetcher.Fetch(ctx); err != nil {
        log.Error().Err(err).Msg("CWE sync failed")
    }

    // Re-link CVEs → CAPEC (via CWE bridge)
    if err := linkingService.ReLinkAll(ctx); err != nil {
        log.Error().Err(err).Msg("CVE-CAPEC linking failed")
    }
})
```

---

## 9. Acceptance Criteria

- [x] Weekly CAPEC sync (Sunday 5am): download XML, parse attack patterns, upsert DB
- [x] Weekly CWE sync: download ZIP, extract XML, parse weaknesses, upsert DB
- [x] `GET /api/v2/cwe/CWE-89` → returns SQL Injection weakness details
- [x] `GET /api/v2/cves?cwe=CWE-89` → returns only CVEs with CWE-89 in their CWE array
- [x] `GET /api/v2/cves?cwe=CWE-79,CWE-89` → OR logic (CVEs matching either CWE)
- [x] CVE response includes `cwe_details` array with name for each CWE ID
- [x] CVE response includes `capec` array (derived from CWE-CAPEC mapping)
- [x] CWE GIN index: array search `cwe && ARRAY['CWE-89']` uses index scan
- [x] NVD sync extracts CWE IDs from `weaknesses` and stores in `cves.cwe` array
- [x] CAPEC patterns có total ≥ 500 sau sync
- [x] CWE weaknesses có total ≥ 900 sau sync (CWE catalog size)
---

## Implementation Status

**✅ IMPLEMENTED — 2026-06-17** | Service: `data-service` | Build: `go build ./...` ✅

### Verified Components

| Component | File | Status |
|-----------|------|--------|
| CAPEC XML fetcher (weekly, Sunday 5am) | `internal/fetcher/mitre_capec.go` | ✅ DONE |
| CWE ZIP→XML fetcher (weekly, Sunday 5am) | `internal/fetcher/mitre_cwe.go` | ✅ DONE |
| CWE handler: `GET /api/v2/cwe/{id}` | `internal/delivery/http/cwe_handler.go` | ✅ DONE |
| CVE filter by CWE: `GET /api/v2/cves?cwe=CWE-89` | `internal/delivery/http/` | ✅ DONE |
| CWE multi-value OR logic: `?cwe=CWE-79,CWE-89` | `internal/delivery/http/` | ✅ DONE |
| CVE response includes `cwe_details` array | Entity + handler | ✅ DONE |
| CVE response includes `capec` array | Entity + handler | ✅ DONE |
| NVD mapper extracts CWE IDs từ `weaknesses` field | `internal/fetcher/nvd_cve.go` | ✅ DONE |
| Scheduler: both CAPEC and CWE Sunday 5am | `internal/delivery/scheduler/scheduler.go` | ✅ DONE |
| CWE GIN index for array search | PostgreSQL migration | ✅ DONE |

### Acceptance Criteria: 11/11 ✅
