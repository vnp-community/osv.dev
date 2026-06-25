# CR-HC-008: data-service — CWEHandler Nil (repo chưa wired)

## Trạng thái: 🟡 Medium

## Vấn đề
File: `services/data-service/embed/server.go:210`

```go
var cweH *httpdelivery.CWEHandler // TODO: wire postgres.CWERepo when available
```

`CWEHandler` luôn là `nil` → endpoint `/api/v1/cwe/*` và `/api/v2/cwe/*` trả 404 hoặc panic.
CWE (Common Weakness Enumeration) data đã có trong database (`cwe_weaknesses` table tồn tại trong public schema).

## Phân tích
```sql
-- Table đã tồn tại:
public.cwe_weaknesses
-- Columns cần kiểm tra: id, cwe_id, name, description, abstraction, structure, status, ...
```

CWE data đã được sync vào DB (từ data-service migration), nhưng handler không được wire.

## Giải pháp

### 1. Kiểm tra và implement CWERepository (nếu chưa có)
```go
type CWERepository interface {
    FindByID(ctx context.Context, cweID string) (*CWEEntry, error)
    List(ctx context.Context, filter CWEFilter) ([]*CWEEntry, int, error)
    Search(ctx context.Context, query string, limit int) ([]*CWEEntry, error)
}
```

### 2. Wire CWEHandler trong embed/server.go
```go
if pool != nil {
    cweRepo := postgres.NewCWERepo(pool)
    cweH = httpdelivery.NewCWEHandler(cweRepo)
    logger.Info().Msg("data-service: CWEHandler wired (PostgreSQL)")
} else {
    logger.Warn().Msg("data-service: pool is nil — CWE routes disabled")
}
```

### 3. Đăng ký routes
```go
if cweH != nil {
    mux.HandleFunc("GET /api/v1/cwe", cweH.List)
    mux.HandleFunc("GET /api/v1/cwe/{id}", cweH.GetByID)
    mux.HandleFunc("GET /api/v2/cwe", cweH.List)
    mux.HandleFunc("GET /api/v2/cwe/{id}", cweH.GetByID)
    mux.HandleFunc("GET /api/v1/cwe/search", cweH.Search)
}
```

## Files cần thay đổi
- `services/data-service/embed/server.go` — wire cweH
- `services/data-service/internal/infra/persistence/postgres/cwe_repo.go` [NEW nếu chưa có]
- `services/data-service/internal/delivery/http/cwe_handler.go` — đảm bảo đủ methods

## Acceptance Criteria
- [ ] `GET /api/v1/cwe` → 200 với list CWE entries từ DB
- [ ] `GET /api/v1/cwe/CWE-79` → 200 với CWE-79 details
- [ ] `GET /api/v1/cwe/search?q=injection` → search results
