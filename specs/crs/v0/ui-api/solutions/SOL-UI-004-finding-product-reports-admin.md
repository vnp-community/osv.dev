# SOL-UI-004 — Finding, Product, Reports & Admin Schema Extensions (CR-UI-005, 007, 009, 010)

**Giải pháp cho:** CR-UI-005, CR-UI-007, CR-UI-009, CR-UI-010  
**Ngày tạo:** 2026-06-16  
**Trạng thái:** ✅ Implemented — *Backend tasks BE-001 → BE-022 hoàn tất (2026-06-17)*  
**Services cần thay đổi:**
- `services/finding-service` (:8085) — finding schema + product grades + reports
- `services/notification-service` (:8087) — webhooks
- `services/audit-service` (:8090) — audit log HTTP
- `services/jira-service` (:8088) — JIRA config HTTP  
**Phụ thuộc kiến trúc:** `specs/01-architecture.md §3.4, §3.7, §3.8, §3.9`, `specs/02-technical-design.md §5, §8, §9`

---

## 1. Finding-Service Extensions (CR-UI-005, CR-UI-007)

### 1.1 Finding Response Schema — Fields còn thiếu

```go
// services/finding-service/internal/adapter/http/dto.go

type FindingListItem struct {
    ID            string   `json:"id"`
    Title         string   `json:"title"`
    Description   string   `json:"description"`
    CveID         *string  `json:"cve_id"`
    Severity      string   `json:"severity"`
    CVSSv3        *float64 `json:"cvss_v3"`
    EPSSScore     *float64 `json:"epss_score"`    // [NEW] — enrich from data-service cache
    IsKEV         bool     `json:"is_kev"`         // [NEW] — lookup from data-service
    Status        string   `json:"status"`         // derived from state machine bool flags
    IsDuplicate   bool     `json:"is_duplicate"`
    DupOfID       *string  `json:"duplicate_finding_id"`
    
    // Hierarchy
    ProductID     string   `json:"product_id"`
    ProductName   string   `json:"product_name"`  // [NEW] JOIN with products table
    EngagementID  string   `json:"engagement_id"`
    TestID        string   `json:"test_id"`
    
    // Asset
    AssetIP       *string  `json:"asset_ip"`
    AssetHostname *string  `json:"asset_hostname"`
    ComponentName *string  `json:"component_name"`
    ComponentVersion *string `json:"component_version"`
    
    // SLA
    SLAExpiry     *string  `json:"sla_expiration_date"`
    SLAStatus     string   `json:"sla_status"`     // [NEW] computed: "ok"|"at_risk"|"breached"
    SLADaysLeft   *int     `json:"sla_days_left"`  // [NEW] computed
    
    // Meta
    CreatedAt     string   `json:"created_at"`
    UpdatedAt     string   `json:"updated_at"`
    MitigatedAt   *string  `json:"mitigated_at"`
    CreatedBy     string   `json:"created_by"`
    AssignedTo    *string  `json:"assigned_to"`
    
    // AI
    AITriageResult *AITriageResult `json:"ai_triage_result"` // [NEW] nullable
    VEXJustification *string      `json:"vex_justification"`
    
    // JIRA
    JiraIssueKey  *string  `json:"jira_issue_key"`  // [NEW] from jira_issues table
    JiraURL       *string  `json:"jira_url"`         // [NEW]
}
```

**SLA Status computation (in-Go, no extra DB query):**
```go
func computeSLAStatus(slaExpiry *time.Time) (string, *int) {
    if slaExpiry == nil {
        return "ok", nil
    }
    daysLeft := int(time.Until(*slaExpiry).Hours() / 24)
    switch {
    case daysLeft < 0:
        abs := -daysLeft
        return "breached", &abs
    case daysLeft <= 7:
        return "at_risk", &daysLeft
    default:
        return "ok", &daysLeft
    }
}
```

**SQL query adjustment** (finding list with product name, jira key):
```sql
-- services/finding-service — list findings with JOIN
SELECT 
    f.id, f.title, f.description, f.cve_id, f.severity,
    f.cvss_v3_score, f.epss_score,                   -- [ADD] epss_score if stored
    f.is_kev,                                          -- [ADD] need to store or join
    f.is_mitigated, f.false_positive, f.out_of_scope, -- state machine flags
    f.risk_accepted, f.is_duplicate, f.duplicate_finding_id,
    f.sla_expiration_date, f.created_at, f.updated_at,
    f.mitigated_at, f.created_by_id, f.assigned_to_id,
    f.component_name, f.component_version,
    -- Product name via JOIN
    p.name AS product_name,                            -- [ADD] JOIN
    -- JIRA key via LEFT JOIN
    ji.jira_key AS jira_issue_key,                    -- [ADD] LEFT JOIN
    CASE WHEN ji.jira_key IS NOT NULL 
         THEN jc.server_url || '/browse/' || ji.jira_key 
    END AS jira_url
FROM findings f
JOIN products p ON p.id = f.product_id
LEFT JOIN jira_issues ji ON ji.finding_id = f.id AND ji.status != 'closed'
LEFT JOIN jira_configs jc ON jc.product_id = f.product_id
WHERE f.product_id = $1
  AND ($2::text IS NULL OR f.severity = $2)
  AND ($3::text IS NULL OR ... status filter ...)
ORDER BY 
    CASE WHEN f.severity = 'Critical' THEN 0
         WHEN f.severity = 'High' THEN 1
         WHEN f.severity = 'Medium' THEN 2
         ELSE 3 END,
    f.sla_expiration_date ASC NULLS LAST
LIMIT $4 OFFSET $5;
```

> **EPSS and KEV enrichment:** `epss_score` và `is_kev` trong findings table — hai lựa chọn:
> 1. **Preferred**: Lưu `epss_score` và `is_kev` vào `findings` table khi import (từ data-service cross-ref). Cần thêm column vào DB schema.
> 2. **Fallback**: Gọi internal data-service `GET /internal/cve/{id}?fields=epss,is_kev` per finding — tốn latency. Không khuyến nghị.

---

### 1.2 Finding List Response Envelope — Thêm Aggregations

```go
// services/finding-service/internal/adapter/http/finding_handler.go

type FindingListResponse struct {
    Findings   []FindingListItem   `json:"findings"`
    Total      int                 `json:"total"`
    Page       int                 `json:"page"`
    PageSize   int                 `json:"page_size"`
    BySeverity map[string]int      `json:"by_severity"`   // [NEW]
    ByStatus   map[string]int      `json:"by_status"`     // [NEW]
    SLAStats   *SLASummary         `json:"sla_stats"`     // [NEW]
}

type SLASummary struct {
    Breached int `json:"breached"`
    AtRisk   int `json:"at_risk"`
    OK       int `json:"ok"`
}
```

**Aggregate query (single SQL for efficiency):**
```sql
-- Count aggregations alongside paginated results using CTEs
WITH filtered AS (
    SELECT f.*, p.name AS product_name
    FROM findings f
    JOIN products p ON p.id = f.product_id
    WHERE ... filter conditions ...
),
paginated AS (
    SELECT * FROM filtered
    ORDER BY ... 
    LIMIT $4 OFFSET $5
),
counts AS (
    SELECT
        COUNT(*) AS total,
        COUNT(*) FILTER (WHERE severity = 'Critical') AS critical,
        COUNT(*) FILTER (WHERE severity = 'High') AS high,
        COUNT(*) FILTER (WHERE severity = 'Medium') AS medium,
        COUNT(*) FILTER (WHERE severity = 'Low') AS low,
        COUNT(*) FILTER (WHERE NOT is_mitigated AND NOT false_positive AND NOT risk_accepted AND NOT out_of_scope AND NOT is_duplicate) AS active,
        COUNT(*) FILTER (WHERE is_mitigated) AS mitigated,
        COUNT(*) FILTER (WHERE false_positive) AS false_positive,
        COUNT(*) FILTER (WHERE risk_accepted) AS risk_accepted,
        COUNT(*) FILTER (WHERE sla_expiration_date < NOW() AND NOT is_mitigated) AS sla_breached,
        COUNT(*) FILTER (WHERE sla_expiration_date BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND NOT is_mitigated) AS sla_at_risk
    FROM filtered
)
SELECT paginated.*, counts.* FROM paginated, counts;
```

---

### 1.3 New Endpoints — Finding-Service

**Bulk Reopen:**
```go
// POST /api/v2/findings/bulk_reopen  (existing path style)
// New alias: POST /api/v1/findings/bulk/reopen
func (h *Handler) BulkReopen(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingIDs []string `json:"finding_ids"`
        Comment    string   `json:"comment"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    userID := r.Header.Get("X-User-ID")
    
    success, failed := 0, []string{}
    for _, id := range req.FindingIDs {
        f, err := h.findingRepo.FindByID(r.Context(), parseUUID(id))
        if err != nil { failed = append(failed, id); continue }
        
        if err := f.Reopen(parseUUID(userID)); err != nil {
            failed = append(failed, id); continue
        }
        h.findingRepo.Update(r.Context(), f)
        
        // NATS publish
        h.nats.PublishJSON("finding.status.changed", map[string]interface{}{
            "finding_id": id, "old_status": f.PreviousState, "new_status": "Active",
        })
        success++
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "success_count": success, "failed_ids": failed,
    })
}
```

**Bulk Assign:**
```go
func (h *Handler) BulkAssign(w http.ResponseWriter, r *http.Request) {
    var req struct {
        FindingIDs []string `json:"finding_ids"`
        AssignedTo string   `json:"assigned_to"` // user email
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    count, err := h.findingRepo.BulkUpdateAssignee(r.Context(), req.FindingIDs, req.AssignedTo)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 200, map[string]interface{}{"success_count": count, "failed_ids": []string{}})
}
```

**Finding Stats:**
```go
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
    productID := r.URL.Query().Get("product_id")
    
    stats, err := h.statsUC.GetFindingStats(r.Context(), productID)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 200, stats)
}
```

**Notes:**
```go
func (h *Handler) AddNote(w http.ResponseWriter, r *http.Request) {
    findingID := r.PathValue("id")
    var req struct{ Content string `json:"content"` }
    json.NewDecoder(r.Body).Decode(&req)
    userID := r.Header.Get("X-User-ID")
    
    note, err := h.noteRepo.Create(r.Context(), &FindingNote{
        FindingID: parseUUID(findingID),
        Content:   req.Content,
        CreatedBy: userID,
    })
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    respondJSON(w, 201, note)
}
```

**Database table for notes** (if not exists):
```sql
-- osv_finding schema
CREATE TABLE finding_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_by  VARCHAR(255) NOT NULL,  -- user email
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_finding_notes_finding_id ON finding_notes(finding_id);
```

---

### 1.4 Product Grades Endpoint (CR-UI-007)

```go
// services/finding-service/internal/adapter/http/product_handler.go

// GET /api/v2/products/grades (new)
// Alias: GET /api/v1/products/grades
func (h *Handler) GetProductGrades(w http.ResponseWriter, r *http.Request) {
    // Get all products with active finding counts
    products, err := h.productRepo.ListWithGrades(r.Context())
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    // Compute overall platform grade
    totalCritical, totalHigh, totalActive := 0, 0, 0
    grades := make([]ProductGradeDTO, len(products))
    for i, p := range products {
        grade := ComputeGrade(p.CriticalCount, p.HighCount, p.TotalActive)
        
        // Trend: compare with last month's grade (stored in grade_snapshots)
        trend := h.computeTrend(r.Context(), p.ID)
        
        grades[i] = ProductGradeDTO{
            ID:            p.ID.String(),
            Name:          p.Name,
            Grade:         grade,
            Score:         gradeToScore(grade),
            CriticalCount: p.CriticalCount,
            HighCount:     p.HighCount,
            Trend:         trend, // "improving" | "worsening" | "stable"
        }
        totalCritical += p.CriticalCount
        totalHigh += p.HighCount
        totalActive += p.TotalActive
    }
    
    overallGrade := ComputeGrade(totalCritical, totalHigh, totalActive)
    overallScore := gradeToScore(overallGrade)
    
    respondJSON(w, 200, map[string]interface{}{
        "products":      grades,
        "overall_grade": overallGrade,
        "overall_score": overallScore,
    })
}
```

**SQL:**
```sql
-- ListWithGrades query
SELECT 
    p.id, p.name,
    COUNT(f.*) FILTER (WHERE f.active AND f.severity = 'Critical' AND NOT f.is_duplicate) AS critical_count,
    COUNT(f.*) FILTER (WHERE f.active AND f.severity = 'High' AND NOT f.is_duplicate) AS high_count,
    COUNT(f.*) FILTER (WHERE f.active AND NOT f.is_duplicate) AS total_active
FROM products p
LEFT JOIN findings f ON f.product_id = p.id
GROUP BY p.id, p.name
ORDER BY critical_count DESC, high_count DESC;
```

**GET /api/v1/products response** — thêm `grade`, `score`, `finding_summary`:
```go
// Modify existing ListProducts handler to include grades
type ProductListItem struct {
    ID            string         `json:"id"`
    Name          string         `json:"name"`
    Description   string         `json:"description"`
    Type          string         `json:"type"`
    Criticality   string         `json:"criticality"`
    Lifecycle     string         `json:"lifecycle"`
    Grade         string         `json:"grade"`          // [NEW] computed
    Score         int            `json:"score"`           // [NEW] computed
    FindingSummary *FindingSummary `json:"finding_summary"` // [NEW]
    SLAConfig     *SLAConfig     `json:"sla_config"`
    Tags          []string       `json:"tags"`
    CreatedAt     string         `json:"created_at"`
}

type FindingSummary struct {
    Critical   int `json:"critical"`
    High       int `json:"high"`
    Medium     int `json:"medium"`
    Low        int `json:"low"`
    TotalActive int `json:"total_active"`
}
```

---

### 1.5 Reports Download Endpoint (CR-UI-009)

```go
// services/finding-service/internal/adapter/http/report_handler.go

// GET /api/v2/reports/{id}/download (new)
func (h *Handler) DownloadReport(w http.ResponseWriter, r *http.Request) {
    reportID := r.PathValue("id")
    
    run, err := h.reportRepo.FindByID(r.Context(), parseUUID(reportID))
    if err != nil {
        respondError(w, 404, "NOT_FOUND", "Report not found")
        return
    }
    
    if run.Status != "completed" {
        respondError(w, 409, "REPORT_NOT_READY", fmt.Sprintf("Report status is %s", run.Status))
        return
    }
    
    // Generate presigned URL from MinIO (5min TTL)
    presignedURL, err := h.minioClient.PresignGetObject(r.Context(), run.ArtifactPath, 5*time.Minute)
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", "Failed to generate download URL")
        return
    }
    
    // Redirect to presigned URL
    http.Redirect(w, r, presignedURL, http.StatusFound)
}
```

```go
// MinIO client helper:
func (m *MinIOClient) PresignGetObject(ctx context.Context, objectPath string, expiry time.Duration) (string, error) {
    // objectPath: "reports/{run_id}/report.pdf"
    u, err := m.client.PresignedGetObject(ctx, m.bucket, objectPath, expiry, nil)
    if err != nil { return "", err }
    return u.String(), nil
}
```

---

## 2. Notification-Service Webhook Extensions (CR-UI-009)

### 2.1 Webhook Test Endpoint

```go
// services/notification-service/internal/adapter/http/webhook_handler.go

// POST /api/v2/notification-webhooks/{id}/test (new)
func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
    webhookID := r.PathValue("id")
    
    wh, err := h.webhookRepo.FindByID(r.Context(), parseUUID(webhookID))
    if err != nil {
        respondError(w, 404, "NOT_FOUND", "Webhook not found")
        return
    }
    
    // Build test payload
    testPayload := map[string]interface{}{
        "event_type": "test",
        "timestamp":  time.Now().Format(time.RFC3339),
        "message":    "This is a test notification from OSV Platform",
        "webhook_id": webhookID,
    }
    
    start := time.Now()
    statusCode, err := h.webhookSender.SendTest(r.Context(), wh, testPayload)
    elapsed := time.Since(start)
    
    deliveryID := "dlv_test_" + uuid.New().String()[:8]
    
    if err != nil {
        respondJSON(w, 200, map[string]interface{}{
            "delivery_id": deliveryID,
            "status": "failed",
            "error": err.Error(),
            "response_time_ms": elapsed.Milliseconds(),
        })
        return
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "delivery_id":       deliveryID,
        "status":            "success",
        "response_code":     statusCode,
        "response_time_ms":  elapsed.Milliseconds(),
    })
}
```

---

## 3. Audit-Service HTTP Endpoint (CR-UI-010)

```go
// services/audit-service/internal/adapter/http/handler.go

// GET /api/v2/audit-log (existing — expose as /api/v1/audit-log too)
func (h *Handler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
    userID     := r.URL.Query().Get("user_id")
    action     := r.URL.Query().Get("action")
    entityType := r.URL.Query().Get("entity_type")
    entityID   := r.URL.Query().Get("entity_id")
    dateFrom   := r.URL.Query().Get("date_from")
    dateTo     := r.URL.Query().Get("date_to")
    page, ps   := parsePagination(r)
    
    events, total, err := h.auditRepo.List(r.Context(), AuditFilter{
        UserID: userID, Action: action,
        EntityType: entityType, EntityID: entityID,
        DateFrom: parseTime(dateFrom), DateTo: parseTime(dateTo),
        Page: page, PageSize: ps,
    })
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "events": mapEvents(events), "total": total,
        "page": page, "page_size": ps,
    })
}
```

---

## 4. JIRA Config HTTP Endpoints (CR-UI-010)

```go
// services/jira-service/internal/adapter/http/config_handler.go

// GET /api/v2/jira-configs/{product_id} (existing path)
// Expose as: GET /api/v1/jira/config
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
    // For UI: return config for currently authenticated user's context
    // Admin: return any product's config
    cfg, err := h.configRepo.FindFirst(r.Context())
    if err != nil {
        respondError(w, 404, "NOT_FOUND", "No JIRA configuration found")
        return
    }
    
    // Mask API token: only show preview
    respondJSON(w, 200, map[string]interface{}{
        "id":                   cfg.ID,
        "jira_url":             cfg.ServerURL,
        "project_key":          cfg.ProjectKey,
        "username":             cfg.Username,
        "api_token_preview":    maskToken(cfg.EncryptedToken), // "ATATT...xyz"
        "is_active":            cfg.Active,
        "webhook_url":          cfg.WebhookURL,
        "created_at":           cfg.CreatedAt,
        "last_sync_at":         cfg.LastSyncAt,
    })
}

// POST /api/v1/jira/config
func (h *Handler) CreateOrUpdateConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        JiraURL    string `json:"jira_url"`
        ProjectKey string `json:"project_key"`
        Username   string `json:"username"`
        APIToken   string `json:"api_token"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Validate connectivity before saving
    if err := h.jiraClient.TestConnection(r.Context(), req.JiraURL, req.Username, req.APIToken); err != nil {
        respondError(w, 400, "JIRA_CONNECTION_FAILED", err.Error())
        return
    }
    
    // Encrypt token: AES-256-GCM (existing pattern in jira-service)
    encToken, err := h.crypto.Encrypt([]byte(req.APIToken))
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", "Encryption failed")
        return
    }
    
    cfg := &JiraConfig{
        ServerURL:      req.JiraURL,
        ProjectKey:     req.ProjectKey,
        Username:       req.Username,
        EncryptedToken: encToken,
        WebhookURL:     h.platformURL + "/api/v1/jira/webhook",
    }
    
    if err := h.configRepo.Upsert(r.Context(), cfg); err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    respondJSON(w, 201, map[string]interface{}{
        "id":          cfg.ID,
        "jira_url":    cfg.ServerURL,
        "project_key": cfg.ProjectKey,
        "webhook_url": cfg.WebhookURL,
        "created_at":  cfg.CreatedAt,
    })
}

// POST /api/v1/jira/config/test
func (h *Handler) TestConfig(w http.ResponseWriter, r *http.Request) {
    cfg, _ := h.configRepo.FindFirst(r.Context())
    
    start := time.Now()
    version, projectFound, err := h.jiraClient.TestConnection(r.Context(), cfg.ServerURL, cfg.Username, cfg.DecryptedToken())
    elapsed := time.Since(start)
    
    if err != nil {
        respondJSON(w, 200, map[string]interface{}{
            "success": false, "error": err.Error(),
            "response_time_ms": elapsed.Milliseconds(),
        })
        return
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "success": true, "jira_version": version,
        "project_found": projectFound,
        "response_time_ms": elapsed.Milliseconds(),
    })
}
```

---

## 5. System Health Endpoint (CR-UI-010 — Gateway BFF)

```go
// apps/osv/internal/gateway/bff/health.go

type ServiceHealthStatus struct {
    Name           string  `json:"name"`
    Status         string  `json:"status"` // healthy|degraded|down
    ResponseTimeMs int64   `json:"response_time_ms"`
    LastCheckedAt  string  `json:"last_checked_at"`
    Version        string  `json:"version"`
    Details        *string `json:"details"`
}

func (bff *HealthBFF) HandleAdminHealth(w http.ResponseWriter, r *http.Request) {
    services := []struct{ name, url string }{
        {"identity-service",      "http://identity-service:8081/health"},
        {"data-service",          "http://data-service:8082/health"},
        {"finding-service",       "http://finding-service:8085/health"},
        {"sla-service",           "http://sla-service:8086/health"},
        {"notification-service",  "http://notification-service:8087/health"},
        {"jira-service",          "http://jira-service:8088/health"},
        {"audit-service",         "http://audit-service:8090/health"},
    }
    
    var (
        results []ServiceHealthStatus
        mu      sync.Mutex
        wg      sync.WaitGroup
    )
    
    for _, svc := range services {
        s := svc
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
            defer cancel()
            
            start := time.Now()
            req, _ := http.NewRequestWithContext(ctx, "GET", s.url, nil)
            resp, err := bff.httpClient.Do(req)
            elapsed := time.Since(start)
            
            status := ServiceHealthStatus{
                Name:           s.name,
                LastCheckedAt:  time.Now().Format(time.RFC3339),
                ResponseTimeMs: elapsed.Milliseconds(),
            }
            
            if err != nil {
                status.Status = "down"
                msg := err.Error()
                status.Details = &msg
            } else {
                defer resp.Body.Close()
                var healthResp struct {
                    Status  string `json:"status"`
                    Version string `json:"version"`
                }
                json.NewDecoder(resp.Body).Decode(&healthResp)
                
                if elapsed > 500*time.Millisecond {
                    status.Status = "degraded"
                    msg := "High response time"
                    status.Details = &msg
                } else if resp.StatusCode == 200 {
                    status.Status = "healthy"
                } else {
                    status.Status = "degraded"
                }
                status.Version = healthResp.Version
            }
            
            mu.Lock()
            results = append(results, status)
            mu.Unlock()
        }()
    }
    
    wg.Wait()
    
    // Determine overall status
    overallStatus := "healthy"
    for _, s := range results {
        if s.Status == "down" { overallStatus = "down"; break }
        if s.Status == "degraded" { overallStatus = "degraded" }
    }
    
    // Check infrastructure (Redis, NATS, Postgres — gateway has direct access)
    natsStatus := bff.checkNATS(r.Context())
    redisStatus := bff.checkRedis(r.Context())
    pgStatus := bff.checkPostgres(r.Context())
    
    respondJSON(w, 200, map[string]interface{}{
        "services":       results,
        "nats":           natsStatus,
        "postgres":       pgStatus,
        "redis":          redisStatus,
        "overall_status": overallStatus,
        "checked_at":     time.Now().Format(time.RFC3339),
    })
}
```

---

## 6. System Settings Endpoint (CR-UI-010 — Gateway)

```go
// apps/osv/internal/gateway/bff/settings.go

// Settings stored in Redis or gateway config (not individual services)
// Simple approach: store in a "platform_settings" PostgreSQL table (gateway DB access)

func (bff *SettingsBFF) GetSettings(w http.ResponseWriter, r *http.Request) {
    settings, err := bff.settingsRepo.Get(r.Context())
    if err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    // Mask sensitive fields
    settings.AI.OpenAIKeyPreview = maskAPIKey(settings.AI.OpenAIAPIKey)
    settings.AI.OpenAIAPIKey = ""  // never return
    settings.Notifications.SMTPPassword = ""
    
    respondJSON(w, 200, settings)
}

func (bff *SettingsBFF) UpdateSettings(w http.ResponseWriter, r *http.Request) {
    var patch map[string]interface{}
    json.NewDecoder(r.Body).Decode(&patch)
    
    if err := bff.settingsRepo.Patch(r.Context(), patch); err != nil {
        respondError(w, 500, "INTERNAL_ERROR", err.Error())
        return
    }
    
    respondJSON(w, 200, map[string]interface{}{"success": true})
}
```

---

## 7. New Gateway Routes

```go
// apps/osv/internal/gateway/router.go additions

// Finding extensions
mux.Handle("GET  /api/v1/findings/stats",       am.Authenticate(p.Forward("finding-service:8085")))
mux.Handle("POST /api/v1/findings/bulk/reopen", am.Authenticate(p.Forward("finding-service:8085")))
mux.Handle("POST /api/v1/findings/bulk/assign", am.Authenticate(p.Forward("finding-service:8085")))
mux.Handle("POST /api/v1/findings/{id}/notes",  am.Authenticate(p.Forward("finding-service:8085")))
mux.Handle("GET  /api/v1/findings/{id}/audit",  am.Authenticate(p.Forward("finding-service:8085")))

// Product grades
mux.Handle("GET  /api/v1/products/grades",      am.Authenticate(p.Forward("finding-service:8085")))
mux.Handle("GET  /api/v1/products/types",       am.Authenticate(p.Forward("finding-service:8085")))

// Reports download
mux.Handle("GET  /api/v1/reports/{id}/download", am.Authenticate(p.Forward("finding-service:8085")))

// Webhook test
mux.Handle("POST /api/v1/webhooks/{id}/test",   am.Authenticate(am.RequireRole("admin")(p.Forward("notification-service:8087"))))

// Audit log
mux.Handle("GET  /api/v1/audit-log",            am.Authenticate(am.RequireRole("admin")(p.Forward("audit-service:8090"))))

// JIRA config
mux.Handle("GET  /api/v1/jira/config",          am.Authenticate(am.RequireRole("admin")(p.Forward("jira-service:8088"))))
mux.Handle("POST /api/v1/jira/config",          am.Authenticate(am.RequireRole("admin")(p.Forward("jira-service:8088"))))
mux.Handle("POST /api/v1/jira/config/test",     am.Authenticate(am.RequireRole("admin")(p.Forward("jira-service:8088"))))

// System health + settings (BFF in gateway)
adminBFF := am.Authenticate(am.RequireRole("admin"))
mux.Handle("GET  /api/v1/admin/health",    adminBFF(bff.HandleAdminHealth))
mux.Handle("GET  /api/v1/admin/settings",  adminBFF(bff.GetSettings))
mux.Handle("PATCH /api/v1/admin/settings", adminBFF(bff.UpdateSettings))
```

---

## 8. Database Schema Additions

```sql
-- finding-service: thêm columns nếu chưa có
ALTER TABLE findings ADD COLUMN IF NOT EXISTS epss_score NUMERIC(6,5);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS is_kev BOOLEAN DEFAULT FALSE;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS assigned_to VARCHAR(255);
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_ip INET;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS asset_hostname VARCHAR(255);

-- finding_notes: nếu chưa có
CREATE TABLE IF NOT EXISTS finding_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id  UUID NOT NULL REFERENCES findings(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_by  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- platform_settings: system settings store (gateway/shared)
CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value_json  JSONB NOT NULL,
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 9. Acceptance Criteria

| Test | Expected |
|------|----------|
| `GET /api/v1/findings` | Response includes `by_severity`, `by_status`, `sla_stats`, `epss_score`, `is_kev` per finding |
| `GET /api/v1/findings?sla_status=breached` | Only findings với `sla_expiration_date < NOW()` |
| `PATCH /api/v1/findings/{id}` invalid transition | 409 `INVALID_TRANSITION` |
| `POST /api/v1/findings/bulk/reopen` | All findings → Active, NATS events published |
| `GET /api/v1/products/grades` | All products với grade + score + trend |
| `GET /api/v1/products` | Each item has `grade`, `score`, `finding_summary` |
| `GET /api/v1/reports/{id}/download` completed report | 302 redirect to MinIO presigned URL |
| `POST /api/v1/webhooks/{id}/test` | 200 với `response_code` và `response_time_ms` |
| `GET /api/v1/audit-log` as admin | 200 paginated events |
| `GET /api/v1/admin/health` | 200 với 7 services + infra status |
| `POST /api/v1/jira/config/test` | 200 `{"success":true, "jira_version":"..."}` |
