# TASK-P2-004 — Wire OpenSearch Client cho search-service

**Bug:** MOCK-009, MOCK-010  
**Priority:** 🟡 P2 — Feature bị disabled  
**Effort:** ~1 giờ  
**Service:** `search-service`  
**Loại thay đổi:** Sửa embedded.go + Config

---

## Mục tiêu

1. **MOCK-009**: `cvesearch.New(..., nil, ...)` — OpenSearch bị bypass, chỉ dùng Postgres FTS
2. **MOCK-010**: `InternalHandler` nil — `/internal/opensearch/index` bị tắt

---

## Preconditions

- [ ] Đọc `services/search-service/embedded.go` — xác định chính xác dòng `osClient = nil`
- [ ] Xem OpenSearch client package đang dùng:
  ```bash
  grep -r "opensearch\|opensearchservice" services/search-service/go.mod
  grep -rn "opensearch.NewClient\|\"opensearch\"" services/search-service/internal/
  ```
- [ ] Xác định `InternalHandler` constructor:
  ```bash
  grep -n "func NewInternalHandler" services/search-service/internal/delivery/http/
  ```

---

## Steps

### Step 1 — Xác định OpenSearch package đang dùng

```bash
# Kiểm tra import paths
grep -rn "opensearch" services/search-service/internal/ | grep "import" | head -5
# Xem client struct được define ở đâu
grep -rn "type.*Client\|OpenSearchClient\|osClient" services/search-service/internal/infra/ | head -10
```

### Step 2 — Sửa embedded.go: Wire OpenSearch client

Mở `services/search-service/embedded.go`.

Tìm dòng pass `nil` làm osClient:
```bash
grep -n "nil\|osClient\|opensearch" services/search-service/embedded.go
```

Thêm đoạn init OpenSearch trước khi tạo `searchUC`:

```go
// FIX MOCK-009: Wire OpenSearch client khi OPENSEARCH_URL được cấu hình
var osClient <TypeCụThể>  // dùng đúng type trong codebase

osURL := os.Getenv("OPENSEARCH_URL")
if osURL != "" {
    // Tạo client theo package đang dùng
    // Ví dụ với opensearch-go:
    client, err := opensearch.NewClient(opensearch.Config{
        Addresses: []string{osURL},
        Username:  os.Getenv("OPENSEARCH_USERNAME"),
        Password:  os.Getenv("OPENSEARCH_PASSWORD"),
    })
    if err != nil {
        log.Warn().Err(err).Str("url", osURL).
            Msg("search-service: OpenSearch init failed, falling back to Postgres FTS")
    } else {
        osClient = client
        log.Info().Str("url", osURL).Msg("search-service: OpenSearch enabled")
    }
} else {
    log.Info().Msg("search-service: OPENSEARCH_URL not set, using Postgres FTS only")
}

// Wire searchUC với osClient (nil-safe — SearchUseCase tự fallback sang Postgres)
searchUC := cvesearch.New(cveRepo, cacheRepo, osClient, log)
```

### Step 3 — Wire InternalHandler

Tìm dòng InternalHandler trong embedded.go:
```bash
grep -n "InternalHandler\|internalH\|nil.*statsH\|nil.*internalH" services/search-service/embedded.go
```

Sửa:
```go
// FIX MOCK-010: Wire InternalHandler khi osClient available
var internalH *deliveryhttp.InternalHandler
if osClient != nil {
    internalH = deliveryhttp.NewInternalHandler(osClient, log)
    log.Info().Msg("search-service: OpenSearch indexing routes enabled")
}

// Wire router — nil-safe (router đã có guard cho internalH nil)
router := deliveryhttp.NewRouter(h, taxH, vendorH, internalH, statsH, log)
```

### Step 4 — Xác nhận router có nil-check cho internalH

```bash
grep -n "internalH\|nil\|InternalHandler" services/search-service/internal/delivery/http/router.go
```

Nếu router chưa có nil-check cho `internalH`, thêm:
```go
// Trong router setup
if internalH != nil {
    r.Post("/internal/opensearch/index", internalH.IndexDocument)
    r.Post("/internal/opensearch/bulk", internalH.BulkIndex)
}
```

### Step 5 — Cập nhật docker-compose

```bash
grep -n "OPENSEARCH" deploy/dev/docker-compose*.yml
```

Thêm nếu chưa có:
```yaml
environment:
  - OPENSEARCH_URL=http://opensearch:9200
  - OPENSEARCH_USERNAME=admin
  - OPENSEARCH_PASSWORD=${OPENSEARCH_PASSWORD:-admin}
```

---

## Acceptance Criteria

- [ ] Khi `OPENSEARCH_URL` được set và OpenSearch chạy → search dùng OpenSearch BM25
- [ ] Khi `OPENSEARCH_URL` không set → search fallback sang Postgres FTS (không crash)
- [ ] Khi `osClient != nil` → `/internal/opensearch/index` hoạt động
- [ ] `go build ./services/search-service/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/search-service/...
go vet ./services/search-service/...

# Verify osClient nil removed
grep -n "osClient\|nil.*osClient\|cvesearch.New" services/search-service/embedded.go

go test ./services/search-service/... -v -run Search
```
