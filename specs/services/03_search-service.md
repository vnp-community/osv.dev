# search-service

**Bounded Context**: Search & Discovery
**Go Module**: `github.com/osv/search-service`

---

## Merge từ

| Source | Trạng thái |
|--------|-----------|
| `services/search-service` | ✅ Active — base chính |
| `services/query-service` | ✅ Active — merged |
| `services/dd-search` | ✅ Active — merged |
| `archive/cve-search-service` | 📦 Archive — merged |
| `archive/search` | 📦 Archive — merged |
| `archive/query-service-old` | 📦 Archive — merged |
| `archive/vulnerability-query` | 📦 Archive — merged |
| `archive/browse-service` | 📦 Archive — merged |

---

## Chức năng

| # | Chức năng | Mô tả |
|---|-----------|-------|
| 1 | **Full-text Search** | Tìm kiếm CVE theo keyword trong title, description, references |
| 2 | **Faceted Filter** | Lọc theo severity, CVSS score, ecosystem, date range, KEV status |
| 3 | **Advanced Query** | DSL query với AND/OR/NOT logic, field-specific filters |
| 4 | **Aggregations** | Thống kê theo severity distribution, ecosystem, timeline |
| 5 | **Autocomplete** | Gợi ý CVE ID, product names khi gõ |
| 6 | **Semantic Search** | Vector similarity search dùng AI embeddings |
| 7 | **Browse** | Duyệt CVE theo category, ecosystem, vendor |
| 8 | **Statistics** | Dashboard metrics (total CVE, monthly trend, top vendors) |
| 9 | **Index Sync** | Subscribe NATS events từ data-service để update index |

---

## Clean Architecture Layout

```
search-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/                         # ← Business rules
│   │   ├── query/
│   │   │   ├── entity.go               # SearchQuery entity
│   │   │   └── builder.go              # Query builder (DSL → ES query)
│   │   ├── result/
│   │   │   ├── entity.go               # SearchResult, Hit entities
│   │   │   └── aggregation.go          # Aggregation result types
│   │   ├── filter/
│   │   │   ├── severity.go             # Severity filter value object
│   │   │   ├── date_range.go           # Date range filter
│   │   │   ├── ecosystem.go            # Ecosystem filter
│   │   │   └── cvss.go                 # CVSS score range filter
│   │   ├── suggest/
│   │   │   └── entity.go               # Suggestion entity
│   │   └── repository/
│   │       ├── search_repo.go          # SearchRepository interface
│   │       └── index_repo.go           # IndexRepository interface
│   │
│   ├── usecase/                        # ← Application use cases
│   │   ├── search_cve/
│   │   │   ├── usecase.go
│   │   │   └── dto.go
│   │   ├── filter_cve/
│   │   │   ├── usecase.go              # Faceted filter query
│   │   │   └── dto.go
│   │   ├── suggest/
│   │   │   └── usecase.go              # Autocomplete suggestions
│   │   ├── aggregate/
│   │   │   └── usecase.go              # Stats & aggregations
│   │   ├── browse/
│   │   │   └── usecase.go              # Browse by category
│   │   └── index_cve/
│   │       └── usecase.go              # Index/re-index CVE documents
│   │
│   ├── delivery/                       # ← Transport layer
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── search_handler.go       # SearchService RPC impl
│   │   └── http/
│   │       ├── router.go
│   │       ├── search_handler.go
│   │       ├── filter_handler.go
│   │       ├── suggest_handler.go
│   │       ├── aggregate_handler.go
│   │       └── browse_handler.go
│   │
│   ├── infra/                          # ← External systems
│   │   ├── elasticsearch/
│   │   │   ├── client.go               # ES/OpenSearch client
│   │   │   ├── search_repo.go          # SearchRepository impl
│   │   │   ├── index_repo.go           # IndexRepository impl
│   │   │   └── mappings/
│   │   │       └── cve_mapping.json    # ES index mapping
│   │   ├── postgres/
│   │   │   └── stats_repo.go           # Fallback statistics queries
│   │   ├── redis/
│   │   │   └── cache.go                # Query result cache (TTL-based)
│   │   └── nats/
│   │       └── subscriber.go           # Subscribe data.cve.* events
│   │
│   └── factory/
│       └── search_engine.go            # Switch between ES/PG backends
│
├── go.mod
└── Dockerfile
```

---

## Domain Model

### SearchQuery
```go
type SearchQuery struct {
    Keyword    string
    Filters    Filters
    Sort       SortOption
    Pagination Pagination
    Mode       SearchMode   // FULLTEXT | SEMANTIC | HYBRID
}

type Filters struct {
    Severity    []string       // CRITICAL, HIGH, MEDIUM, LOW
    MinCVSS     *float64
    MaxCVSS     *float64
    Ecosystems  []string       // npm, pypi, go, maven, etc.
    DateFrom    *time.Time
    DateTo      *time.Time
    KEVOnly     bool
    HasExploit  bool
    CWEID       []string
    VendorName  string
}

type SearchResult struct {
    Total   int64
    Hits    []Hit
    Facets  Facets
    Took    time.Duration
}

type Hit struct {
    CVEID     string
    Score     float64
    Highlight map[string][]string
    Summary   CVESummary
}
```

### Aggregation
```go
type AggregationResult struct {
    SeverityDist  map[string]int64    // {CRITICAL: 1200, HIGH: 5000}
    EcosystemDist map[string]int64
    MonthlyTrend  []MonthlyCount
    TopVendors    []VendorCount
    TopCWEs       []CWECount
    TotalCVE      int64
    KEVCount      int64
}
```

---

## API Specification

### HTTP REST Endpoints

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| `POST` | `/search` | JWT | Full-text search CVE |
| `POST` | `/search/filter` | JWT | Faceted filter query |
| `POST` | `/search/advanced` | JWT | Advanced DSL query |
| `GET`  | `/search/suggest` | JWT | Autocomplete (q= param) |
| `POST` | `/search/semantic` | JWT | Vector similarity search |
| `POST` | `/search/aggregate` | JWT | Statistics & aggregations |
| `GET`  | `/browse` | JWT | Browse by category |
| `GET`  | `/browse/ecosystem/{name}` | JWT | Browse by ecosystem |
| `GET`  | `/browse/vendor/{name}` | JWT | Browse by vendor |
| `POST` | `/admin/reindex` | Admin | Trigger full re-index |

### Request/Response Examples

```json
// POST /search
{
  "keyword": "log4shell remote code execution",
  "filters": {
    "severity": ["CRITICAL", "HIGH"],
    "ecosystems": ["maven"],
    "date_from": "2021-01-01",
    "kev_only": false
  },
  "sort": {"field": "cvss_score", "order": "desc"},
  "pagination": {"page": 1, "size": 20}
}

// Response
{
  "total": 42,
  "hits": [
    {
      "cve_id": "CVE-2021-44228",
      "score": 9.8,
      "summary": {
        "title": "Apache Log4j2 Remote Code Execution",
        "severity": "CRITICAL",
        "cvss_v31": 10.0,
        "published": "2021-12-10"
      }
    }
  ],
  "facets": {
    "severity": {"CRITICAL": 5, "HIGH": 37},
    "ecosystem": {"maven": 42}
  },
  "took_ms": 23
}
```

### gRPC Services (internal)

```protobuf
service SearchService {
    rpc Search(SearchRequest) returns (SearchResponse);
    rpc Suggest(SuggestRequest) returns (SuggestResponse);
    rpc Aggregate(AggregateRequest) returns (AggregateResponse);
    rpc IndexCVE(IndexCVERequest) returns (IndexCVEResponse);
}
```

---

## Event Subscriptions (NATS)

| Event | Subject | Action |
|-------|---------|--------|
| `CVECreated` | `data.cve.created` | Index new CVE |
| `CVEUpdated` | `data.cve.updated` | Update CVE in index |
| `CVEWithdrawn` | `data.cve.withdrawn` | Remove from index |

---

## Elasticsearch Index Mapping

```json
{
  "mappings": {
    "properties": {
      "cve_id":      {"type": "keyword"},
      "title":       {"type": "text", "analyzer": "english"},
      "description": {"type": "text", "analyzer": "english"},
      "severity":    {"type": "keyword"},
      "cvss_score":  {"type": "float"},
      "ecosystems":  {"type": "keyword"},
      "published_at":{"type": "date"},
      "is_kev":      {"type": "boolean"},
      "cwe_ids":     {"type": "keyword"},
      "embedding":   {"type": "dense_vector", "dims": 1536}
    }
  }
}
```

---

## Dependencies

```
github.com/elastic/go-elasticsearch/v8   # Elasticsearch client
github.com/go-chi/chi/v5                 # HTTP router
github.com/redis/go-redis/v9             # Result cache
github.com/nats-io/nats.go               # NATS subscriber
google.golang.org/grpc                   # gRPC server
github.com/osv/shared/pkg                # Shared utilities
github.com/osv/shared/proto              # gRPC contracts
```

---

## Configuration

```yaml
server:
  http_port: 8083
  grpc_port: 50053

elasticsearch:
  addresses: ["${ES_ADDR}"]
  index: "cve_index"
  username: "${ES_USER}"
  password: "${ES_PASSWORD}"

redis:
  addr: "${REDIS_ADDR}"
  db: 1
  cache_ttl: "5m"

nats:
  url: "${NATS_URL}"
  consumer: "search-service"

search:
  default_size: 20
  max_size: 100
  semantic_dims: 1536
```
