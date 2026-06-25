# TASK-P0-002 — Nil-check trong APIKeyValidator

**Bug:** MOCK-012  
**Priority:** 🔴 P0 — Production Crash Risk  
**Effort:** ~10 phút  
**Service:** `gateway-service`  
**Loại thay đổi:** Code fix only (không cần DB migration)

---

## Mục tiêu

`APIKeyValidator.Validate()` sẽ panic (nil pointer dereference) khi `v.repo == nil` và Redis cache miss — xảy ra khi Redis restart hoặc API key dùng lần đầu. Cần thêm nil-check để return `ErrInvalidAPIKey` thay vì crash.

---

## Preconditions

- [ ] Đọc file: `services/gateway-service/internal/auth/apikey_validator.go`
- [ ] Xác định: tên error constant cho invalid API key (`ErrInvalidAPIKey`, `errInvalidKey`, `ErrInvalidToken`...)

---

## Steps

### Step 1 — Đọc file hiện tại

```
File: services/gateway-service/internal/auth/apikey_validator.go
```

Xác định:
1. Tên hàm `Validate` (có thể là `Validate`, `ValidateAPIKey`, `Authenticate`)
2. Tên error constant cho invalid key
3. Vị trí Redis cache lookup (hot path)
4. Vị trí `v.repo.FindByHash(` (cold path — chỗ sẽ panic)

### Step 2 — Thêm nil-check cho `v.repo` trước cold path

Trong hàm `Validate`, **sau** phần Redis cache lookup và **trước** khi gọi `v.repo.FindByHash(`:

```go
// Hot path: Redis cache lookup
if cached, err := v.cache.Get(ctx, cacheKey).Bytes(); err == nil {
    // ... parse cached claims
    return &claims, nil
}

// MOCK-012 FIX: nil-check — không thể validate API key nếu không có DB
if v.repo == nil {
    return nil, ErrInvalidAPIKey  // dùng đúng tên error trong codebase
}

// Cold path: PostgreSQL lookup
key, err := v.repo.FindByHash(ctx, hash)
// ...
```

### Step 3 — Xác nhận không có chỗ nào khác dereference `v.repo` mà thiếu nil-check

```bash
grep -n "v\.repo\." services/gateway-service/internal/auth/apikey_validator.go
```

---

## Acceptance Criteria

- [ ] Khi `repo == nil` và Redis cache miss → function return `ErrInvalidAPIKey` (hoặc tương đương), không panic
- [ ] Khi `repo == nil` và Redis cache hit → vẫn trả về claims bình thường (không bị ảnh hưởng)
- [ ] `go build ./services/gateway-service/...` — build thành công
- [ ] `go vet ./services/gateway-service/...` — không có warning

---

## Test Commands

```bash
# Build check
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/gateway-service/...

# Verify nil-check added
grep -n "repo == nil" services/gateway-service/internal/auth/apikey_validator.go

# Run existing tests
go test ./services/gateway-service/internal/auth/... -v
```
