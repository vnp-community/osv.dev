# F15 — AI Enrichment: Data Flow

---

## 1. CVE Embedding Generation Pipeline

```
[NATS: ingestion.cve.synced {source, count, synced_at}]
    │
    ▼
ai-service subscriber:
    Fetch newly synced CVE IDs từ data-service
    Queue embedding generation (goroutine pool, rate-limited)
    │
    ▼
For each CVE:
    1. Check Redis cache: osv:embed:{cve_id}
       [HIT] → skip (already done)
       [MISS] → continue
    │
    2. Build embedding input text:
       "{cve_id} {description} {cwe_ids} {vendors}"
    │
    3. Call Ollama (local):
       POST http://ollama:11434/api/embed
       [Success] → embedding vector
       [Fail] → try OpenAI API
    │
    4. Store:
       UPDATE cves SET embedding=$1 WHERE cve_id=$2  (pgvector)
       SET Redis: osv:embed:{cve_id} TTL 7d
    │
    5. Log: {cve_id, provider, dims, duration_ms}
```

---

## 2. Semantic Search Flow

```
Client → POST /api/v2/cves/search/semantic {query: "..."}
    │
    ▼
search-service:
    1. hash = SHA256(query)
       GET Redis: osv:embed:query:{hash}
       [HIT] → skip to step 4
       [MISS] → continue
    │
    2. Call ai-service.embed(query)
       → ai-service calls Ollama/OpenAI
       → return []float32 (1536 dims)
    │
    3. SET Redis: osv:embed:query:{hash} TTL 1h
    │
    4. pgvector query:
       SELECT cve_id, description, severity, epss_score,
              1 - (embedding <=> $vector) as similarity
       FROM cves
       WHERE 1-(embedding<=>$vector) > 0.7
       ORDER BY similarity DESC LIMIT 20
    │
    5. Enrich results (EPSS, KEV flags, CWE)
    │
    ▼
Client ← 200 {cves: [{...similarity_score}], total}
```

---

## 3. [Planned] Finding Triage Flow

```
Client → POST /api/v1/ai/triage/{finding_id}
    │
    ▼
ai-service:
    1. Fetch finding details
    2. Build prompt từ finding context
    3. Try LLM providers in order: Ollama → OpenAI → Azure
       │
       ├── [Success] → parse JSON response
       └── [All fail] → return 503 "AI unavailable"
    │
    4. INSERT finding_triage record (advisory, not enforced)
    │
    ▼
Client ← 200 {
    remarks: "Confirmed",
    confidence: 0.92,
    justification: "Log4j 2.14.1 is confirmed vulnerable...",
    suggested_actions: ["Apply patch to 2.15.0+", "Add WAF rule"]
}
```

---

## 4. NATS Events Consumed

| Event | Action in ai-service |
|-------|---------------------|
| `ingestion.cve.synced` | Queue embedding generation cho newly synced CVEs |
| `ingestion.cve.updated` | Invalidate Redis cache + regenerate embedding |
