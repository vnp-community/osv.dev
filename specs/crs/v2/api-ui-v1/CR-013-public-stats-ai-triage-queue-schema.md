# Change Request 013: Public Stats Endpoint (No Auth) & AI Triage Queue Schema

**Tạo:** 2026-06-19  
**Status:** New — 1 endpoint public mới + AI Triage Queue schema upgrade.  
**Nguồn:** openapi.yaml schemas `PublicStats`, `AITriageQueueItem`, `AITriageQueueResponse`, `ReviewTriageRequest`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

---

## 1. Bối cảnh

### 1.1 Public Stats

`LoginScreen.tsx` sau khi fix hardcode gọi `GET /api/v2/public/stats` để hiển thị platform stats trên màn hình login. Endpoint này **phải public** (không cần JWT).

### 1.2 AI Triage Queue Schema

`AITriage.tsx` sau khi fix hardcode gọi `GET /api/v1/ai/triage/queue` và cần `AITriageQueueResponse` đầy đủ — bao gồm `ai_result`, `human_decision`, `stats` summary block.

| Thay đổi | Endpoint | Service | Trạng thái |
|---|---|---|---|
| Endpoint mới (public) | `GET /api/v2/public/stats` | gateway BFF | ❌ THIẾU |
| Schema upgrade | `GET /api/v1/ai/triage/queue` response | ai-service | ⚠️ Thiếu fields |
| Schema update | `POST /api/v1/ai/triage/{findingId}/review` body | ai-service | ⚠️ Thiếu `note` field |

---

## 2. Chi tiết Thay đổi

### 2.1 [HIGH] `GET /api/v2/public/stats` — Platform Public Statistics

**Tính chất quan trọng:**
- ❌ **Không cần JWT** — endpoint này PHẢI accessible mà không có Authorization header
- ✅ Rate limited (prevent abuse)
- ✅ Backend cache TTL 5 phút

**Gateway routing** — thêm vào `apps/osv/internal/gateway/router.go`:

```go
// PUBLIC endpoint — không có protected() middleware
mux.Handle("GET /api/v2/public/stats",
    rl.Limit("60/minute")(http.HandlerFunc(publicBFF.HandlePublicStats)))
```

**Implementation — Gateway BFF** (tổng hợp từ các services):

```go
// apps/osv/internal/gateway/bff/public_bff.go
package bff

type PublicBFF struct {
    scanClient    ScanServiceClient
    findingClient FindingServiceClient
    cache         *redis.Client
}

func (b *PublicBFF) HandlePublicStats(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Check cache first
    cacheKey := "public:stats"
    if cached, err := b.cache.Get(ctx, cacheKey).Result(); err == nil {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Cache", "HIT")
        w.Write([]byte(cached))
        return
    }
    
    // Aggregate từ nhiều services
    stats := PublicStats{
        TotalCVEs:      "240K+",              // Từ cve-service (static hoặc DB count)
        ScansToday:     b.getScansToday(ctx), // Từ scan-service
        FindingAccuracy: "98.4%",             // Từ ai-service metrics (hoặc config)
        UptimeSLA:      "99.99%",             // Từ monitoring metrics (hoặc config)
        ThreatIndicators: ThreatIndicators{
            CriticalThreats: b.getCriticalCount(ctx),    // Từ finding-service
            KEVActive:       b.getKEVActiveCount(ctx),   // Từ kev-service
            AssetsAtRisk:    b.getAssetsAtRisk(ctx),     // Từ asset-service
        },
    }
    
    // Cache 5 phút
    data, _ := json.Marshal(stats)
    b.cache.Set(ctx, cacheKey, data, 5*time.Minute)
    
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Cache", "MISS")
    w.Write(data)
}
```

**Fallback logic** — nếu một service down, dùng giá trị default/cached:
```go
func (b *PublicBFF) getScansToday(ctx context.Context) int {
    count, err := b.scanClient.CountCompletedToday(ctx)
    if err != nil {
        // Graceful degradation — trả về 0, không panic
        return 0
    }
    return count
}
```

**Response:**
```json
{
  "total_cves": "240K+",
  "scans_today": 1847,
  "finding_accuracy": "98.4%",
  "uptime_sla": "99.99%",
  "threat_indicators": {
    "critical_threats": 14,
    "kev_active": 7,
    "assets_at_risk": 23
  }
}
```

**CORS headers** (nếu login page serve từ domain khác):
```go
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET")
```

---

### 2.2 [HIGH] AI Triage Queue — Schema Upgrade

`GET /api/v1/ai/triage/queue` hiện trả về:
```json
{ "items": [{ "finding_id": "...", "status": "pending" }] }
```

Frontend cần `AITriageQueueResponse` đầy đủ:

```json
{
  "items": [
    {
      "finding_id": "F-2847",
      "finding_title": "Apache Log4j RCE via JNDI Injection",
      "cve_id": "CVE-2025-44228",
      "severity": "Critical",
      "ai_result": {
        "remarks": "Confirmed",
        "confidence": 0.94,
        "justification": "Log4j 2.x present in classpath. JNDI lookup enabled. External connectivity confirmed.",
        "actions": ["Upgrade to log4j 2.17.1+", "Disable JNDI lookups", "Apply WAF rules"],
        "generated_at": "2026-06-19T02:00:00Z"
      },
      "human_decision": null,
      "human_note": null,
      "reviewed_by": null,
      "reviewed_at": null
    }
  ],
  "total": 47,
  "stats": {
    "pending": 47,
    "accepted_today": 23,
    "avg_confidence": 87.4,
    "false_positive_rate": 4.2
  }
}
```

**AI-service handler update:**

```go
// services/ai-service/internal/handler/triage.go
func (h *Handler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    items, _ := h.repo.GetQueueItems(ctx, TriageQueueFilter{
        Status:   r.URL.Query().Get("status"),
        Severity: r.URL.Query().Get("severity"),
        Page:     parseIntParam(r, "page", 1),
        PageSize: parseIntParam(r, "page_size", 20),
    })
    
    total, _ := h.repo.CountQueueItems(ctx)
    stats, _ := h.repo.GetQueueStats(ctx)
    
    json.NewEncoder(w).Encode(AITriageQueueResponse{
        Items: items,
        Total: total,
        Stats: stats,
    })
}
```

**DB schema update** — bảng `triage_queue` cần thêm human decision fields:

```sql
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS human_decision VARCHAR(20) 
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    ADD COLUMN IF NOT EXISTS human_note TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
```

---

### 2.3 [MEDIUM] `POST /api/v1/ai/triage/{findingId}/review` — Thêm `note` Field

Request body cần support `note`:

```json
{
  "decision": "overridden",
  "note": "Môi trường sandbox — không ảnh hưởng production"
}
```

**Handler update:**

```go
// services/ai-service/internal/handler/triage.go  
func (h *Handler) ReviewFinding(w http.ResponseWriter, r *http.Request) {
    var req ReviewTriageRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    err := h.repo.UpdateHumanDecision(r.Context(), r.PathValue("findingId"), HumanDecisionUpdate{
        Decision:   req.Decision,
        Note:       req.Note,
        ReviewedBy: extractUserFromContext(r.Context()),
        ReviewedAt: time.Now(),
    })
    
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

### 2.4 Audit Log — Thêm `search` và `severity` query params

`GET /api/v1/audit-log` cần thêm params:
- `search` — full-text search trong action, username, resource
- `severity` — filter: `Info | Warning | Critical`
- `date_from`, `date_to` — ISO datetime range

Backend audit-log handler cần support các query params này.

---

## 3. Tiêu chí nghiệm thu (Acceptance Criteria)

### Public Stats
1. `GET /api/v2/public/stats` trả về `PublicStats` — HTTP 200 **không cần Authorization header**.
2. Response có đủ 5 fields: `total_cves`, `scans_today`, `finding_accuracy`, `uptime_sla`, `threat_indicators`.
3. Nếu một upstream service down, endpoint vẫn trả HTTP 200 với giá trị default (graceful degradation).
4. Response được cache — `X-Cache: HIT` trong 5 phút sau lần đầu call.
5. Không cache quá 10 phút — `scans_today` phải reasonably fresh.

### AI Triage Queue
6. `GET /api/v1/ai/triage/queue` trả về `AITriageQueueResponse` với `items`, `total`, `stats`.
7. Mỗi item có `ai_result` object với `remarks`, `confidence`, `justification`, `actions`.
8. `human_decision` là `null` nếu chưa review, hoặc `"accepted" | "overridden" | "rejected"`.
9. `stats.pending` phản ánh số items chưa có `human_decision`.
10. `POST /api/v1/ai/triage/{findingId}/review` với `{ decision, note }` update database — HTTP 200.
11. Sau khi review, `GET /api/v1/ai/triage/queue` phản ánh `human_decision` đã set.

### Audit Log
12. `GET /api/v1/audit-log?search=login` trả về entries có "login" trong action/username/resource.
13. `GET /api/v1/audit-log?severity=Critical` chỉ trả critical audit events.
14. `GET /api/v1/audit-log?date_from=2026-06-01&date_to=2026-06-19` filter theo time range.
