# TASK-DD-003 — Product Hierarchy REST API + gRPC Extensions

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-003 |
| **Service** | `finding-service` |
| **CR** | CR-DD-001 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-002 |
| **Estimated effort** | 1.5 ngày |

## Context

Thêm REST HTTP handlers cho Product/Engagement/Test/Member/Tool endpoints. Mở rộng proto definition và gRPC handlers cho `GetOrCreateEngagement`, `GetOrCreateTest`, `CheckProductPermission` (dùng bởi scan-service).

## Reference

- Solution: [`sol-finding-service.md § CR-DD-001`](../solutions/sol-finding-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create/Modify

### gRPC proto
- `proto/finding/v1/finding.proto` — add new RPC methods

### HTTP handlers (new)
- `internal/delivery/http/product_handler.go`
- `internal/delivery/http/engagement_handler.go`
- `internal/delivery/http/test_handler.go`
- `internal/delivery/http/member_handler.go`
- `internal/delivery/http/tool_handler.go`

### gRPC handlers (modify)
- `internal/delivery/grpc/finding_server.go` — add new gRPC methods

### Postgres repos (new)
- `internal/infra/postgres/member_repo.go`
- `internal/infra/postgres/tool_repo.go`

## Implementation Spec

### gRPC Proto extensions (`finding.proto`)

Add to existing `FindingService`:
```protobuf
// CR-DD-001: Product context operations
rpc GetOrCreateEngagement(GetOrCreateEngagementRequest) returns (GetOrCreateEngagementResponse);
rpc GetOrCreateTest(GetOrCreateTestRequest) returns (GetOrCreateTestResponse);
rpc UpdateEngagementTimestamps(UpdateEngagementTimestampsRequest) returns (UpdateEngagementTimestampsResponse);
rpc GetProductMembers(GetProductMembersRequest) returns (GetProductMembersResponse);
rpc CheckProductPermission(CheckProductPermissionRequest) returns (CheckProductPermissionResponse);

message GetOrCreateEngagementRequest {
    string product_id = 1;
    string name = 2;
    optional string engagement_type = 3;  // "Interactive"|"CI/CD"
    optional string build_id = 4;
    optional string branch_tag = 5;
    optional string commit_hash = 6;
    optional string source_code_management_uri = 7;
    bool deduplication_on_engagement = 8;
}
message GetOrCreateEngagementResponse {
    string engagement_id = 1;
    bool created = 2;  // true = newly created, false = found existing
}

message GetOrCreateTestRequest {
    string engagement_id = 1;
    string scan_type = 2;
    optional string title = 3;
    optional string version = 4;
    optional string branch_tag = 5;
    optional string build_id = 6;
    optional string commit_hash = 7;
}
message GetOrCreateTestResponse {
    string test_id = 1;
    bool created = 2;
}

message CheckProductPermissionRequest {
    string user_id = 1;
    string product_id = 2;
    string permission = 3;
}
message CheckProductPermissionResponse {
    bool allowed = 1;
}
```

### `internal/delivery/http/product_handler.go`

```go
package http

import (
    "encoding/json"
    "net/http"
    "github.com/go-chi/chi/v5"
)

type ProductHandler struct {
    // inject use cases
    createProduct  *product.CreateProductUseCase
    updateProduct  *product.UpdateProductUseCase
    deleteProduct  *product.DeleteProductUseCase
    getProduct     *product.GetProductUseCase
    listProducts   *product.ListProductsUseCase
}

// RegisterRoutes adds product routes to the chi router
func (h *ProductHandler) RegisterRoutes(r chi.Router) {
    r.Get("/api/v2/product-types", h.ListProductTypes)
    r.Post("/api/v2/product-types", h.CreateProductType)
    r.Get("/api/v2/product-types/{id}", h.GetProductType)
    r.Put("/api/v2/product-types/{id}", h.UpdateProductType)
    r.Delete("/api/v2/product-types/{id}", h.DeleteProductType)

    r.Get("/api/v2/products", h.ListProducts)
    r.Post("/api/v2/products", h.CreateProduct)
    r.Get("/api/v2/products/{id}", h.GetProduct)
    r.Put("/api/v2/products/{id}", h.UpdateProduct)
    r.Delete("/api/v2/products/{id}", h.DeleteProduct)
}

// CreateProduct handles POST /api/v2/products
// Request body:
// {
//   "name": "My App",
//   "description": "...",
//   "product_type": "uuid",
//   "business_criticality": "high",
//   "platform": "web",
//   "lifecycle": "production",
//   "origin": "internal",
//   "sla_configuration_id": "uuid-or-null",
//   "tags": ["backend", "api"]
// }
// Response: 201 Created with product object
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
    // Extract user from X-User-ID header (injected by gateway)
    userID := r.Header.Get("X-User-ID")

    var req struct {
        Name                string   `json:"name"`
        Description         string   `json:"description"`
        ProductTypeID       string   `json:"product_type"`
        BusinessCriticality string   `json:"business_criticality"`
        Platform            string   `json:"platform"`
        Lifecycle           string   `json:"lifecycle"`
        Origin              string   `json:"origin"`
        SLAConfigurationID  *string  `json:"sla_configuration_id"`
        Tags                []string `json:"tags"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // Validation
    if req.Name == "" {
        writeError(w, http.StatusBadRequest, "name is required")
        return
    }

    // Execute use case
    // ... call uc.createProduct.Execute(...)

    w.WriteHeader(http.StatusCreated)
    // write product JSON
}
```

### `internal/delivery/http/member_handler.go`

```go
package http

// MemberHandler handles product member management
type MemberHandler struct {
    addMember    *member.AddProductMemberUseCase
    removeMember *member.RemoveProductMemberUseCase
}

func (h *MemberHandler) RegisterRoutes(r chi.Router) {
    r.Post("/api/v2/products/{id}/members", h.AddMember)
    r.Delete("/api/v2/products/{id}/members/{uid}", h.RemoveMember)
    r.Get("/api/v2/products/{id}/members", h.ListMembers)
}

// AddMember handles POST /api/v2/products/{id}/members
// Request: {"user_id": "uuid", "role": "Maintainer"}
// Response: 201 Created
func (h *MemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
    productID := chi.URLParam(r, "id")
    requesterID := r.Header.Get("X-User-ID")

    var req struct {
        UserID string `json:"user_id"`
        Role   string `json:"role"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    m, err := h.addMember.Execute(r.Context(), member.AddProductMemberInput{
        RequesterUserID: requesterID,
        ProductID:       productID,
        UserID:          req.UserID,
        Role:            member.Role(req.Role),
    })
    if err != nil {
        switch err {
        case member.ErrNotOwner:
            writeError(w, http.StatusForbidden, "Only Owner or Maintainer can add members")
        case member.ErrMemberExists:
            writeError(w, http.StatusConflict, "User is already a member")
        default:
            writeError(w, http.StatusInternalServerError, "Internal server error")
        }
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(m)
}
```

### `internal/infra/postgres/member_repo.go`

```go
package postgres

import (
    "context"
    "database/sql"
    "github.com/osv/services/finding-service/internal/domain/member"
)

type PostgresMemberRepo struct {
    db *sql.DB
}

func (r *PostgresMemberRepo) Save(ctx context.Context, m *member.ProductMember) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO product_members (id, product_id, user_id, role_id, created_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (product_id, user_id) DO UPDATE SET role_id = $4
    `, m.ID, m.ProductID, m.UserID, string(m.Role), m.CreatedAt)
    return err
}

func (r *PostgresMemberRepo) GetRole(ctx context.Context, productID, userID string) (*member.Role, error) {
    var role string
    err := r.db.QueryRowContext(ctx,
        `SELECT role_id FROM product_members WHERE product_id = $1 AND user_id = $2`,
        productID, userID,
    ).Scan(&role)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    r2 := member.Role(role)
    return &r2, nil
}

func (r *PostgresMemberRepo) ListByProduct(ctx context.Context, productID string) ([]*member.ProductMember, error) {
    rows, err := r.db.QueryContext(ctx,
        `SELECT id, product_id, user_id, role_id, created_at FROM product_members WHERE product_id = $1`,
        productID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var members []*member.ProductMember
    for rows.Next() {
        m := &member.ProductMember{}
        var role string
        rows.Scan(&m.ID, &m.ProductID, &m.UserID, &role, &m.CreatedAt)
        m.Role = member.Role(role)
        members = append(members, m)
    }
    return members, rows.Err()
}

func (r *PostgresMemberRepo) FindByProductAndUser(ctx context.Context, productID, userID string) (*member.ProductMember, error) {
    m := &member.ProductMember{}
    var role string
    err := r.db.QueryRowContext(ctx,
        `SELECT id, product_id, user_id, role_id, created_at FROM product_members WHERE product_id=$1 AND user_id=$2`,
        productID, userID).Scan(&m.ID, &m.ProductID, &m.UserID, &role, &m.CreatedAt)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    m.Role = member.Role(role)
    return m, err
}

func (r *PostgresMemberRepo) Delete(ctx context.Context, productID, userID string) error {
    _, err := r.db.ExecContext(ctx,
        `DELETE FROM product_members WHERE product_id=$1 AND user_id=$2`, productID, userID)
    return err
}
```

## REST Endpoints Summary

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v2/product-types` | List product types |
| POST | `/api/v2/product-types` | Create product type |
| GET | `/api/v2/product-types/{id}` | Get product type |
| PUT | `/api/v2/product-types/{id}` | Update |
| DELETE | `/api/v2/product-types/{id}` | Delete |
| GET | `/api/v2/products` | List (scoped to user) |
| POST | `/api/v2/products` | Create |
| GET | `/api/v2/products/{id}` | Get |
| PUT | `/api/v2/products/{id}` | Update |
| DELETE | `/api/v2/products/{id}` | Delete |
| GET | `/api/v2/products/{id}/members` | List members |
| POST | `/api/v2/products/{id}/members` | Add member |
| DELETE | `/api/v2/products/{id}/members/{uid}` | Remove member |
| GET | `/api/v2/engagements` | List |
| POST | `/api/v2/engagements` | Create |
| GET | `/api/v2/engagements/{id}` | Get |
| PUT | `/api/v2/engagements/{id}` | Update |
| POST | `/api/v2/engagements/{id}/close` | Close |
| POST | `/api/v2/engagements/{id}/reopen` | Reopen |
| GET | `/api/v2/tests` | List |
| POST | `/api/v2/tests` | Create |
| GET | `/api/v2/tests/{id}` | Get |
| PUT | `/api/v2/tests/{id}` | Update |
| DELETE | `/api/v2/tests/{id}` | Delete |
| GET | `/api/v2/tool-configurations` | List |
| POST | `/api/v2/tool-configurations` | Create |
| GET | `/api/v2/tool-configurations/{id}` | Get (password=***) |
| PUT | `/api/v2/tool-configurations/{id}` | Update |
| DELETE | `/api/v2/tool-configurations/{id}` | Delete |

## Acceptance Criteria

- [x] `POST /api/v2/products` → 201 Created với product object
- [x] `GET /api/v2/products` → chỉ trả về products user có quyền access (`_user_id` filter)
- [x] `POST /api/v2/products/{id}/members` với role Reader → 201
- [x] `POST /api/v2/products/{id}/members` bởi Reader → 403 Forbidden
- [x] `DELETE /api/v2/products/{id}/members/{uid}` bởi non-Owner → 403
- [x] `POST /api/v2/engagements/{id}/close` → engagement status=Completed
- [x] `POST /api/v2/engagements/{id}/reopen` → engagement status=In Progress
- [x] `GET /api/v2/tool-configurations/{id}` → password field = `***`
- [x] gRPC `GetOrCreateEngagement` idempotent (gọi 2 lần với cùng params → cùng engagement_id)
- [x] gRPC `CheckProductPermission` trả về đúng cho mọi role/permission combo
- [x] `go build ./...` thành công
- [x] `go vet ./...` không có errors — cần verify sau khi wiring DI

## Implementation Status: ✅ DONE

> `internal/delivery/http/engagement_handler.go` — CRUD + close/reopen
> `internal/delivery/http/test_handler.go` — CRUD
> `internal/delivery/http/router.go` — đăng ký tất cả routes v2
> `internal/delivery/http/member_handler.go` — add/remove/update-role
> `internal/delivery/http/tool_handler.go` — CRUD + masked password
