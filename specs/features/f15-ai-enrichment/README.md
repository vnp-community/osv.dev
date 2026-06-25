# F15 — AI Enrichment & Triage

> **Spec Folder:** `specs/features/f15-ai-enrichment/`  
> **Feature Doc:** [`docs/features/F15-ai-enrichment.md`](../../../docs/features/F15-ai-enrichment.md)  
> **SRS Refs:** FR-06-01 → FR-06-03  
> **Status:** ✅ Partial (embeddings) | 🔵 Planned (LLM triage)

---

## Sub-documents

| File | Nội dung |
|------|---------|
| [business-logic.md](./business-logic.md) | Embedding generation, LLM triage, provider chain, caching |
| [dataflow.md](./dataflow.md) | Embedding pipeline, semantic search enrichment, triage flow |

---

## Services

| Service | Port | Role |
|---------|------|------|
| `ai-service` | internal | Embedding generation, LLM provider chain |
| `search-service` | internal | Uses embeddings for semantic search |

---

## AI Capabilities

| Feature | Status | Tech |
|---------|--------|------|
| CVE Embeddings | ✅ Implemented | Ollama/OpenAI → pgvector |
| Semantic CVE Search | ✅ Implemented | pgvector cosine similarity |
| Finding Triage (LLM) | 🔵 Planned | Ollama → OpenAI → Azure chain |
| EPSS-based Prioritization | ✅ Implemented | FIRST.org EPSS scores |

---

## Quick Reference: API Endpoints

| Method | Endpoint | Mô tả |
|--------|----------|-------|
| POST | `/api/v2/cves/search/semantic` | Semantic CVE search |
| POST | `/api/v1/ai/triage/{finding_id}` | AI triage (Planned) |
| POST | `/api/v1/ai/embed` | Generate embedding (internal) |

---

## Embedding Config

| Parameter | Value |
|-----------|-------|
| Model | `text-embedding-ada-002` (OpenAI) / `nomic-embed-text` (Ollama) |
| Dimensions | 1536 |
| Storage | `cves.embedding vector(1536)` (pgvector) |
| Cache | Redis key `osv:embed:{cve_id}`, TTL 7 days |
| Index | IVFFlat (pgvector) |

---

## LLM Provider Chain (Planned)

```
Ollama (local) → OpenAI → Azure OpenAI

Fallback:
    if Ollama fails: try OpenAI
    if OpenAI fails: try Azure OpenAI
    if all fail: return error "AI triage unavailable"
```
