# CR-UI-008 — AI Center API

**Series:** UI-API v2  
**Ngày tạo:** 2026-06-16  
**Cập nhật:** 2026-06-17  
**Trạng thái:** 🟢 Mock Layer Complete / Backend v3.0 Planned  
**Ưu tiên:** P1 — High (v3.0 feature)  
**Nguồn yêu cầu:** `ui/specs/TDD.md` §9, `docs/SRS.md` §3.6, `docs/PRD.md` §4.9  
**Services ảnh hưởng:** `gateway (:8080)`, `ai-service`, `finding-service (:8085)`  
**Dependency:** CR-OVS-005

---

## 1. Bối cảnh

Module AI Center (`/ai/*`) bao gồm 2 screens:
- **AI Triage Queue** (`/ai/triage`): Queue findings cần AI triage, inline accept/override/reject
- **AI Enrichment** (`/ai/enrichment`): CVE embedding status, re-enrich actions

`ai-service` hiện có embedding generation cho pgvector (search-service). CR-OVS-005 sẽ thêm LLM triage cho findings. CR này định nghĩa API cho UI.

---

## 2. Endpoints yêu cầu

### 2.1 POST /api/v1/ai/triage/{findingId}

**Mô tả:** Request AI triage cho một finding. Async — LLM call có thể mất vài giây.

**Auth:** Required (`finding:write`)

**Request Body:** Empty (finding data lấy từ finding-service bởi ai-service)

**Response 200 (đồng bộ, nếu LLM đã cache):**
```json
{
  "finding_id": "F-2847",
  "remarks": "Confirmed",
  "confidence": 0.94,
  "justification": "CVE-2025-44228 has a CVSS score of 10.0, is in CISA KEV, and actively exploited. Component log4j-core 2.14.1 is within the affected version range.",
  "actions": [
    "Update log4j-core to version 2.15.0 or later",
    "Apply WAF rule to block JNDI lookup patterns",
    "Audit all log4j usages in codebase"
  ],
  "generated_at": "2026-06-16T12:00:00Z",
  "ai_provider": "ollama"
}
```

**Response 202 (async — đang processing):**
```json
{
  "finding_id": "F-2847",
  "status": "processing",
  "estimated_ms": 3000
}
```

**Remarks enum:**
- `Confirmed` — finding xác nhận là valid vulnerability
- `FalsePositive` — AI gợi ý có thể false positive
- `NotAffected` — component/version không affected
- `Unexplored` — chưa đủ context để phân tích

**AI Provider Chain:** Ollama → OpenAI → Azure OpenAI (ordered failover)

---

### 2.2 GET /api/v1/ai/triage/queue

**Mô tả:** List findings trong triage queue.

**Auth:** Required (`finding:read`)

**Query Parameters:**
| Param | Type | Default | Mô tả |
|-------|------|---------|-------|
| `status` | string | `pending` | `pending,confirmed,false_positive,not_affected` |
| `severity` | string[] | all | Severity filter |
| `page` | int | 1 | Phân trang |
| `page_size` | int | 20 | Items per page |

**Response 200:**
```json
{
  "queue": [
    {
      "finding_id": "F-2847",
      "finding_title": "Apache Log4j2 JNDI Remote Code Execution",
      "cve_id": "CVE-2025-44228",
      "severity": "Critical",
      "ai_result": {
        "remarks": "Confirmed",
        "confidence": 0.94,
        "justification": "...",
        "actions": ["..."],
        "generated_at": "2026-06-16T12:00:00Z",
        "ai_provider": "ollama"
      },
      "human_decision": null,
      "human_note": null,
      "reviewed_by": null,
      "reviewed_at": null
    }
  ],
  "total": 15,
  "stats": {
    "pending": 15,
    "confirmed": 48,
    "false_positive": 12,
    "not_affected": 8,
    "time_saved_hours": 24.5
  },
  "page": 1,
  "page_size": 20
}
```

---

### 2.3 POST /api/v1/ai/triage/{findingId}/review

**Mô tả:** Human review AI triage result (accept/override/reject).

**Auth:** Required (`finding:write`)

**Request Body:**
```json
{
  "decision": "accepted",
  "note": "Confirmed the issue. Will fix in next sprint."
}
```

| Field | Mô tả |
|-------|-------|
| `decision` | `accepted` — chấp nhận AI suggestion |
| `decision` | `overridden` — override AI với human decision |
| `decision` | `rejected` — bác bỏ AI suggestion |
| `note` | Human note/comment |

**Response 200:**
```json
{
  "finding_id": "F-2847",
  "decision": "accepted",
  "note": "Confirmed the issue.",
  "reviewed_by": "bob@company.com",
  "reviewed_at": "2026-06-16T12:05:00Z"
}
```

**Side effects:**
- Nếu `decision=accepted` và `ai_result.remarks=FalsePositive` → auto-suggest transition `status → false_positive` (UI confirm dialog)
- Publish NATS `ai.triage.reviewed` event

---

### 2.4 GET /api/v1/ai/enrichment

**Mô tả:** CVE enrichment status overview — AI Enrichment screen.

**Auth:** Required (`finding:read`)

**Response 200:**
```json
{
  "stats": {
    "total_cves": 312450,
    "with_embedding": 298000,
    "embedding_coverage_pct": 95.4,
    "last_enrichment_run": "2026-06-16T06:00:00Z",
    "semantic_search_accuracy": 0.82
  },
  "recent_enrichments": [
    {
      "cve_id": "CVE-2026-12345",
      "has_embedding": true,
      "embedding_dims": 1536,
      "is_cached": true,
      "ai_severity": "Critical",
      "ai_provider": "ollama",
      "enriched_at": "2026-06-16T06:00:00Z"
    }
  ],
  "total": 298000
}
```

---

### 2.5 POST /api/v1/ai/enrichment/trigger

**Mô tả:** Trigger enrichment job cho danh sách CVEs hoặc toàn bộ.

**Auth:** Required (`system:configure`)

**Request Body:**
```json
{
  "cve_ids": ["CVE-2026-12345", "CVE-2026-99999"],
  "force_refresh": false
}
```

If `cve_ids` is empty → re-enrich all CVEs without embeddings.

**Response 202:**
```json
{
  "job_id": "enrich_job_001",
  "status": "queued",
  "cve_count": 2
}
```

---

### 2.6 GET /api/v1/ai/enrichment/{cveId}

**Mô tả:** Enrichment status cho một CVE cụ thể.

**Auth:** Required

**Response 200:**
```json
{
  "cve_id": "CVE-2025-44228",
  "has_embedding": true,
  "embedding_dims": 1536,
  "is_cached": true,
  "cache_ttl_seconds": 432000,
  "ai_severity": {
    "severity": "Critical",
    "confidence": 0.98,
    "reasoning": "CVSS 10.0, actively exploited",
    "source": "cvss_v3"
  },
  "ai_provider": "ollama",
  "enriched_at": "2026-06-16T06:00:00Z"
}
```

---

## 3. Data Models

### AITriageResult Object
```json
{
  "remarks": "Confirmed|FalsePositive|NotAffected|Unexplored",
  "confidence": 0.94,
  "justification": "string",
  "actions": ["string"],
  "generated_at": "ISO8601",
  "ai_provider": "ollama|openai|azure"
}
```

### CVEEnrichmentStatus Object
```json
{
  "cve_id": "string",
  "has_embedding": true,
  "embedding_dims": 1536,
  "is_cached": true,
  "ai_severity": "Severity|null",
  "ai_provider": "ollama|openai|azure|null",
  "enriched_at": "ISO8601|null"
}
```

---

## 4. Acceptance Criteria

> **Chú thích:** `[x]` = đã implement (UI mock layer + component); `[ ]` = backend pending (phụ thuộc CR-OVS-005)

- [x] `POST /api/v1/ai/triage/{findingId}` → 200 với triage result hoặc 202 khi processing _(mock: ai.handlers.ts)_
- [x] Triage result có đủ fields: `remarks`, `confidence`, `justification`, `actions` _(mock data)_
- [x] `POST /api/v1/ai/triage/{findingId}/review` với `decision=accepted` → 200 _(mock: ai.handlers.ts)_
- [x] `GET /api/v1/ai/triage/queue` → list với stats (pending, confirmed, time_saved) _(mock: ai.handlers.ts)_
- [x] `GET /api/v1/ai/enrichment` → stats với `embedding_coverage_pct` _(mock: ai.handlers.ts)_
- [x] `POST /api/v1/ai/enrichment/trigger` → 202 job queued _(mock: ai.handlers.ts)_
- [x] LLM failover hoạt động: Ollama → OpenAI → Azure — _(mock)_

---

## 5. Phụ thuộc

| CR | Mô tả |
|----|-------|
| CR-GCV-004 (v1) | CVE embeddings (partial) — đã implement |
| CR-OVS-005 (v2) | LLM triage, AI-service full — planned |
