# Solution 003: Products v1 Compatibility Adapter

**Status**: Proposed
**Target Service**: `finding-service`, `apps/osv` (Gateway)
**Related CR**: [CR-003-products-v1-compatibility.md](../CR-003-products-v1-compatibility.md)

## 1. Hướng Tiếp Cận (Approach)
Tránh việc sửa đổi mã nguồn Frontend (có nguy cơ sinh lỗi Regression) hoặc tạo Gateway Transform Middleware (gây khó debug). Thay vào đó, ta sẽ bổ sung lớp Adapter v1 tại `finding-service` nhằm đảm bảo chuẩn Clean Architecture: Tái sử dụng Use Cases ở v2, chỉ viết thêm HTTP Handlers để map REST payload/URL.

## 2. Thiết kế tại API Gateway (`apps/osv`)
Mở port route prefix `v1` cho Products và Engagements về `finding-service:8085`.

```go
// apps/osv/internal/gateway/router.go
findingRouterV1 := router.Group("/api/v1")
findingRouterV1.Use(authMiddleware)

// Forward v1 endpoints to finding-service
findingRouterV1.Any("/products", proxy.Forward("finding-service:8085"))
findingRouterV1.Any("/products/*", proxy.Forward("finding-service:8085"))
findingRouterV1.Any("/engagements/*", proxy.Forward("finding-service:8085"))
```

## 3. Thiết kế HTTP Adapter tại `finding-service`
Tạo một thư mục mới `internal/adapter/http/v1` nằm cạnh `internal/adapter/http/v2`.

```go
// services/finding-service/internal/adapter/http/v1/product_handler.go

type ProductHandlerV1 struct {
    productUseCase domain.ProductUseCase
}

func (h *ProductHandlerV1) GetProducts(w http.ResponseWriter, r *http.Request) {
    // 1. Dùng Use Case của v2 để lấy data
    products, err := h.productUseCase.List(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 2. Map thành DTO của v1 (nếu có khác biệt như Camel vs Snake case)
    var resp []ProductDTOV1
    for _, p := range products {
        resp = append(resp, ProductDTOV1{
            ID:   p.ID,
            Name: p.Name,
            // mapping ...
        })
    }
    
    // 3. Trả về JSON
    json.NewEncoder(w).Encode(resp)
}

// Map các route con
func (h *ProductHandlerV1) GetEngagements(w http.ResponseWriter, r *http.Request) {
    productID := chi.URLParam(r, "id")
    // Gọi UseCase của v2 lọc theo ProductID
    engagements, err := h.engagementUseCase.ListByProductID(r.Context(), productID)
    // Map & Return JSON...
}
```

## 4. Ưu Điểm
*   **Decoupled**: Core Use Case không thay đổi.
*   **Dễ bảo trì**: Dễ dàng gỡ bỏ toàn bộ thư mục `v1` khi Frontend có thời gian refactor gọi API chuẩn `v2` trong tương lai.
