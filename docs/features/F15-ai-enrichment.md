# F15 — AI Enrichment & Triage

**Status:** 🔵 Planned v3.0 (embeddings partial in v2.2)  
**CR References:** CR-OVS-005, CR-GCV-004  
**Services:** `ai-service-ovs` (HTTP: 8052, gRPC: 50052)  
**UI Routes:** `/ai/triage`, `/ai/enrichment`  
**UI Components:** `AITriage`, `AIEnrichment`

---

## 1. Mô tả

AI Center cung cấp hai tính năng AI chính: (1) **CVE Embedding** cho semantic search và (2) **AI-powered Finding Triage** giúp security analysts phân loại findings nhanh hơn với LLM recommendations. LLM provider chain đảm bảo high availability với fallover tự động.

---

## 2. CVE Embedding Generation

### 2.1 Tổng quan
- **Status:** ✅ Partial (search-service, v2.2)
- **Model:** OpenAI text-embedding-ada-002 / Ollama local embedding
- **Dimensions:** 1536 dims
- **Storage:** pgvector `cves.embedding` column

### 2.2 Flow
```
ingestion.cve.synced (NATS)
    → ai-service receives event
    → Check Redis cache: osv:embed:{cve_id}
    → Cache MISS: call embedding model
    → Store in pgvector (cves.embedding)
    → Store in Redis cache (TTL 7 days)
    → Publish: ai.cve.enriched (NATS)
```

### 2.3 Cache Strategy
- **Redis key:** `osv:embed:{cve_id}`
- **TTL:** 7 ngày
- **Cache HIT latency:** < 10ms
- **Cache MISS:** Call embedding model (~200ms)

### 2.4 Enrichment Status API
```
GET /api/v1/ai/enrichment/status         → Overall embedding coverage
GET /api/v1/ai/enrichment/cves/{id}      → Status cho specific CVE
POST /api/v1/ai/enrichment/cves/{id}     → Force re-embed một CVE
```

**Status Response:**
```json
{
  "total_cves": 320000,
  "embedded": 310000,
  "coverage_percent": 96.8,
  "last_batch_run": "2026-06-18T06:00:00Z",
  "pending_queue": 10000
}
```

---

## 3. AI Finding Triage

### 3.1 Tổng quan
AI-assisted triage giúp analysts phân loại findings nhanh hơn. LLM phân tích context và đề xuất recommendation — **human review bắt buộc**, AI chỉ là gợi ý.

### 3.2 Triage Input
```json
{
  "finding_id": "finding-001",
  "title": "Log4Shell RCE in authentication service",
  "description": "Remote code execution via JNDI lookup in log4j-core 2.14.1",
  "cve_id": "CVE-2021-44228",
  "severity": "CRITICAL",
  "component": "log4j-core:2.14.1",
  "context": {
    "service_name": "auth-service",
    "internet_facing": true,
    "has_waf": false,
    "environment": "production"
  }
}
```

### 3.3 Triage Output (LLM)
```json
{
  "triage_id": "triage-001",
  "finding_id": "finding-001",
  "remarks": "Confirmed",
  "confidence": 0.95,
  "justification": "CVE-2021-44228 is a critical RCE. log4j-core 2.14.1 is in the vulnerable range. Service is internet-facing without WAF protection.",
  "actions": [
    "Upgrade log4j-core to 2.17.1 or later",
    "Apply temporary mitigation: -Dlog4j2.formatMsgNoLookups=true",
    "Monitor for exploitation attempts in logs"
  ],
  "status": "pending_review",
  "created_at": "2026-06-18T10:00:00Z"
}
```

### 3.4 Triage Remarks
| Remark | Mô tả |
|--------|-------|
| `Confirmed` | Vulnerability confirmed, cần xử lý |
| `FalsePositive` | Không thực sự vulnerable trong context này |
| `NotAffected` | Vulnerable code path không reachable |

### 3.5 Triage Status Flow
```
pending_review → accepted → applied_to_finding
              → rejected  (analyst disagrees)
```

### 3.6 Triage APIs
```
POST /api/v1/ai/triage/{findingId}          → Request triage
GET /api/v1/ai/triage/{findingId}           → Get triage result
PATCH /api/v1/ai/triage/{triageId}/review   → Accept/Reject recommendation
GET /api/v1/ai/triage/queue                 → Pending triage queue
```

---

## 4. LLM Provider Chain

**Ordered failover:**
```
Ollama (local) → OpenAI GPT-4 → Azure OpenAI
```

### 4.1 Provider Selection Logic
1. Try Ollama (local inference, free, private)
2. If unavailable/timeout → Try OpenAI
3. If unavailable/quota exceeded → Try Azure OpenAI
4. If all fail → Return `status: unavailable`

### 4.2 Provider Configs
```json
{
  "providers": [
    {
      "name": "ollama",
      "url": "http://ollama:11434",
      "model": "llama3.2",
      "timeout_seconds": 30
    },
    {
      "name": "openai",
      "api_key": "[env: OPENAI_API_KEY]",
      "model": "gpt-4o-mini",
      "timeout_seconds": 15
    },
    {
      "name": "azure_openai",
      "endpoint": "[env: AZURE_OPENAI_ENDPOINT]",
      "api_key": "[env: AZURE_OPENAI_KEY]",
      "deployment": "gpt-4"
    }
  ]
}
```

---

## 5. AI Center UI

### 5.1 AITriage (`/ai/triage`)
- Queue view: Findings chờ triage
- Triage detail: Show LLM recommendation
- Accept/Reject buttons với comment field
- History: Applied triages

### 5.2 AIEnrichment (`/ai/enrichment`)
- Embedding coverage gauge (96.8%)
- Embedding queue progress
- Manual re-embed trigger per CVE
- Provider status (Ollama / OpenAI / Azure)

---

## 6. NATS Events

| Event | Publisher | Subscribers |
|-------|-----------|------------|
| `ai.cve.enriched` | ai-service | search-service (update pgvector index) |
| `ingestion.cve.synced` | data-service | ai-service (trigger embedding) |

---

## 7. Non-Functional Requirements

| NFR | Target |
|-----|--------|
| Embedding cache hit | < 10ms |
| Embedding generation (cache miss) | < 500ms |
| LLM triage response | < 30 giây (Ollama), < 10 giây (OpenAI) |
| Embedding coverage | > 95% của CVE database |
| Triage: human review required | Tất cả AI recommendations |
| Privacy | Ollama local inference mặc định |
