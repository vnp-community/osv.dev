# Service 08 — Search Service

> **Version:** 1.0 | **Status:** Proposed | **Priority:** P1  
> **Language:** Go  
> **Pattern:** CQRS (Query side) + Clean Architecture  
> **Communication:** gRPC (query) + NATS (index events)  
> **Search Engine:** OpenSearch / Elasticsearch

---

## 1. Trách Nhiệm

Service cung cấp **full-text search và semantic search** cho vulnerabilities — thay thế Datastore keyword search cơ bản trong website cũ.

**Responsibilities:**
- Full-text search theo summary, description, package name, ID
- Faceted search: filter theo ecosystem, severity, date range
- Fuzzy matching và typo tolerance
- Semantic search (vector similarity) — AI Ready feature
- Autocomplete / type-ahead suggestions
- Index maintenance: consume VulnImported/VulnUpdated/VulnWithdrawn events
- Search analytics (popular queries, zero-result tracking)

**NOT Responsible for:**
- Exact vulnerability lookup by ID (Query Service)
- SEMVER/commit range queries (Query Service)
- Storing primary data (Ingestion Service)

---

## 2. Clean Architecture Layers

```
Domain:
  ├── SearchDocument entity (indexed representation of vulnerability)
  ├── SearchQuery value object (parsed query with filters)
  ├── SearchResult value object (hit + score + highlights)
  ├── Facet value object (ecosystem, severity counts)
  └── Repository: SearchIndexRepository

Application:
  ├── SearchVulnerabilitiesQuery + Handler    (full-text)
  ├── SemanticSearchQuery + Handler           (vector)
  ├── AutocompleteQuery + Handler
  ├── IndexVulnerabilityCommand + Handler     (write)
  └── RemoveVulnerabilityCommand + Handler    (write)

Infrastructure:
  ├── OpenSearchAdapter (primary search engine)
  ├── NATSConsumer (listen for vuln events)
  ├── VectorSearchAdapter (Vertex AI / Qdrant)
  ├── RedisCache (hot searches)
  └── SearchAnalyticsWriter

Interface:
  ├── gRPC handler (SearchService)
  └── NATS consumer (index updates)
```

---

## 3. Directory Structure

```
services/search/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   │   ├── entity/
│   │   │   ├── search_document.go          # Indexed representation
│   │   │   └── search_result.go
│   │   ├── valueobject/
│   │   │   ├── search_query.go             # Parsed query + filters
│   │   │   ├── facet.go                    # ecosystem/severity counts
│   │   │   ├── highlight.go                # Matched text fragments
│   │   │   ├── sort_order.go               # relevance | date | severity
│   │   │   └── search_cursor.go            # Pagination
│   │   ├── service/
│   │   │   ├── query_parser.go             # Parse user query string
│   │   │   └── document_mapper.go          # Vuln → SearchDocument
│   │   └── repository/
│   │       └── search_index_repo.go        # Interface
│   ├── application/
│   │   ├── query/
│   │   │   ├── search_vulnerabilities/
│   │   │   │   ├── query.go
│   │   │   │   ├── handler.go
│   │   │   │   └── handler_test.go
│   │   │   ├── semantic_search/
│   │   │   │   ├── query.go
│   │   │   │   └── handler.go
│   │   │   └── autocomplete/
│   │   │       ├── query.go
│   │   │       └── handler.go
│   │   ├── command/
│   │   │   ├── index_vulnerability/
│   │   │   │   ├── command.go
│   │   │   │   └── handler.go
│   │   │   └── remove_vulnerability/
│   │   │       ├── command.go
│   │   │       └── handler.go
│   │   └── port/
│   │       ├── vector_search_port.go        # Outbound: vector similarity
│   │       └── analytics_port.go            # Outbound: track analytics
│   └── infra/
│       ├── opensearch/
│       │   ├── opensearch_adapter.go        # OpenSearch client
│       │   ├── index_mapping.go             # Index schema
│       │   └── query_builder.go             # Build OS queries
│       ├── vector/
│       │   ├── vertex_ai_adapter.go         # Vertex AI Vector Search
│       │   └── qdrant_adapter.go            # Qdrant (self-hosted option)
│       ├── cache/
│       │   └── redis/
│       │       └── search_cache.go
│       ├── messaging/
│       │   └── nats/
│       │       └── vuln_event_consumer.go
│       └── analytics/
│           └── bigquery_writer.go
├── interface/
│   ├── grpc/
│   │   ├── handler/
│   │   │   └── search_handler.go
│   │   └── proto/
│   │       └── search_service.proto
│   └── http/
│       └── handler/
│           └── health_handler.go
├── config/config.go
├── Dockerfile
└── go.mod
```

---

## 4. Proto Definition

```protobuf
// proto/search_service.proto
syntax = "proto3";
package osv.search.v1;

service SearchService {
  // Full-text search
  rpc Search(SearchRequest) returns (SearchResponse);
  
  // Semantic / vector search (AI feature)
  rpc SemanticSearch(SemanticSearchRequest) returns (SearchResponse);
  
  // Autocomplete suggestions
  rpc Autocomplete(AutocompleteRequest) returns (AutocompleteResponse);
}

message SearchRequest {
  string query       = 1;   // Free-text query
  SearchFilters filters = 2;
  SortOrder sort     = 3;
  int32 page_size    = 4;   // Default: 20, Max: 100
  string page_token  = 5;
}

message SearchFilters {
  repeated string ecosystems = 1;
  repeated string severities = 2;  // CRITICAL | HIGH | MEDIUM | LOW
  string date_from    = 3;         // RFC3339
  string date_to      = 4;         // RFC3339
  bool   withdrawn    = 5;         // Include withdrawn?
  repeated string sources = 6;     // ghsa, nvd, oss-fuzz...
}

enum SortOrder {
  SORT_RELEVANCE = 0;
  SORT_DATE_DESC = 1;
  SORT_DATE_ASC  = 2;
  SORT_SEVERITY  = 3;
}

message SearchResponse {
  repeated SearchHit hits     = 1;
  repeated Facet ecosystems   = 2;  // Aggregated facets
  repeated Facet severities   = 3;
  int64 total_hits            = 4;
  string next_page_token      = 5;
  int64 took_ms               = 6;
}

message SearchHit {
  string vuln_id     = 1;
  string summary     = 2;
  string ecosystem   = 3;
  string severity    = 4;
  string modified    = 5;
  float  score       = 6;
  repeated Highlight highlights = 7;
}

message Highlight {
  string field    = 1;  // "summary" | "details"
  string fragment = 2;  // HTML with <em> tags
}

message Facet {
  string name  = 1;
  int64  count = 2;
}

message SemanticSearchRequest {
  string query       = 1;  // Natural language query
  SearchFilters filters = 2;
  int32 top_k        = 3;  // Default: 10
}

message AutocompleteRequest {
  string prefix     = 1;   // Partial query
  int32  max_results = 2;  // Default: 5
}

message AutocompleteResponse {
  repeated string suggestions = 1;
}
```

---

## 5. OpenSearch Index Mapping

```json
{
  "mappings": {
    "properties": {
      "vuln_id": { "type": "keyword" },
      "summary": {
        "type": "text",
        "analyzer": "standard",
        "fields": {
          "keyword": { "type": "keyword" },
          "suggest": { "type": "completion" }
        }
      },
      "details": { "type": "text", "analyzer": "standard" },
      "ecosystems": { "type": "keyword" },
      "packages": { "type": "keyword" },
      "purls": { "type": "keyword" },
      "aliases": { "type": "keyword" },
      "source": { "type": "keyword" },
      "severity_type": { "type": "keyword" },
      "cvss_score": { "type": "float" },
      "cvss_severity": { "type": "keyword" },
      "published": { "type": "date" },
      "modified": { "type": "date" },
      "is_withdrawn": { "type": "boolean" },

      "description_embedding": {
        "type": "knn_vector",
        "dimension": 768,
        "method": {
          "name": "hnsw",
          "space_type": "cosinesimil",
          "engine": "nmslib"
        }
      }
    }
  },
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "index.knn": true
  }
}
```

---

## 6. Application — Search Handler

```go
// application/query/search_vulnerabilities/handler.go

type Handler struct {
    searchRepo  repository.SearchIndexRepository
    queryParser *service.QueryParser
    cache       SearchCache
    analytics   port.AnalyticsPort
    tracer      trace.Tracer
}

func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
    ctx, span := h.tracer.Start(ctx, "SearchVulnerabilities")
    defer span.End()
    
    span.SetAttributes(attribute.String("query", q.Query))
    
    // 1. Parse and validate query
    parsed, err := h.queryParser.Parse(q.Query, q.Filters)
    if err != nil {
        return nil, domain.NewValidationError("invalid search query", err)
    }
    
    // 2. Check cache (hot queries)
    cacheKey := parsed.CacheKey()
    if cached, ok := h.cache.Get(ctx, cacheKey); ok {
        span.SetAttributes(attribute.Bool("cache_hit", true))
        return cached, nil
    }
    
    // 3. Execute search
    results, err := h.searchRepo.Search(ctx, &repository.SearchParams{
        Query:     parsed.QueryString,
        Filters:   parsed.Filters,
        SortOrder: q.SortOrder,
        PageSize:  q.PageSize,
        Cursor:    q.PageToken,
    })
    if err != nil {
        return nil, err
    }
    
    // 4. Track analytics
    go h.analytics.TrackSearch(context.Background(), q.Query, results.TotalHits)
    
    // 5. Cache if popular query
    h.cache.Set(ctx, cacheKey, results, 30*time.Second)
    
    return results, nil
}
```

---

## 7. Semantic Search (AI Feature)

```go
// application/query/semantic_search/handler.go

type SemanticHandler struct {
    vectorSearch port.VectorSearchPort  // Vertex AI or Qdrant
    searchRepo   repository.SearchIndexRepository
    tracer       trace.Tracer
}

func (h *SemanticHandler) Handle(ctx context.Context, q SemanticQuery) (*Result, error) {
    ctx, span := h.tracer.Start(ctx, "SemanticSearch")
    defer span.End()
    
    // 1. Generate embedding for query text
    embedding, err := h.vectorSearch.Embed(ctx, q.Query)
    if err != nil {
        return nil, fmt.Errorf("embed query: %w", err)
    }
    
    // 2. k-NN search using cosine similarity
    hits, err := h.vectorSearch.Search(ctx, &port.VectorSearchParams{
        Embedding: embedding,
        TopK:      q.TopK,
        Filters:   q.Filters,
    })
    if err != nil {
        return nil, err
    }
    
    // 3. Hydrate with full data from OpenSearch
    return h.searchRepo.GetByIDs(ctx, extractIDs(hits))
}
```

---

## 8. Index Maintenance

```go
// infra/messaging/nats/vuln_event_consumer.go

// Consumes domain events and maintains search index
func (c *Consumer) handleVulnImported(ctx context.Context, evt *events.VulnImported) error {
    // Fetch full vulnerability from Query Service (or cache)
    vuln, err := c.vulnClient.GetByID(ctx, evt.VulnID)
    if err != nil {
        return err
    }
    
    // Map to search document
    doc := service.MapVulnToSearchDocument(vuln)
    
    // Add embedding if available (from AI Enrichment Service)
    if evt.HasEmbedding {
        doc.Embedding = evt.Embedding
    }
    
    // Upsert into OpenSearch
    return c.searchRepo.Upsert(ctx, doc)
}

func (c *Consumer) handleVulnWithdrawn(ctx context.Context, evt *events.VulnWithdrawn) error {
    return c.searchRepo.MarkWithdrawn(ctx, evt.VulnID)
}
```

---

## 9. SLO Targets

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Search latency P50 | < 50ms |
| Search latency P99 | < 500ms |
| Semantic search P50 | < 200ms |
| Index staleness | < 30s after VulnUpdated event |
| Search result accuracy | > 95% relevance @10 |
| Index coverage | 100% of active vulnerabilities |

---

## 10. Implementation Status

> **Status:** 🔶 Partial — Domain + Application core complete | **Updated:** 2026-06-01

### Implemented
- [x] `domain/entity/entity.go` — SearchDocument (incl. knn_vector field), SearchResult, Highlight, Facet
- [x] `domain/valueobject/valueobject.go` — SearchQuery, SortOrder, SearchCursor
- [x] `domain/service/service.go` — DocumentMapper (VulnPayload→SearchDocument), QueryParser (ecosystem/severity hints extraction)
- [x] `domain/repository/repository.go` — SearchIndexRepo + SearchCache interfaces
- [x] `application/query/search_vulnerabilities/handler.go` — Cache-aside Redis TTL=30s + async analytics
- [x] `application/command/index_vulnerability/handler.go` — Upsert command
- [x] `infra/messaging/nats/consumer.go` — VulnImported/Updated/Withdrawn → index (MaxDeliver=5)
- [x] `Dockerfile`, `go.mod`

### Pending
- [ ] `infra/opensearch/opensearch_adapter.go` — OpenSearch client (Search, Upsert, MarkWithdrawn, GetByIDs)
- [ ] `infra/opensearch/query_builder.go` — OpenSearch DSL query construction
- [ ] `infra/opensearch/index_mapping.go` — Index schema (matches spec JSON mapping)
- [ ] `infra/vector/vertex_ai_adapter.go` — Vertex AI Vector Search for semantic search
- [ ] `application/query/semantic_search/handler.go` — Vector search handler
- [ ] `application/query/autocomplete/handler.go` — Autocomplete handler
- [ ] `infra/cache/redis/search_cache.go` — Redis search result cache
- [ ] `infra/analytics/bigquery_writer.go` — Search analytics
- [ ] `interface/grpc/handler/search_handler.go` — gRPC handler (proto-gen)
- [ ] `cmd/server/main.go` — Entry point
- [ ] Integration tests (OpenSearch testcontainer), Makefile

### Deviations from Spec
- OpenSearch index mapping JSON defined in spec; implementation pending OpenSearch SDK integration
- Vector search uses Vertex AI (not Qdrant) as primary; Qdrant listed as secondary option
