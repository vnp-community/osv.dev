# Solution 004: Assets Subresources & BFF

**Status**: Proposed
**Target Service**: `asset-service`, `apps/osv` (Gateway BFF)
**Related CR**: [CR-004-assets-subresources.md](../CR-004-assets-subresources.md)

## 1. Hướng Tiếp Cận (Approach)
Theo TDD, `asset-service` là một service mới, quản lý thông tin các tài sản (IP, Host, Cloud instance). Tuy nhiên, yêu cầu Frontend lấy các "Findings theo Asset" sẽ vi phạm tính độc lập (Decoupling) nếu `asset-service` trực tiếp gọi Database của `finding-service`.
Do đó, chúng ta sẽ áp dụng **BFF Pattern (Backend For Frontend)** được cấu hình ngay tại Gateway (`apps/osv/internal/gateway/bff/`).

## 2. API Thiết Kế
### 2.1 `/api/v1/assets/tags` (Xử lý trực tiếp tại `asset-service`)
`asset-service` sẽ query tập hợp tags duy nhất trong DB.

```go
// services/asset-service/internal/infra/postgres/asset_repo.go
func (r *AssetRepository) GetUniqueTags(ctx context.Context) ([]string, error) {
    // Postgres unnest tags array and get distinct values
    query := `
        SELECT DISTINCT unnest(tags) as tag
        FROM assets
        ORDER BY tag ASC
    `
    // Execute query...
}
```
Gateway đơn giản là forward route này đến `asset-service`.

### 2.2 `/api/v1/assets/{id}/findings` (Xử lý thông qua BFF Gateway)
Gateway sẽ chặn request này, định tuyến lại thành lời gọi hợp lệ tới `finding-service` với Query Param tương ứng. `finding-service` vốn dĩ quản lý Finding và đã có cơ chế lọc (Filter).

**Tại `finding-service`**:
Bổ sung query param `asset_id` vào chức năng Filter danh sách Findings v2 hiện có: `GET /api/v2/findings?asset_id={id}`.

**Tại Gateway (`apps/osv/internal/gateway/bff/asset_bff.go`)**:
Viết một HTTP Handler chặn lời gọi `/api/v1/assets/{id}/findings` và map lại thành Reverse Proxy tới `finding-service`:

```go
func HandleAssetFindings(p *ReverseProxy) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        assetID := chi.URLParam(r, "id")
        
        // Rewrite URL: Từ /api/v1/assets/{id}/findings 
        // Sang: /api/v2/findings?asset_id={id}
        r.URL.Path = "/api/v2/findings"
        
        q := r.URL.Query()
        q.Set("asset_id", assetID)
        r.URL.RawQuery = q.Encode()
        
        // Forward tới finding-service:8085
        p.Forward("finding-service:8085")(w, r)
    }
}
```

## 3. Ưu Điểm
*   **Tránh High Coupling**: `asset-service` không cần biết đến sự tồn tại của bảng `findings`.
*   **Hiệu suất**: BFF thực hiện map đường dẫn trực tiếp, không cản trở băng thông (Proxy streaming).
*   **Bám sát TDD**: Kiến trúc Gateway có sẵn tính năng Transform và BFF.
