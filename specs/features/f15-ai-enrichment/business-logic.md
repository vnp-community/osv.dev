# F15 — AI Enrichment: Business Logic

---

## 1. CVE Embedding Generation

### 1.1 Khi nào generate embedding

```
Hai trigger:
    1. Sau khi CVE được upsert (NATS: ingestion.cve.synced)
       ai-service nhận → queue embedding generation cho CVEs mới
    
    2. Manual API call (internal):
       POST /api/v1/ai/embed {cve_id}
```

### 1.2 Input preparation

```
buildEmbeddingInput(cve):
    text = cve.cve_id + " " + 
           cve.description + " " +
           (cve.cwe_ids.join(" ") if cve.cwe_ids else "") + " " +
           (cve.vendors.join(" ") if cve.vendors else "")
    
    return truncate(text, max_tokens=8192)  // model token limit
```

### 1.3 Generate và cache

```
generateEmbedding(cve_id):
    // Check cache
    cached = Redis.GET("osv:embed:{cve_id}")
    if cached: return cached
    
    // Prepare input
    cve = DB.getCVE(cve_id)
    text = buildEmbeddingInput(cve)
    
    // Call LLM provider (với fallback)
    embedding = callEmbeddingModel(text)  // []float32 (1536 dims)
    
    // Store
    DB.UPDATE cves SET embedding = embedding WHERE id = cve_id
    Redis.SET("osv:embed:{cve_id}", embedding, TTL=7d)
    
    return embedding
```

### 1.4 LLM Provider cho Embedding

```
callEmbeddingModel(text):
    // Try Ollama (local) first
    try:
        result = OllamaClient.embed(model="nomic-embed-text", input=text)
        return result.embedding  // 768 dims (khác OpenAI)
    except error:
        log warning "Ollama unavailable, try OpenAI"
    
    // Fallback OpenAI
    try:
        result = OpenAIClient.embed(model="text-embedding-ada-002", input=text)
        return result.data[0].embedding  // 1536 dims
    except error:
        log error "All embedding providers failed"
        return null
```

---

## 2. Semantic Search

### 2.1 Query embedding

```
POST /api/v2/cves/search/semantic {query: "buffer overflow in web authentication"}

1. Check cache: Redis.GET("osv:embed:query:{SHA256(query)}")
   [HIT] → use cached
   [MISS] → callEmbeddingModel(query)
             Redis.SET("osv:embed:query:{hash}", embedding, TTL=1h)

2. pgvector similarity search:
    SELECT cve_id, description, severity, epss_score,
           1 - (embedding <=> $1::vector) as similarity
    FROM cves
    WHERE embedding IS NOT NULL
      AND 1 - (embedding <=> $1::vector) > 0.7
    ORDER BY similarity DESC
    LIMIT 20
```

### 2.2 Similarity thresholds

```
Similarity score interpretation:
    >= 0.9: rất liên quan — gần như cùng concept
    0.7-0.9: liên quan — semantic match
    0.5-0.7: có liên quan một phần
    < 0.5: không include trong kết quả
```

---

## 3. [Planned] LLM Finding Triage

### 3.1 Input

```
POST /api/v1/ai/triage/{finding_id}

Input được build từ:
{
    finding_id:  "fnd-123",
    title:       "Remote Code Execution via Log4j",
    description: "...",
    cve_id:      "CVE-2021-44228",
    severity:    "Critical",
    cvss:        10.0,
    epss:        0.975,
    is_kev:      true,
    component:   "log4j-core:2.14.1",
    context:     "Java web application, internet-facing, handles customer data"
}
```

### 3.2 LLM Prompt

```
System prompt:
    "You are a security expert. Analyze the vulnerability and determine:
    1. Is this a confirmed vulnerability (True Positive)?
    2. Could this be a False Positive?
    3. What is the actual risk given the context?
    Respond ONLY in JSON."

User prompt:
    "Finding: {title}
    CVE: {cve_id}, CVSS: {cvss}, EPSS: {epss}, KEV: {is_kev}
    Component: {component}
    Context: {context}
    
    Respond with JSON:
    {
        remarks: 'Confirmed' | 'FalsePositive' | 'NotAffected',
        confidence: 0.0-1.0,
        justification: 'brief explanation',
        suggested_actions: ['action1', 'action2']
    }"
```

### 3.3 LLM Provider Chain

```
callLLMTriage(prompt):
    providers = [OllamaProvider, OpenAIProvider, AzureOpenAIProvider]
    
    for provider in providers:
        try:
            response = provider.complete(prompt, max_tokens=500, temperature=0.1)
            result = JSON.parse(response.text)
            validate_schema(result)
            return result
        except error:
            log warning "provider {name} failed: {error}"
            continue
    
    return error("all LLM providers unavailable")
```

### 3.4 Triage Result Usage

```
Sau khi nhận triage result:
    INSERT finding_triage {
        finding_id,
        remarks: result.remarks,
        confidence: result.confidence,
        justification: result.justification,
        suggested_actions: result.suggested_actions,
        triaged_by: "ai",
        provider: provider_name,
        triaged_at: NOW()
    }
    
    // Không tự động thay đổi finding state — chỉ là suggestion
    // Human reviewer phải approve
```

---

## 4. Business Rules

| Rule | Chi tiết |
|------|---------|
| Embedding fallback | Ollama → OpenAI, if all fail → CVE stored without embedding |
| Embedding not blocking | Nếu embed fail, CVE vẫn được lưu bình thường |
| Triage is advisory | AI triage chỉ là gợi ý — không tự động thay đổi finding state |
| Cache embeddings | Redis TTL 7 ngày để tránh gọi API mỗi lần |
| Dimension mismatch | Ollama (768 dims) vs OpenAI (1536 dims) — cần consistent provider |
| Query embedding cache | TTL 1h cho query vectors (ít quan trọng hơn CVE vectors) |
