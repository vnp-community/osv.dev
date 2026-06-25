# SOL-UI-005 — v3.0 Planned Services: Scanning, Assets, AI (CR-UI-004, 006, 008)

**Giải pháp cho:** CR-UI-004, CR-UI-006, CR-UI-008  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** ✅ Backend Services Implemented — *CR-OVS-001 (scan-service), CR-OVS-007 (asset-service), CR-OVS-005 (ai-service) đã implement xong 2026-06-17; sẵn sàng cắm gateway routes*  
**Services đã xây dựng:**
- `scan-service` v3.0 với Nmap/ZAP/Agent + SSE (:8058) — ✅ [SOL-OVS-001](../../v1/OpenVulnScan/solutions/SOL-OVS-001-scan-service.md)
- `asset-service` (:8068) — ✅ [SOL-OVS-007](../../v1/OpenVulnScan/solutions/SOL-OVS-007-asset-management.md)
- `ai-service` (:8052) — ✅ [SOL-OVS-005](../../v1/OpenVulnScan/solutions/SOL-OVS-005-ai-service.md)  
**Phụ thuộc kiến trúc:** `specs/01-architecture.md §10.2–10.7`, `specs/02-technical-design.md §14.2, 14.5, 14.7`

---

> **✅ Cập nhật 2026-06-17:** Cả 3 backend services đã implement xong theo spec SOL-OVS tương ứng. **Bước tiếp theo:** cắm gateway routes (mục 1.3, 2.3, 3.3) và deploy docker-compose.
>
> Tài liệu này tóm tắt **mapping từ UI CR → OVS CR** và bổ sung các điểm cần thiết để UI hoạt động đúng, không duplicate toàn bộ OVS solutions.

---

## 1. Active Scanning API (CR-UI-004 → CR-OVS-001)

### 1.1 Reference

Giải pháp đầy đủ tại: `specs/crs/v1/OpenVulnScan/solutions/SOL-OVS-001-scan-service.md`

### 1.2 UI-Specific Requirements

Các điểm UI cần nhưng OVS CR có thể chưa cover đầy đủ:

**a) Scan list response cần thêm:**
```go
// scan-service response — ensure these fields exist
type ScanListItem struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    Type         string    `json:"type"`          // "nmap_full" | "nmap_discovery" | "zap" | "agent"
    Status       string    `json:"status"`         // pending | queued | running | completed | failed | cancelled
    Progress     int       `json:"progress"`       // 0-100
    Targets      []string  `json:"targets"`
    FindingCount int       `json:"finding_count"`
    CreatedAt    string    `json:"created_at"`
    StartedAt    *string   `json:"started_at"`
    CompletedAt  *string   `json:"completed_at"`
    Duration     *int      `json:"duration_seconds"`
    Tags         []string  `json:"tags"`           // for filtering
    CreatedBy    string    `json:"created_by"`
}
```

**b) Scheduled scans (CR-UI-004 §4):**
```go
// GET /api/v1/scans/scheduled — UI needs this for the cron scheduler UI

type ScheduledScan struct {
    ID           string  `json:"id"`
    Name         string  `json:"name"`
    CronExpr     string  `json:"cron_expr"`        // "0 2 * * *"
    ScanType     string  `json:"scan_type"`
    Targets      []string `json:"targets"`
    IsEnabled    bool    `json:"is_enabled"`
    NextRunAt    string  `json:"next_run_at"`
    LastRunAt    *string `json:"last_run_at"`
    LastStatus   *string `json:"last_status"`
}
```

**c) Scan import (upload file):**
```go
// POST /api/v1/scans/import — multipart form
// Content-Type: multipart/form-data
// Fields: file (binary), scan_type (nmap_xml|zap_json|bandit|...), product_id, engagement_id, test_title

// Response:
type ScanImportResult struct {
    ScanID          string `json:"scan_id"`
    Created         int    `json:"created"`
    Duplicates      int    `json:"duplicates"`
    Total           int    `json:"total"`
    FindingIDs      []string `json:"finding_ids"`
}
```

### 1.3 Gateway Routes for Scanning

```go
// apps/osv/internal/gateway/router.go

// Scan service routes (all JWT authenticated)
mux.Handle("GET  /api/v1/scans",                am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("POST /api/v1/scans",                am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("GET  /api/v1/scans/{id}",           am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("POST /api/v1/scans/{id}/cancel",    am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("GET  /api/v1/scans/{id}/stream",    am.Authenticate(p.ForwardSSE("scan-service:8058")))  // SSE
mux.Handle("GET  /api/v1/scans/{id}/results/nmap", am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("GET  /api/v1/scans/{id}/results/zap",  am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("GET  /api/v1/scans/scheduled",      am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("POST /api/v1/scans/scheduled",      am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("PATCH /api/v1/scans/scheduled/{id}", am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("DELETE /api/v1/scans/scheduled/{id}", am.Authenticate(p.Forward("scan-service:8058")))
mux.Handle("POST /api/v1/scans/import",         am.Authenticate(p.ForwardWithMaxBody("scan-service:8058", 100*1024*1024))) // 100MB max
```

### 1.4 SSE Event Format (UI expectation)

Frontend event listener expects:
```javascript
// Frontend EventSource handler
const source = new EventSource(`/api/v1/scans/${scanId}/stream`, {
    headers: { Authorization: `Bearer ${token}` }
})

source.addEventListener('progress', (e) => {
    const data = JSON.parse(e.data)
    // data: { status, progress, current_target, hosts_discovered, findings_so_far }
})

source.addEventListener('done', () => {
    source.close()
})
```

**scan-service SSE must emit:**
```
event: progress
data: {"status":"running","progress":45,"current_target":"192.168.1.10","hosts_discovered":12,"findings_so_far":8}

event: progress
data: {"status":"running","progress":100}

event: done
data: {"status":"completed","finding_count":23}
```

---

## 2. Asset Management API (CR-UI-006 → CR-OVS-007)

### 2.1 Reference

Giải pháp đầy đủ tại: `specs/crs/v1/OpenVulnScan/solutions/SOL-OVS-007-asset-service.md`

### 2.2 UI-Specific Requirements

**Asset list response:**
```go
// asset-service response type
type AssetListItem struct {
    ID           string    `json:"id"`
    IPAddress    string    `json:"ip_address"`
    Hostname     *string   `json:"hostname"`
    OS           *string   `json:"os"`
    Type         string    `json:"type"`           // "server" | "workstation" | "network_device" | "web_app"
    Status       string    `json:"status"`         // "active" | "inactive" | "decommissioned"
    Tags         []string  `json:"tags"`
    RiskScore    float64   `json:"risk_score"`     // 0.0-10.0
    ActiveFindings int     `json:"active_findings"`
    LastScanAt   *string   `json:"last_scan_at"`
    Services     []ServicePort `json:"services"` // open ports/services
}

type ServicePort struct {
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "tcp" | "udp"
    Service  string `json:"service"`  // "http" | "ssh" | "ftp"
    Version  string `json:"version"`
    State    string `json:"state"`    // "open" | "filtered"
}
```

**Asset findings:**
```go
// GET /api/v1/assets/{id}/findings — reuse FindingListItem from finding-service
// Filter by asset_ip or asset_hostname
```

**Asset update (tags, type, status):**
```go
type UpdateAssetRequest struct {
    Type   *string  `json:"type"`
    Status *string  `json:"status"`
    Tags   []string `json:"tags"`
}
```

**Asset tags endpoint** (for UI filter dropdown):
```go
// GET /api/v1/assets/tags
// Response: {"tags": ["production", "dmz", "internal", "k8s-node"], "total": 4}
```

### 2.3 Gateway Routes for Assets

```go
// Asset service routes
mux.Handle("GET  /api/v1/assets",               am.Authenticate(p.Forward("asset-service:8068")))
mux.Handle("GET  /api/v1/assets/tags",          am.Authenticate(p.Forward("asset-service:8068")))
mux.Handle("GET  /api/v1/assets/{id}",          am.Authenticate(p.Forward("asset-service:8068")))
mux.Handle("PATCH /api/v1/assets/{id}",         am.Authenticate(p.Forward("asset-service:8068")))
mux.Handle("GET  /api/v1/assets/{id}/findings", am.Authenticate(p.Forward("finding-service:8085")))
// Note: /assets/{id}/findings forwards to finding-service (asset_ip filter)
```

### 2.4 Risk Score API

UI Asset Management page cần:
```json
// GET /api/v1/assets?sort_by=risk_score_desc&status=active
{
  "assets": [...],
  "total": 47,
  "risk_summary": {
    "critical": 3,
    "high": 12,
    "medium": 18,
    "low": 14
  }
}
```

`risk_summary` = aggregate của `risk_score`:
```go
// asset-service: add to ListAssets handler
func (h *Handler) ListAssets(w http.ResponseWriter, r *http.Request) {
    assets, total, err := h.assetRepo.List(r.Context(), filter)
    
    riskSummary := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
    for _, a := range assets {
        switch {
        case a.RiskScore >= 9.0: riskSummary["critical"]++
        case a.RiskScore >= 7.0: riskSummary["high"]++
        case a.RiskScore >= 4.0: riskSummary["medium"]++
        default:                 riskSummary["low"]++
        }
    }
    
    respondJSON(w, 200, map[string]interface{}{
        "assets": mapAssets(assets), "total": total,
        "risk_summary": riskSummary,
    })
}
```

---

## 3. AI Center API (CR-UI-008 → CR-OVS-005)

### 3.1 Reference

Giải pháp đầy đủ tại: `specs/crs/v1/OpenVulnScan/solutions/SOL-OVS-005-ai-service.md`

### 3.2 UI-Specific Requirements

**AI Triage Queue:**
```go
// GET /api/v1/ai/triage/queue?status=pending&page=1
type TriageQueueItem struct {
    FindingID    string   `json:"finding_id"`
    FindingTitle string   `json:"finding_title"`
    CveID        *string  `json:"cve_id"`
    Severity     string   `json:"severity"`
    ProductName  string   `json:"product_name"`
    Status       string   `json:"status"`      // "pending" | "analyzing" | "completed" | "rejected"
    AIResult     *AIResult `json:"ai_result"`  // null if pending
    QueuedAt     string   `json:"queued_at"`
}

type AIResult struct {
    TriageStatus  string  `json:"triage_status"` // "confirmed" | "false_positive" | "needs_review"
    Confidence    float64 `json:"confidence"`
    Remarks       string  `json:"remarks"`
    EPSSScore     float64 `json:"epss_score"`
    Reasoning     string  `json:"reasoning"`
    AnalyzedAt    string  `json:"analyzed_at"`
}
```

**Trigger AI triage for finding:**
```go
// POST /api/v1/ai/triage/{findingId}
// Request: {} (empty, or optional priority)
// Response:
type TriggerTriageResponse struct {
    FindingID string `json:"finding_id"`
    Status    string `json:"status"`    // "queued" | "already_queued" | "completed"
    QueuePosition int `json:"queue_position"`
    EstimatedWait string `json:"estimated_wait"` // "< 1 min" | "2-5 min"
}
```

**AI Triage review (human decision):**
```go
// POST /api/v1/ai/triage/{findingId}/review
type ReviewRequest struct {
    Decision string `json:"decision"` // "approve" | "reject" | "escalate"
    Comment  string `json:"comment"`
}
// On "approve": auto-close finding (call finding-service)
// On "reject": mark finding as needing manual review
```

**CVE Enrichment status:**
```go
// GET /api/v1/ai/enrichment
// List CVEs with AI enrichment status

type CVEEnrichmentItem struct {
    CveID         string   `json:"cve_id"`
    Status        string   `json:"status"`    // "enriched" | "pending" | "failed"
    AISeverity    string   `json:"ai_severity"`
    Confidence    float64  `json:"confidence"`
    HasExploit    bool     `json:"has_exploit"`
    EmbeddingDims int      `json:"embedding_dims"` // 1536
    EnrichedAt    *string  `json:"enriched_at"`
}

// POST /api/v1/ai/enrichment/trigger — trigger enrichment for specific CVE
// Body: {"cve_id": "CVE-2024-1234"}

// GET /api/v1/ai/enrichment/{cveId} — enrichment detail for one CVE
```

### 3.3 Gateway Routes for AI

```go
// AI service routes
mux.Handle("POST /api/v1/ai/triage/{findingId}",        am.Authenticate(p.Forward("ai-service:8052")))
mux.Handle("POST /api/v1/ai/triage/{findingId}/review", am.Authenticate(p.Forward("ai-service:8052")))
mux.Handle("GET  /api/v1/ai/triage/queue",              am.Authenticate(p.Forward("ai-service:8052")))
mux.Handle("GET  /api/v1/ai/enrichment",                am.Authenticate(p.Forward("ai-service:8052")))
mux.Handle("POST /api/v1/ai/enrichment/trigger",        am.Authenticate(am.RequireRole("admin", "user")(p.Forward("ai-service:8052"))))
mux.Handle("GET  /api/v1/ai/enrichment/{cveId}",        am.Authenticate(p.Forward("ai-service:8052")))
```

### 3.4 AI Triage Database (ai-service)

```sql
-- ai-service database — triage queue table
CREATE TABLE finding_triage_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id      UUID NOT NULL UNIQUE,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority        INT NOT NULL DEFAULT 5,  -- 1=highest, 10=lowest
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    analyzed_at     TIMESTAMPTZ,
    triage_status   VARCHAR(20),  -- confirmed, false_positive, needs_review
    confidence      NUMERIC(5,4),
    remarks         TEXT,
    reasoning       TEXT,
    epss_score      NUMERIC(6,5),
    reviewed_by     VARCHAR(255),
    reviewed_at     TIMESTAMPTZ,
    human_decision  VARCHAR(20)   -- approve, reject, escalate
);

CREATE INDEX idx_triage_queue_status ON finding_triage_queue(status, priority, queued_at);
```

---

## 4. MFA + OAuth2 Auth Extensions (CR-UI-001 §2.5-2.9 → CR-OVS-003)

> Ces endpoints seront implémentés par `auth-service` v3.0 (CR-OVS-003) avec RS256, Argon2id, TOTP.

### 4.1 MFA Setup Flow

```go
// GET /api/v1/auth/mfa/setup
// Returns TOTP QR code data
type MFASetupResponse struct {
    Secret     string `json:"secret"`       // base32 encoded, display once
    QRCodeURL  string `json:"qr_code_url"`  // data:image/png;base64,... for QR display
    OTPAuthURL string `json:"otp_auth_url"` // otpauth://totp/OSV:user@example.com?secret=...
}

// POST /api/v1/auth/mfa/confirm
// Confirms TOTP code to enable MFA
type MFAConfirmRequest struct {
    Code string `json:"code"` // 6-digit TOTP
}
```

### 4.2 OAuth2 Flow

```go
// GET /api/v1/auth/oauth/google  → redirect to Google OAuth consent
// GET /api/v1/auth/oauth/github  → redirect to GitHub OAuth consent
// GET /api/v1/auth/callback?code=...&state=... → exchange code, issue JWT, redirect to UI

// callback redirect: {FRONTEND_URL}/auth/callback?token={access_token}
```

### 4.3 Gateway Routes for v3.0 Auth

```go
// v3.0 only — added when auth-service:8051 is deployed
mux.Handle("GET  /api/v1/auth/mfa/setup",    am.Authenticate(p.Forward("auth-service:8051")))
mux.Handle("POST /api/v1/auth/mfa/confirm",  am.Authenticate(p.Forward("auth-service:8051")))
mux.Handle("GET  /api/v1/auth/oauth/google", p.Forward("auth-service:8051"))
mux.Handle("GET  /api/v1/auth/oauth/github", p.Forward("auth-service:8051"))
mux.Handle("GET  /api/v1/auth/callback",     p.Forward("auth-service:8051"))
```

---

## 5. Migration Path: v2.2 → v3.0

Để tránh breaking changes khi upgrade:

| Phase | Action |
|-------|--------|
| **Phase 1** (Sprint 1-2) | Deploy v2.2 identity-service extensions (SOL-UI-001) — login via `/api/v1/auth/login` |
| **Phase 2** (Sprint 3-4) | Deploy scan-service v3.0 — UI tự detect via `/api/v1/admin/health` → enable Scanning tab |
| **Phase 3** (Sprint 5-6) | Deploy asset-service, ai-service — UI enable Assets + AI tabs |
| **Phase 4** (Sprint 7) | Deploy auth-service v3.0 — UI enable MFA/OAuth2 settings |
| **Cutover** | Redirect `/api/v1/auth/*` từ identity-service:8081 → auth-service:8051 in gateway |

**Feature flags via health API:**
```go
// Frontend checks /api/v1/admin/health on app init
// If "scan-service" not in response → disable Scanning tab
// If "asset-service" not in response → disable Assets tab
// If "ai-service" not in response → disable AI Center tab
```

---

## 6. Summary: CR-UI → Service Map

| CR | UI Feature | Service | Version |
|----|-----------|---------|---------|
| CR-UI-001 | Auth/Profile/API Keys | identity-service:8081 | v2.2 ext |
| CR-UI-001 (§2.5+) | MFA/OAuth2 | auth-service:8051 | v3.0 |
| CR-UI-002 | Dashboard BFF | apps/osv (gateway) | v2.2 ext |
| CR-UI-002 | SSE Notifications | notification-service:8087 | v2.2 ext |
| CR-UI-003 | CVE Schema Updates | data-service:8082 + search-service | v2.2 ext |
| CR-UI-004 | Active Scanning + SSE | scan-service:8058 | v3.0 |
| CR-UI-005 | Finding Extensions | finding-service:8085 | v2.2 ext |
| CR-UI-006 | Asset Management | asset-service:8068 | v3.0 |
| CR-UI-007 | Product Grades | finding-service:8085 | v2.2 ext |
| CR-UI-008 | AI Center | ai-service:8052 | v3.0 |
| CR-UI-009 | Reports + Webhooks | finding-service:8085 + notification-service:8087 | v2.2 ext |
| CR-UI-010 | Admin + JIRA + Health | identity-service + jira-service + gateway BFF | v2.2 ext |

---

## 7. Acceptance Criteria (v3.0 features)

> **Chú thích:** Backend services đã implement. Các test này sẵn sàng chạy sau khi gateway routes được cắm và deploy.

| Test | Expected | Trạng thái |
|------|----------|-------------|
| `POST /api/v1/scans` với targets["192.168.1.0/24"] | 201 `{"id":"...","status":"pending"}` | ⏳ Gateway route pending |
| `GET /api/v1/scans/{id}/stream` | SSE stream với progress events | ⏳ Gateway SSE route pending |
| SSE terminates on scan completion | `event: done` received, EventSource closed | ⏳ Gateway SSE route pending |
| `GET /api/v1/assets` | 200 với `risk_summary` aggregate | ⏳ Gateway route pending |
| `GET /api/v1/assets/{id}/findings` | Findings filtered by asset IP | ⏳ Gateway route pending |
| `POST /api/v1/ai/triage/{findingId}` | 200 `{"status":"queued","queue_position":N}` | ⏳ Gateway route pending |
| `GET /api/v1/ai/triage/queue` | List với `ai_result` populated for completed | ⏳ Gateway route pending |
| `GET /api/v1/auth/mfa/setup` | 200 với `qr_code_url` và `otp_auth_url` | ⏳ auth-service v3.0 pending |

**Blocker duy nhất còn lại:** gateway router chưa có routes cho scan-service, asset-service, ai-service (mục 1.3, 2.3, 3.3 trong tài liệu này).
