# TASK-BE-P0-01 — Seed OpenSearch Index + Fix CVE Search 500

**Phase:** Sprint 1 — P0 Unblock  
**Nguồn giải pháp:** [`solutions/SOL-001_fix-cve-search-500.md`](../solutions/SOL-001_fix-cve-search-500.md)  
**Ưu tiên:** 🔴 P0 — Blocking (CVE search hoàn toàn không dùng được)  
**Phụ thuộc:** Không có  
**Status:** ✅ **DONE** — 2026-06-19

---

## Mục tiêu

Fix `POST /api/v2/cves/search` trả 500. Seed OpenSearch index với dữ liệu CVE từ PostgreSQL. Kích hoạt PostgreSQL GIN fallback khi OpenSearch không sẵn sàng.

---

## Files cần sửa

### [MODIFY] `services/search-service/internal/usecase/search_usecase.go` (hoặc tương đương)

**Tìm file:**
```bash
grep -r "func.*Search\|opensearch\|elasticsearch" \
  services/search-service/internal/ --include="*.go" -l
```

**Thêm dual-backend fallback:**
```go
func (uc *SearchUseCase) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
    // Try OpenSearch first
    if uc.opensearch != nil {
        result, err := uc.opensearch.Search(ctx, req)
        if err == nil {
            return result, nil
        }
        log.Warn().Err(err).Msg("OpenSearch unavailable, falling back to PostgreSQL GIN")
    }
    // Fallback: PostgreSQL full-text search
    return uc.postgres.Search(ctx, req)
}
```

**Cải thiện error response** — phân biệt 500 vs 503:
```go
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
    resp, err := h.uc.Search(r.Context(), req)
    if err != nil {
        var status int
        var msg string
        switch {
        case errors.Is(err, ErrIndexEmpty):
            status = http.StatusServiceUnavailable
            msg = "Search index is being populated. Please retry in a few minutes."
        case errors.Is(err, ErrBackendUnavailable):
            status = http.StatusServiceUnavailable
            msg = "Search backend unavailable"
        default:
            status = http.StatusInternalServerError
            msg = "Internal search error"
        }
        log.Error().Err(err).Msg("search failed")
        respondJSON(w, status, map[string]string{"error": msg})
        return
    }
    respondJSON(w, http.StatusOK, resp)
}
```

---

## Bước thực hiện

### Bước 1 — Kiểm tra trạng thái OpenSearch

```bash
# Kiểm tra OpenSearch có chạy không
curl http://localhost:9200/_cluster/health

# Kiểm tra index vulnerabilities
curl http://localhost:9200/vulnerabilities/_count
# Nếu {"count":0} hoặc 404 → cần seed data
```

### Bước 2 — Tạo OpenSearch index mapping (nếu chưa có)

```bash
curl -X PUT http://localhost:9200/vulnerabilities \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "number_of_shards": 1,
      "number_of_replicas": 0,
      "analysis": {
        "analyzer": {
          "cve_analyzer": {
            "type": "standard"
          }
        }
      }
    },
    "mappings": {
      "properties": {
        "cve_id":        { "type": "keyword" },
        "description":   { "type": "text", "analyzer": "cve_analyzer" },
        "severity_v3":   { "type": "keyword" },
        "cvss_v3_score": { "type": "float" },
        "cvss_v3_vector":{ "type": "keyword" },
        "epss_score":    { "type": "float" },
        "epss_percentile":{ "type": "float" },
        "is_kev":        { "type": "boolean" },
        "is_exploit":    { "type": "boolean" },
        "known_ransomware":{ "type": "boolean" },
        "vendor":        { "type": "keyword" },
        "product":       { "type": "keyword" },
        "published_at":  { "type": "date" },
        "modified_at":   { "type": "date" },
        "data_source":   { "type": "keyword" }
      }
    }
  }'
```

### Bước 3 — Seed NVD API Key (nếu có)

```bash
# Nếu có NVD API key:
# Thêm vào deploy/dev/.env:
NVD_API_KEY=<your-nvd-api-key>

# Restart data-service để trigger STARTUP_SYNC_ENABLED=true:
docker compose -f deploy/dev/docker-compose.server.yml restart data-service
docker logs osv-backend-data-service-1 --tail 100 | grep -i "nvd\|sync\|index"
```

### Bước 4 — Seed dữ liệu thủ công từ PostgreSQL → OpenSearch (nếu NVD key không có)

Tìm script seed hoặc internal sync endpoint:
```bash
# Tìm script seed trong data-service
find services/data-service/scripts -name "*.go" -o -name "*.sh"

# Thử trigger sync qua internal API
curl -X POST http://localhost:8082/internal/sync/opensearch
# hoặc
curl -X POST http://localhost:8082/internal/reindex
```

Nếu không có script, implement migration script:
```go
// services/data-service/scripts/seed_opensearch.go
// Script đọc từ PostgreSQL table cves và bulk index vào OpenSearch
```

### Bước 5 — Fix search-service error handling (code change)

```bash
# Tìm search handler file
grep -r "search failed\|SearchHandler\|HandleSearch" \
  services/search-service/internal/ --include="*.go" -l
```

Sửa handler để phân biệt error types và kích hoạt fallback.

---

## Acceptance Criteria

- [ ] `curl http://localhost:9200/vulnerabilities/_count` trả `{"count": N}` với N > 0
- [ ] `POST /api/v2/cves/search` với body `{"page":1,"page_size":5}` trả HTTP 200
- [ ] Response có field `data` (array), `total`, `page`, `page_size`
- [ ] Khi OpenSearch down, search vẫn hoạt động qua PostgreSQL GIN fallback
- [ ] Error messages rõ ràng (503 khi backend down, không phải 500 generic)

## Verification

```bash
# Test search basic
curl -X POST https://c12.openledger.vn/api/v2/cves/search \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"page":1,"page_size":5}'
# Expected: HTTP 200 { "data": [...], "total": N }

# Test search với keyword
curl -X POST https://c12.openledger.vn/api/v2/cves/search \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"query":"log4j","page":1,"page_size":5}'
# Expected: HTTP 200 với CVE-2021-44228 trong kết quả
```
