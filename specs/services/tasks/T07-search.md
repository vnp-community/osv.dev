# Task T07 — Search Service

> **Priority:** P1 | **Phase:** 3 | **Spec:** `specs/services/08-search-service.md`  
> **Depends on:** T00-shared-libs, T12-infrastructure (NATS, OpenSearch, Redis)

## Mục Tiêu
Full-text search và semantic search (vector) cho vulnerabilities. Thay thế Datastore keyword search cơ bản.

## Trách Nhiệm
- Full-text search: summary, description, package name, ID
- Faceted search: ecosystem, severity, date range filters
- Fuzzy matching, typo tolerance
- Semantic search (vector similarity) — AI Ready
- Autocomplete/type-ahead
- Index maintenance: consume VulnImported/Updated/Withdrawn events
- Search analytics (popular queries, zero-result tracking)

## Không Làm
- Exact ID lookup (Query Service), SEMVER queries (Query Service), store primary data

## Cấu Trúc File

```
services/search/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── search_document.go    # Indexed representation
│   │   │   └── search_result.go
│   │   ├── valueobject/
│   │   │   ├── search_query.go       # Parsed query + filters
│   │   │   ├── facet.go
│   │   │   ├── highlight.go
│   │   │   ├── sort_order.go         # relevance | date_desc | severity
│   │   │   └── search_cursor.go
│   │   ├── service/
│   │   │   ├── query_parser.go       # Parse user query string
│   │   │   └── document_mapper.go   # Vuln → SearchDocument
│   │   └── repository/search_index_repo.go  # Interface
│   ├── application/
│   │   ├── query/
│   │   │   ├── search_vulnerabilities/{query,handler,handler_test}.go
│   │   │   ├── semantic_search/{query,handler}.go
│   │   │   └── autocomplete/{query,handler}.go
│   │   ├── command/
│   │   │   ├── index_vulnerability/{command,handler}.go
│   │   │   └── remove_vulnerability/{command,handler}.go
│   │   └── port/
│   │       ├── vector_search_port.go  # Interface: Embed + Search
│   │       └── analytics_port.go
│   └── infra/
│       ├── opensearch/
│       │   ├── opensearch_adapter.go  # OpenSearch client
│       │   ├── index_mapping.go       # Index schema JSON
│       │   └── query_builder.go       # Build OpenSearch DSL queries
│       ├── vector/
│       │   ├── vertex_ai_adapter.go
│       │   └── qdrant_adapter.go
│       ├── cache/redis/search_cache.go
│       ├── messaging/nats/vuln_event_consumer.go
│       └── analytics/bigquery_writer.go
├── interface/
│   ├── grpc/
│   │   ├── handler/search_handler.go
│   │   └── proto/search_service.proto
│   └── http/handler/health_handler.go
└── config/config.go
```

## OpenSearch Index Mapping

```json
{
  "mappings": {
    "properties": {
      "vuln_id":      { "type": "keyword" },
      "summary":      { "type": "text", "analyzer": "standard",
                        "fields": {
                          "keyword": { "type": "keyword" },
                          "suggest": { "type": "completion" }
                        }},
      "details":      { "type": "text", "analyzer": "standard" },
      "ecosystems":   { "type": "keyword" },
      "packages":     { "type": "keyword" },
      "purls":        { "type": "keyword" },
      "aliases":      { "type": "keyword" },
      "source":       { "type": "keyword" },
      "severity_type": { "type": "keyword" },
      "cvss_score":   { "type": "float" },
      "cvss_severity": { "type": "keyword" },
      "published":    { "type": "date" },
      "modified":     { "type": "date" },
      "is_withdrawn": { "type": "boolean" },
      "description_embedding": {
        "type": "knn_vector", "dimension": 768,
        "method": { "name": "hnsw", "space_type": "cosinesimil", "engine": "nmslib" }
      }
    }
  },
  "settings": {
    "number_of_shards": 3, "number_of_replicas": 1,
    "refresh_interval": "5s", "index.knn": true
  }
}
```

## Proto

```protobuf
service SearchService {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc SemanticSearch(SemanticSearchRequest) returns (SearchResponse);
  rpc Autocomplete(AutocompleteRequest) returns (AutocompleteResponse);
}
message SearchRequest {
  string query = 1; SearchFilters filters = 2;
  SortOrder sort = 3; int32 page_size = 4; string page_token = 5;
}
message SearchFilters {
  repeated string ecosystems = 1; repeated string severities = 2;
  string date_from = 3; string date_to = 4;
  bool withdrawn = 5; repeated string sources = 6;
}
enum SortOrder { SORT_RELEVANCE=0; SORT_DATE_DESC=1; SORT_DATE_ASC=2; SORT_SEVERITY=3; }
message SearchResponse {
  repeated SearchHit hits = 1;
  repeated Facet ecosystems = 2; repeated Facet severities = 3;
  int64 total_hits = 4; string next_page_token = 5; int64 took_ms = 6;
}
message SearchHit {
  string vuln_id; string summary; string ecosystem; string severity;
  string modified; float score; repeated Highlight highlights;
}
message SemanticSearchRequest {
  string query = 1; SearchFilters filters = 2; int32 top_k = 3;
}
message AutocompleteRequest { string prefix = 1; int32 max_results = 2; }
message AutocompleteResponse { repeated string suggestions = 1; }
```

## Search Handler

```go
// application/query/search_vulnerabilities/handler.go
func Handle(ctx, q Query) (*Result, error):
  // 1. Parse query (QueryParser: extract keywords, detect filters)
  // 2. Check Redis cache (TTL 30s for hot queries)
  // 3. Build OpenSearch DSL query (multi_match + filters + sort + pagination)
  // 4. Execute → SearchResponse
  // 5. Track analytics (goroutine: BigQuery)
  // 6. Cache result in Redis
```

## Semantic Search Handler

```go
// application/query/semantic_search/handler.go
func Handle(ctx, q SemanticQuery) (*Result, error):
  // 1. Embed q.Query via VectorSearchPort → []float32
  // 2. k-NN search in OpenSearch (knn_vector field, cosine similarity)
  // 3. Apply filters (post-filter)
  // 4. Fetch full data from OpenSearch by vuln IDs
```

## Index Maintenance (NATS Consumer)

```go
// infra/messaging/nats/vuln_event_consumer.go
// Subscribe "osv.vuln.>" (VulnImported, VulnUpdated, VulnWithdrawn)
// On VulnImported/Updated:
//   1. Fetch full vuln from Query Service (gRPC) hoặc nhận từ event payload
//   2. Map to SearchDocument (DocumentMapper)
//   3. Upsert into OpenSearch (with embedding if available in event)
// On VulnWithdrawn:
//   searchRepo.MarkWithdrawn(ctx, vulnID)  // update is_withdrawn=true
// SLO: index within 30s of event
```

## Caching
```
Redis cache:
  osv:search:cache:{sha256(query+filters+page)} TTL=30s
  osv:autocomplete:{prefix} TTL=60s

Invalidation: none needed (short TTL + eventual consistency ok)
```

## Analytics
```go
// infra/analytics/bigquery_writer.go
// Write search events to BigQuery (async, fire-and-forget)
type SearchEvent struct {
    Query       string    `bigquery:"query"`
    TotalHits   int64     `bigquery:"total_hits"`
    Duration    int64     `bigquery:"duration_ms"`
    SearchedAt  time.Time `bigquery:"searched_at"`
}
```

## SLO Targets
- Search P50: <50ms, P99: <500ms
- Semantic search P50: <200ms
- Index staleness: <30s after VulnUpdated event
- Index coverage: 100% of active vulns

## Checklist Thực Thi

> **Status: ✅ COMPLETED (Core)** — 2026-06-01

- [x] Implement `DocumentMapper` (VulnPayload → SearchDocument)
- [x] Implement `QueryParser` (parse user query, extract ecosystem/severity hints via `ecosystem:`/`severity:` prefix)
- [x] `domain/entity`: SearchDocument (incl. knn_vector field), SearchResult, Highlight, Facet
- [x] `domain/valueobject`: SearchQuery, SortOrder
- [x] `domain/repository`: SearchIndexRepo, SearchCache interfaces
- [x] Implement `SearchVulnerabilitiesHandler` (cache-aside Redis TTL=30s + async analytics)
- [x] Implement `IndexVulnerabilityHandler` (upsert command)
- [x] Implement NATS consumer (`VulnImported/Updated/Withdrawn` → index) — durable, MaxDeliver=5
- [x] `Dockerfile` (multi-stage → distroless)
- [x] `go.mod` + workspace entry
- [ ] Create OpenSearch index mapping JSON config file
- [ ] Implement `OpenSearchAdapter` (Search, Upsert, MarkWithdrawn, GetByIDs)
- [ ] Implement `QueryBuilder` (build OpenSearch DSL from SearchParams: multi_match + filters)
- [ ] Implement `SemanticSearchHandler` (embed → kNN search)
- [ ] Implement `AutocompleteHandler` (completion suggester)
- [ ] Implement `VectorSearchPort` (VertexAI adapter — Embed + Search)
- [ ] Implement Redis cache adapter (sha256 key, TTL=30s for search, TTL=60s for autocomplete)
- [ ] Implement `BigQueryWriter` analytics adapter
- [ ] gRPC handler + proto (`search_service.proto`)
- [ ] Bootstrap: initial full index script (bulk import all existing vulns)
- [ ] Unit tests: QueryParser, DocumentMapper
- [ ] Integration tests: OpenSearch test container + NATS
- [ ] Makefile
