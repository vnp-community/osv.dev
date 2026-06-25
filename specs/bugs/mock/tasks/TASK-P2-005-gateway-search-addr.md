# TASK-P2-005 — Dùng cfg.SearchAddr thay vì hardcode trong gateway-service

**Bug:** MOCK-013  
**Priority:** 🟡 P2 — Config không hoạt động  
**Effort:** ~30 phút  
**Service:** `gateway-service`  
**Loại thay đổi:** Sửa embedded.go — thay hardcoded string bằng config value

---

## Mục tiêu

`gateway-service/embedded.go` hardcode `"http://localhost:8083"` cho search-service ở 3 chỗ, bỏ qua field `SearchAddr` trong `EmbeddedConfig`. Khi deploy với search-service trên host khác → proxy fail.

---

## Steps

### Step 1 — Xác định các chỗ hardcode

```bash
grep -n "localhost:8083\|8083" services/gateway-service/embedded.go
```

Ghi lại line numbers của tất cả hardcoded strings.

### Step 2 — Xác định cách các service addr khác được xử lý

```bash
grep -n "coalesce\|cfg\.\|Addr" services/gateway-service/embedded.go | head -20
```

Xem pattern đang dùng. Thường là:
```go
identityHTTP := coalesce(cfg.IdentityAddr, "http://localhost:8081")
```

### Step 3 — Thêm searchHTTP variable và thay tất cả hardcode

Trong function `WireEmbedded`, tìm vị trí các addr khác được khởi tạo và thêm:

```go
// FIX MOCK-013: dùng cfg.SearchAddr thay vì hardcode
searchHTTP := coalesce(cfg.SearchAddr, "http://localhost:8083")
```

Sau đó tìm và thay tất cả `"http://localhost:8083"` bằng `searchHTTP`:

```go
// Thay tất cả 3 chỗ sau:
// "search-service": "http://localhost:8083"  →  "search-service": searchHTTP
// "search": "http://localhost:8083"           →  "search": searchHTTP
// trong UIAPIHandler map
```

### Step 4 — Cập nhật monolith main để pass SearchAddr

Tìm nơi `EmbeddedConfig` được khởi tạo:
```bash
grep -rn "EmbeddedConfig\|gateway\.Config\|WireEmbedded" apps/osv/
```

Thêm `SearchAddr` vào config:
```go
gatewayCfg := gatewayservice.EmbeddedConfig{
    JWTSecret:    os.Getenv("JWT_SECRET"),
    IdentityAddr: os.Getenv("IDENTITY_SERVICE_ADDR"),
    SearchAddr:   os.Getenv("SEARCH_SERVICE_ADDR"),   // ← thêm dòng này
    FindingAddr:  os.Getenv("FINDING_SERVICE_ADDR"),
    // ...
}
```

---

## Acceptance Criteria

- [ ] Không còn `"http://localhost:8083"` hardcoded trong `embedded.go` (trừ default value trong `coalesce`)
- [ ] Khi `SEARCH_SERVICE_ADDR=http://search-host:8083` được set → proxy forward đúng tới host đó
- [ ] `go build ./services/gateway-service/... ./apps/osv/...` — thành công

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/gateway-service/...
go build ./apps/osv/...

# Verify hardcodes removed
grep -n '"http://localhost:8083"' services/gateway-service/embedded.go
# Expected: chỉ còn trong coalesce default (nếu có), không có chỗ nào khác

# Verify searchHTTP variable added
grep -n "searchHTTP\|SearchAddr\|coalesce.*8083" services/gateway-service/embedded.go
```
