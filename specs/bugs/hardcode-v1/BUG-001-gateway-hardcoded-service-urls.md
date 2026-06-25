# BUG-001 — Gateway: Hardcoded localhost service URLs in embedded.go

## Metadata
- **ID**: BUG-001
- **Service**: `gateway-service`
- **File**: [`embedded.go`](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/embedded.go)
- **Lines**: 56–103 (also 145, 97)
- **Severity**: High
- **Category**: Hardcode / Configuration
- **Status**: Open

## Mô tả

Trong `gateway-service/embedded.go`, hàm `WireEmbedded` sử dụng các địa chỉ HTTP
`localhost:PORT` được hardcode trực tiếp trong source code, đặc biệt là các dòng:

```go
// Lines 56–64: fallback defaults cho EmbeddedConfig
identityHTTP     := coalesce(cfg.IdentityAddr, "http://localhost:8081")
dataHTTP         := coalesce(cfg.DataAddr, "http://localhost:8082")
findingHTTP      := coalesce(cfg.FindingAddr, "http://localhost:8085")
scanHTTP         := coalesce(cfg.ScanAddr, "http://localhost:8088")
notificationHTTP := coalesce(cfg.NotificationAddr, "http://localhost:8087")
assetHTTP        := coalesce(cfg.AssetAddr, "http://localhost:8091")
productHTTP      := coalesce(cfg.ProductAddr, "http://localhost:8092")
slaHTTP          := coalesce(cfg.SLAAddr, "http://localhost:8093")
aiHTTP           := coalesce(cfg.AIAddr, "http://localhost:8089")

// Lines 81, 97, 145: search-service KHÔNG được đọc từ cfg, LUÔN là localhost:8083
"search-service": "http://localhost:8083",
"search":         "http://localhost:8083",
"search":         "http://localhost:8083",
```

**Vấn đề nghiêm trọng**: `search-service` hoàn toàn thiếu trong `EmbeddedConfig` — không
có field `SearchAddr` — nên nó **không thể** được override bởi caller và sẽ luôn trỏ
đến `localhost:8083` dù môi trường là staging, production hay container.

## Root Cause

`EmbeddedConfig` struct thiếu field `SearchAddr`:

```go
type EmbeddedConfig struct {
    JWTSecret        string
    IdentityAddr     string
    DataAddr         string
    SearchAddr       string  // <-- THIẾU
    FindingAddr      string
    ScanAddr         string
    NotificationAddr string
    AIAddr           string
    RankingAddr      string
    AssetAddr        string
    ProductAddr      string
    SLAAddr          string
}
```

## Tác động

- Không thể deploy trong container/K8s nếu search-service không chạy trên `localhost:8083`
- Tất cả CVE search, semantic search, EPSS, CWE endpoints sẽ fail trong production
- Các service khác có thể fallback nhưng search không có cơ chế override

## Fix Proposal

### 1. Thêm `SearchAddr` vào `EmbeddedConfig`

```go
type EmbeddedConfig struct {
    JWTSecret        string
    IdentityAddr     string
    DataAddr         string
    SearchAddr       string  // NEW
    FindingAddr      string
    ScanAddr         string
    NotificationAddr string
    AIAddr           string
    RankingAddr      string
    AssetAddr        string
    ProductAddr      string
    SLAAddr          string
    JiraAddr         string  // NEW (currently proxied through findingHTTP incorrectly)
    ReportAddr       string  // NEW (currently proxied through findingHTTP incorrectly)
}
```

### 2. Dùng `coalesce` cho search-service

```go
searchHTTP := coalesce(cfg.SearchAddr, "http://localhost:8083")

upstreamURLs := map[string]string{
    "search-service": searchHTTP,  // was: "http://localhost:8083"
    "search":         searchHTTP,  // was: "http://localhost:8083"
    ...
}

// In NewUIAPIHandler call:
"search": searchHTTP,  // was: "http://localhost:8083"
```

### 3. Đọc từ environment variables làm last resort

Nếu `EmbeddedConfig` không được điền, fallback về env vars trước localhost:

```go
searchHTTP := coalesce(cfg.SearchAddr, os.Getenv("SEARCH_SERVICE_HTTP"), "http://localhost:8083")
```

## References

- [embedded.go L56-103](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/embedded.go#L56-L103)
- [embedded.go L145](file:///Users/binhnt/Lab/sec/cve/osv.dev/services/gateway-service/embedded.go#L145)
