# Change Request 014: AI Triage Queue — Full Schema with Human Decision Fields

**Tạo:** 2026-06-19  
**Status:** New — Schema upgrade toàn diện + DB migration + audit trail cho human decisions.  
**Nguồn:** [08_ai_triage.md](../../../ui/specs/bugs/hardcode/solutions/08_ai_triage.md) · openapi.yaml schemas `AITriageQueueItem`, `AITriageQueueStats`, `AITriageQueueResponse`, `ReviewTriageRequest`  
**Target directory:** `specs/crs/v2/api-ui-v1/`

> **Lưu ý:** CR-013 đã đề cập sơ lược về AI triage. CR-014 này bổ sung spec chi tiết đầy đủ bao gồm DB schema, business logic, edge cases, và audit trail.

---

## 1. Bối cảnh

### 1.1 Vấn đề hiện tại

`AITriage.tsx` hiện có:
```typescript
// ❌ HARDCODE — features/ai-center/components/AITriage.tsx
const queue = [
  { findingId: 'F-2847', remarks: 'Confirmed', confidence: 0.98, ... },
  { findingId: 'F-2843', remarks: 'FalsePositive', confidence: 0.87, ... },
  // 6 items cứng — không phản ánh dữ liệu thực
];
// Stats hardcode: { label: "Accepted Today", value: 8 }
// ❌ Accept/Override/Reject button không gọi API — không lưu vào DB
```

### 1.2 Sau khi fix

Frontend sử dụng `useAITriageQueue()` hook để:
1. Load queue từ `GET /api/v1/ai/triage/queue`
2. Gửi human decision qua `POST /api/v1/ai/triage/{findingId}/review`
3. Stats (pending, accepted today, avg confidence, FP rate) tính từ server

### 1.3 Gap analysis

| Item | Trạng thái hiện tại | Yêu cầu |
|---|---|---|
| `GET /api/v1/ai/triage/queue` response schema | `TriageQueueItem` (cũ — thiếu `ai_result`, `human_decision`) | `AITriageQueueResponse` đầy đủ |
| `ai_result` object trong queue item | ❌ Không có | `{ remarks, confidence, justification, actions, generated_at }` |
| `human_decision` field | ❌ Không có | `"accepted" \| "overridden" \| "rejected" \| null` |
| `human_note` field | ❌ Không có | Optional text note từ reviewer |
| `reviewed_by` + `reviewed_at` | ❌ Không có | Audit trail |
| `stats` block trong response | ❌ Không có | `{ pending, accepted_today, avg_confidence, false_positive_rate }` |
| `POST /api/v1/ai/triage/{id}/review` body | Chỉ có `decision` | Thêm `note` field |
| DB `triage_queue` table | Thiếu human decision columns | Migration cần thiết |

---

## 2. Spec chi tiết

### 2.1 Request/Response Schemas

#### `GET /api/v1/ai/triage/queue`

**Query Parameters:**

| Param | Type | Default | Mô tả |
|---|---|---|---|
| `status` | string | `all` | Filter: `pending \| accepted \| overridden \| rejected \| all` |
| `severity` | string | — | Filter: `Critical \| High \| Medium \| Low` |
| `remarks` | string | — | Filter AI verdict: `Confirmed \| FalsePositive \| NotAffected \| Unexplored` |
| `page` | integer | `1` | Pagination |
| `page_size` | integer | `20` | Max 100 |

**Response `AITriageQueueResponse`:**

```json
{
  "items": [
    {
      "finding_id": "F-2847",
      "finding_title": "Apache Log4j2 JNDI RCE via LDAP",
      "cve_id": "CVE-2025-44228",
      "severity": "Critical",
      "ai_result": {
        "remarks": "Confirmed",
        "confidence": 0.98,
        "justification": "CVSS 10.0, EPSS 98.2%, active CISA KEV. Host webserver01 runs Log4j2 2.14.0 (vulnerable). No compensating controls detected.",
        "actions": [
          "Upgrade Log4j2 to 2.17.1+",
          "Apply JVM flag: -Dlog4j2.formatMsgNoLookups=true",
          "Enable WAF rule CVE-2021-44228"
        ],
        "generated_at": "2026-06-19T04:00:00Z"
      },
      "human_decision": null,
      "human_note": null,
      "reviewed_by": null,
      "reviewed_at": null
    },
    {
      "finding_id": "F-2843",
      "finding_title": "Information Disclosure via Stack Trace",
      "cve_id": null,
      "severity": "Medium",
      "ai_result": {
        "remarks": "FalsePositive",
        "confidence": 0.87,
        "justification": "Stack trace appears in non-production error page only. Header 'X-Environment: dev' confirms staging instance.",
        "actions": [
          "Verify environment isolation",
          "Disable debug mode in production config"
        ],
        "generated_at": "2026-06-19T03:30:00Z"
      },
      "human_decision": "accepted",
      "human_note": "Confirmed staging-only exposure",
      "reviewed_by": "bob.chen@company.com",
      "reviewed_at": "2026-06-19T04:30:00Z"
    }
  ],
  "total": 47,
  "stats": {
    "pending": 31,
    "accepted_today": 8,
    "avg_confidence": 87,
    "false_positive_rate": 12.3
  }
}
```

---

#### `POST /api/v1/ai/triage/{findingId}/review`

**Request body `ReviewTriageRequest`:**

```json
{
  "decision": "accepted",
  "note": "Confirmed staging-only. No production impact."
}
```

| Field | Type | Required | Mô tả |
|---|---|---|---|
| `decision` | string enum | ✅ | `"accepted" \| "overridden" \| "rejected"` |
| `note` | string | ❌ | Optional reviewer note (max 1000 chars) |

**Response:**

```json
{ "success": true }
```

**Error cases:**

| Status | Điều kiện |
|---|---|
| `404` | Finding ID không tồn tại trong triage queue |
| `409` | Finding đã được review (`human_decision != null`). Dùng thêm param `?force=true` để override |
| `403` | User không có quyền `finding:write` |

---

### 2.2 AI Triage Flow

```
Finding được phát hiện (scan completed)
         ↓
scan-service emit event → ai-service
         ↓
ai-service call LLM provider (Ollama/OpenAI)
         ↓
ai-service store result vào triage_queue
    - remarks (Confirmed/FalsePositive/NotAffected/Unexplored)
    - confidence (0-1)
    - justification (text)
    - actions (string[])
    - human_decision = NULL (chưa review)
         ↓
Frontend poll GET /api/v1/ai/triage/queue
         ↓
Analyst review → POST /api/v1/ai/triage/{id}/review
         ↓
ai-service cập nhật human_decision, human_note, reviewed_by, reviewed_at
         ↓
(optional) ai-service cập nhật finding status nếu decision = "accepted"
```

---

### 2.3 DB Schema Migration

**ai-service** cần thêm columns vào bảng `triage_queue`:

```sql
-- Migration: add human decision fields to triage_queue
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS human_decision VARCHAR(20)
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    ADD COLUMN IF NOT EXISTS human_note TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Thêm index cho filter
CREATE INDEX IF NOT EXISTS idx_triage_human_decision ON triage_queue(human_decision);
CREATE INDEX IF NOT EXISTS idx_triage_reviewed_at    ON triage_queue(reviewed_at DESC);
```

**Cột `ai_result` hiện lưu như thế nào?** Nếu là JSON column:

```sql
-- Kiểm tra xem ai_result đã là JSON column chưa
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'triage_queue' AND column_name = 'ai_result';

-- Nếu chưa có, thêm vào:
ALTER TABLE triage_queue
    ADD COLUMN IF NOT EXISTS ai_result JSONB,
    ADD COLUMN IF NOT EXISTS ai_generated_at TIMESTAMPTZ;
```

**Full table schema sau migration:**

```sql
CREATE TABLE triage_queue (
    id              VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id      VARCHAR(36) NOT NULL UNIQUE,
    finding_title   TEXT NOT NULL,
    cve_id          VARCHAR(50),
    severity        VARCHAR(20) NOT NULL,
    
    -- AI Result
    remarks         VARCHAR(20) NOT NULL
        CHECK (remarks IN ('Confirmed', 'FalsePositive', 'NotAffected', 'Unexplored')),
    confidence      DECIMAL(5,4) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    justification   TEXT NOT NULL,
    actions         TEXT[] NOT NULL DEFAULT '{}',
    ai_generated_at TIMESTAMPTZ NOT NULL,
    
    -- Human Decision (nullable = pending)
    human_decision  VARCHAR(20)
        CHECK (human_decision IN ('accepted', 'overridden', 'rejected')),
    human_note      TEXT,
    reviewed_by     VARCHAR(255),
    reviewed_at     TIMESTAMPTZ,
    
    -- Metadata
    queued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

### 2.4 Stats Computation

`stats` block phải được compute từ DB — không hardcode:

```go
// services/ai-service/internal/repository/triage.go
type QueueStats struct {
    Pending          int     `json:"pending"`
    AcceptedToday    int     `json:"accepted_today"`
    AvgConfidence    float64 `json:"avg_confidence"`  // 0-100 (percent)
    FalsePositiveRate float64 `json:"false_positive_rate"` // percentage
}

func (r *Repo) GetQueueStats(ctx context.Context) (*QueueStats, error) {
    row := r.db.QueryRowContext(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE human_decision IS NULL)                    AS pending,
            COUNT(*) FILTER (
                WHERE human_decision = 'accepted' 
                AND reviewed_at >= CURRENT_DATE
            )                                                                 AS accepted_today,
            ROUND(AVG(confidence) * 100, 1)                                   AS avg_confidence,
            ROUND(
                100.0 * COUNT(*) FILTER (WHERE remarks = 'FalsePositive') 
                / NULLIF(COUNT(*), 0)
            , 1)                                                              AS false_positive_rate
        FROM triage_queue
    `)
    
    var stats QueueStats
    row.Scan(&stats.Pending, &stats.AcceptedToday, &stats.AvgConfidence, &stats.FalsePositiveRate)
    return &stats, nil
}
```

---

### 2.5 Review Handler — Full Implementation

```go
// services/ai-service/internal/handler/triage.go

func (h *Handler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q := r.URL.Query()
    
    filter := TriageFilter{
        Status:   q.Get("status"),   // maps to human_decision filter
        Severity: q.Get("severity"),
        Remarks:  q.Get("remarks"),
        Page:     parseIntDefault(q.Get("page"), 1),
        PageSize: min(parseIntDefault(q.Get("page_size"), 20), 100),
    }
    
    items, total, _ := h.repo.GetQueueItems(ctx, filter)
    stats, _         := h.repo.GetQueueStats(ctx)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(AITriageQueueResponse{
        Items: mapToResponse(items),
        Total: total,
        Stats: stats,
    })
}

func (h *Handler) ReviewFinding(w http.ResponseWriter, r *http.Request) {
    ctx       := r.Context()
    findingID := r.PathValue("findingId")
    force     := r.URL.Query().Get("force") == "true"
    
    var req ReviewTriageRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"INVALID_BODY","message":"Invalid JSON"}`, 400)
        return
    }
    
    // Validate decision enum
    valid := map[string]bool{"accepted": true, "overridden": true, "rejected": true}
    if !valid[req.Decision] {
        http.Error(w, `{"error":"INVALID_DECISION","message":"decision must be accepted|overridden|rejected"}`, 400)
        return
    }
    
    // Check finding exists
    existing, err := h.repo.GetQueueItem(ctx, findingID)
    if err != nil || existing == nil {
        http.Error(w, `{"error":"NOT_FOUND","message":"Finding not in triage queue"}`, 404)
        return
    }
    
    // Check already reviewed (409 Conflict)
    if existing.HumanDecision != nil && !force {
        http.Error(w, `{"error":"ALREADY_REVIEWED","message":"Use ?force=true to override"}`, 409)
        return
    }
    
    // Extract reviewer identity from JWT context
    reviewer := extractEmailFromContext(ctx)
    now      := time.Now().UTC()
    
    err = h.repo.UpdateHumanDecision(ctx, findingID, HumanDecisionUpdate{
        Decision:   req.Decision,
        Note:       req.Note,
        ReviewedBy: reviewer,
        ReviewedAt: now,
    })
    if err != nil {
        http.Error(w, `{"error":"INTERNAL","message":"Failed to save decision"}`, 500)
        return
    }
    
    // (Optional) Propagate decision to finding-service
    if req.Decision == "accepted" {
        go h.propagateDecision(context.Background(), findingID, existing.Remarks)
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

### 2.6 Gateway Routing

Kiểm tra `apps/osv/internal/gateway/router.go` — các routes AI đã tồn tại hay chưa:

```go
// Đảm bảo các routes này tồn tại và đúng thứ tự:
// (Queue phải TRƯỚC specific finding routes)
mux.Handle("GET /api/v1/ai/triage/queue",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("POST /api/v1/ai/triage/{findingId}/review",
    protected(proxy.Forward("ai-service:9103")))

mux.Handle("GET /api/v1/ai/triage/{findingId}",
    protected(proxy.Forward("ai-service:9103")))
```

---

### 2.7 (Optional) Propagate Decision to Finding-Service

Khi analyst `accept` một AI suggestion, finding-service có thể tự động:
- Update `finding.status` → `confirmed` (nếu `remarks = "Confirmed"`)
- Update `finding.status` → `false_positive` (nếu `remarks = "FalsePositive"`)

```go
// services/ai-service/internal/handler/triage.go
func (h *Handler) propagateDecision(ctx context.Context, findingID string, remarks string) {
    if remarks == "FalsePositive" {
        h.findingClient.UpdateStatus(ctx, findingID, "false_positive")
    } else if remarks == "Confirmed" {
        h.findingClient.AddTag(ctx, findingID, "ai-confirmed")
    }
}
```

---

## 3. MSW Handler Reference (Frontend Dev)

Frontend team dùng handler này để dev offline:

```typescript
// src/mocks/handlers/ai.handlers.ts
import { http, HttpResponse } from 'msw';

const triageQueueFixture = [
  {
    finding_id: 'F-2847', finding_title: 'Apache Log4j2 JNDI RCE via LDAP',
    cve_id: 'CVE-2025-44228', severity: 'Critical',
    ai_result: {
      remarks: 'Confirmed', confidence: 0.98,
      justification: 'CVSS 10.0, EPSS 98.2%, active CISA KEV. Host webserver01 runs Log4j2 2.14.0.',
      actions: ['Upgrade Log4j2 to 2.17.1+', 'Apply JVM flag: -Dlog4j2.formatMsgNoLookups=true'],
      generated_at: new Date(Date.now() - 3600000).toISOString(),
    },
    human_decision: null, human_note: null, reviewed_by: null, reviewed_at: null,
  },
  {
    finding_id: 'F-2843', finding_title: 'Information Disclosure via Stack Trace',
    cve_id: null, severity: 'Medium',
    ai_result: {
      remarks: 'FalsePositive', confidence: 0.87,
      justification: "Stack trace in non-production error page. X-Environment: dev confirmed.",
      actions: ['Verify environment isolation', 'Disable debug mode in prod'],
      generated_at: new Date(Date.now() - 7200000).toISOString(),
    },
    human_decision: 'accepted', human_note: 'Confirmed staging-only',
    reviewed_by: 'bob.chen@company.com',
    reviewed_at: new Date(Date.now() - 1800000).toISOString(),
  },
  {
    finding_id: 'F-2841', finding_title: 'Spring Framework RCE via ClassLoader',
    cve_id: 'CVE-2025-77001', severity: 'Critical',
    ai_result: {
      remarks: 'Confirmed', confidence: 0.94,
      justification: 'Spring 5.3.x detected in maven dependencies. Remote code execution via ClassLoader manipulation.',
      actions: ['Upgrade Spring to 6.0.6+', 'Patch CVE-2022-22965 mitigations'],
      generated_at: new Date(Date.now() - 1800000).toISOString(),
    },
    human_decision: null, human_note: null, reviewed_by: null, reviewed_at: null,
  },
];

export const aiHandlers = [
  http.get('/api/v1/ai/triage/queue', ({ request }) => {
    const url = new URL(request.url);
    const status   = url.searchParams.get('status');
    const severity = url.searchParams.get('severity');
    const remarks  = url.searchParams.get('remarks');

    let items = [...triageQueueFixture];
    if (status === 'pending')    items = items.filter(i => !i.human_decision);
    if (status === 'accepted')   items = items.filter(i => i.human_decision === 'accepted');
    if (status === 'overridden') items = items.filter(i => i.human_decision === 'overridden');
    if (status === 'rejected')   items = items.filter(i => i.human_decision === 'rejected');
    if (severity)                items = items.filter(i => i.severity === severity);
    if (remarks)                 items = items.filter(i => i.ai_result.remarks === remarks);

    const pending       = triageQueueFixture.filter(i => !i.human_decision).length;
    const acceptedToday = triageQueueFixture.filter(i => i.human_decision === 'accepted').length;
    const avgConf       = triageQueueFixture.reduce((s, i) => s + i.ai_result.confidence, 0)
                          / triageQueueFixture.length * 100;

    return HttpResponse.json({
      items,
      total: items.length,
      stats: {
        pending,
        accepted_today: acceptedToday,
        avg_confidence: Math.round(avgConf),
        false_positive_rate: 12.3,
      },
    });
  }),

  http.post('/api/v1/ai/triage/:findingId/review', async ({ params, request }) => {
    const body = await request.json() as { decision: string; note?: string };
    const item = triageQueueFixture.find(i => i.finding_id === params.findingId);

    if (!item) {
      return HttpResponse.json(
        { error: 'NOT_FOUND', message: 'Finding not in triage queue' },
        { status: 404 }
      );
    }

    if (item.human_decision) {
      const url = new URL(request.url);
      if (url.searchParams.get('force') !== 'true') {
        return HttpResponse.json(
          { error: 'ALREADY_REVIEWED', message: 'Use ?force=true to override' },
          { status: 409 }
        );
      }
    }

    Object.assign(item, {
      human_decision: body.decision,
      human_note: body.note ?? null,
      reviewed_by: 'current-user@company.com',
      reviewed_at: new Date().toISOString(),
    });

    return HttpResponse.json({ success: true });
  }),
];
```

---

## 4. Tiêu chí nghiệm thu (Acceptance Criteria)

### Schema & Data

1. `GET /api/v1/ai/triage/queue` trả về `AITriageQueueResponse` với `items`, `total`, `stats` — HTTP 200.
2. Mỗi item có `ai_result` object với 5 fields: `remarks`, `confidence`, `justification`, `actions`, `generated_at`.
3. `remarks` enum: `Confirmed | FalsePositive | NotAffected | Unexplored`.
4. `confidence` là số thực 0.0–1.0 (không phải 0–100).
5. `human_decision` là `null` nếu chưa review, hoặc `"accepted" | "overridden" | "rejected"`.
6. `reviewed_by` chứa email của người review — không phải user ID.

### Stats

7. `stats.pending` = đúng số items có `human_decision IS NULL` trong DB.
8. `stats.accepted_today` = số items được accept trong ngày hiện tại (UTC 00:00 đến hiện tại).
9. `stats.avg_confidence` = average confidence * 100 (ví dụ: `0.87` → `87`).
10. `stats.false_positive_rate` = % items có `remarks = "FalsePositive"` trên tổng items.

### Filter

11. `?status=pending` chỉ trả items có `human_decision IS NULL`.
12. `?status=accepted` chỉ trả items có `human_decision = 'accepted'`.
13. `?severity=Critical` chỉ trả Critical findings.
14. `?remarks=FalsePositive` chỉ trả items AI verdict là FalsePositive.

### Review

15. `POST /api/v1/ai/triage/{findingId}/review` với `{ decision: "accepted" }` → HTTP 200, `{ success: true }`.
16. `POST /api/v1/ai/triage/{findingId}/review` với `{ decision: "accepted", note: "..." }` lưu note vào DB.
17. `reviewed_by` được extract từ JWT token — không nhận qua request body.
18. Review finding không tồn tại → HTTP 404.
19. Review finding đã reviewed (không có `?force=true`) → HTTP 409.
20. Review finding đã reviewed với `?force=true` → HTTP 200, cập nhật lại.
21. Request với `decision` ngoài enum → HTTP 400.

### DB

22. Sau khi review, `GET /api/v1/ai/triage/queue` phản ánh `human_decision` đã set.
23. `stats.pending` giảm sau mỗi review action.
24. `stats.accepted_today` tăng nếu decision là `"accepted"`.
