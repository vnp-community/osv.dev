# SPRINT-07 — Search Enhancement

> **Thời gian:** Q1 2027, Tháng 8 (3 tuần)  
> **Mục tiêu:** Semantic search, faceted search, saved alerts  
> **Refs:** [04-roadmap.md §2.8](../04-roadmap.md), [06-new-features.md §4](../06-new-features.md)

---

## Tổng Quan

```
Sprint Goal: "User có thể tìm CVE bằng ngôn ngữ tự nhiên và subscribe alerts"

Deliverables:
  1. Semantic/Vector search — k-NN OpenSearch
  2. Faceted search với aggregations
  3. Saved searches + alert subscriptions
  4. Search index mapping update (KEV, EPSS, tags)
```

---

## TASK-07-01 · Update OpenSearch Index Mapping [✅ DONE]

**Status:** ✅ Hoàn thành 2026-06-03  
**Effort:** 1 ngày  
**Priority:** P0 (prerequisite cho tất cả search features)  
**Files:**
- [entity.go](../../../../services/search/internal/domain/entity/entity.go)

### Đã implement
- [x] `SearchDocument` entity với tất cả enrichment fields:
  - `KEV bool`, `KEVDateAdded`, `KEVDueDate`, `KEVRansomware`
  - `EPSSScore float32`, `EPSSPercentile float32`, `EPSSDate string`
  - `Tags []string`, `CWEIDs []string`, `ExploitAvailable bool`
  - `Similarity float32` (khi vector search trả về)
- [x] `OpenSearchIndexMapping` constant — JSON mapping cho k-NN index
  - `knn_vector` field với dimension=1536, engine=nmslib, space=cosinesimil
  - Tất cả keyword, float, boolean, date fields
- [x] `FacetedSearchResult` — wrapper cho faceted search với `FacetMap`
- [x] `FacetMap` type — `map[string]map[string]int` cho aggregations


### Subtasks

- [ ] Tìm hiểu OpenSearch index trong `services/search/`
- [ ] Tạo migration script: thêm fields vào existing index
- [ ] Test migration trên dev environment
- [ ] Re-index nếu cần thiết (zero-downtime với alias swap)

---

## TASK-07-02 · Semantic/Vector Search [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 3 ngày  
**Priority:** P1  
**Refs:** [06-new-features.md §4.1](../06-new-features.md)

### Architecture
```
Query: "HTTP/2 rapid reset attack"
  → ai-enrichment generates embedding (1536-dim vector)
  → search service k-NN query OpenSearch
  → Rerank kết quả by BM25 text score + cosine similarity
  → Return top-K results
```

### Subtasks

#### TASK-07-02a · Embedding Pipeline (ai-enrichment side) [✅ DONE]
- [ ] Ensure mỗi CVE enriched record có `embedding` field trong OpenSearch
- [ ] Store embedding khi VectorEmbedding stage hoàn thành
- [ ] Handle embedding updates khi summary/description thay đổi

#### TASK-07-02b · Query Embedding [✅ DONE]
- [ ] `services/search/internal/application/semantic_search.go`
- [ ] `SemanticSearchHandler`:
  1. Nhận user query string
  2. Call ai-enrichment gRPC để generate embedding cho query
  3. k-NN search OpenSearch với embedding
  4. Rerank bằng BM25
  5. Return results

#### TASK-07-02c · API Endpoint [✅ DONE]
```
POST /v1/search/semantic
{
  "query": "HTTP/2 rapid reset",
  "k": 10,
  "min_score": 0.7,
  "filter": {
    "ecosystem": ["Go"],
    "min_severity": "HIGH",
    "kev": true
  }
}
```
- [ ] Add endpoint trong search service
- [ ] Input validation
- [ ] Response format consistent với existing search

---

## TASK-07-03 · Faceted Search & Aggregations [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 2 ngày  
**Priority:** P1  
**Refs:** [06-new-features.md §4.2](../06-new-features.md)

### Subtasks

- [ ] `POST /v1/search/faceted` endpoint
- [ ] Implement filters:
  - `ecosystem: ["Java", "Python"]`
  - `severity: ["CRITICAL", "HIGH"]`
  - `kev: true/false`
  - `has_fix: true/false`
  - `epss_min: 0.5`
  - `year: 2023`
  - `tags: ["impact:rce"]`
- [ ] Implement sort: `epss_score:desc`, `published:desc`, `cvss_score:desc`
- [ ] Implement aggregations:
  - `by_ecosystem` — count per ecosystem
  - `by_severity` — count per severity tier
  - `by_year` — CVE count per year
  - `by_source` — count per source
  - `epss_histogram` — distribution buckets
- [ ] Response:
```json
{
  "hits": [...],
  "total": 1234,
  "aggregations": {
    "by_ecosystem": {"PyPI": 234, "Go": 123},
    "by_severity": {"CRITICAL": 45, "HIGH": 678}
  }
}
```

---

## TASK-07-04 · Saved Searches & Alerts [✅ DONE]

**Status:** ✅ Hoàn thành (2026-06-03)  
**Effort:** 4 ngày  
**Priority:** P2  
**Refs:** [06-new-features.md §4.3](../06-new-features.md)

### Subtasks

#### TASK-07-04a · Saved Search CRUD [✅ DONE]
- [ ] API:
  ```
  POST   /v1/saved-searches              — Create
  GET    /v1/saved-searches              — List
  GET    /v1/saved-searches/{id}         — Get
  PUT    /v1/saved-searches/{id}         — Update
  DELETE /v1/saved-searches/{id}         — Delete
  ```
- [ ] Storage: Firestore `saved_searches` collection
- [ ] Schema:
```go
type SavedSearch struct {
    ID          string
    Name        string
    Query       SearchQuery
    Channels    []string  // ["slack:webhook-url", "email:user@org.com"]
    MinSeverity string
    KEVOnly     bool
    Ecosystems  []string
    CreatedAt   time.Time
    LastAlerted *time.Time
}
```

#### TASK-07-04b · Match Engine [✅ DONE]
- [ ] Khi CVE mới được ingest → check tất cả saved searches
- [ ] Implement efficient matching (không query OpenSearch mỗi CVE)
- [ ] Approach: Match theo filters (severity, ecosystem, kev) rồi text match
- [ ] Batch: Nhóm alerts, gửi tối đa 1 lần/phút per saved search

#### TASK-07-04c · Notification Integration [✅ DONE]
- [ ] Publish `notification.alert.saved_search` NATS event
- [ ] notification-service subscribes và gửi:
  - Slack: rich attachment với CVE details
  - Email: HTML email với CVE summary
  - Webhook: POST JSON payload đến user URL

---

## Sprint 07 Definition of Done

- [ ] OpenSearch có embedding field cho tất cả CVEs
- [ ] Semantic search trả về results với cosine similarity score
- [ ] Faceted search hỗ trợ đầy đủ filters và aggregations
- [ ] Saved search CRUD hoạt động
- [ ] Alert gửi trong < 5 phút sau CVE mới match
- [ ] `go build ./services/search/...` pass
- [ ] `go test ./services/search/...` pass
