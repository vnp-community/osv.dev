# TASK-DD-001 — Product/Engagement/Test Domain Extensions

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-001 |
| **Service** | `finding-service` |
| **CR** | CR-DD-001 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | — (có thể bắt đầu ngay) |
| **Estimated effort** | 1 ngày |

## Context

`finding-service` hiện đã có domain stubs cho `product`, `product_type`, `engagement`, `test`.  
Task này mở rộng các entities hiện có và thêm 2 domain mới: `member` và `tool`.

## Reference

- Solution: [`sol-finding-service.md § CR-DD-001`](../solutions/sol-finding-service.md)
- CR spec: [`CR-DD-001-product-engagement-test-hierarchy.md`](../CR-DD-001-product-engagement-test-hierarchy.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create/Modify

### Modify (extend existing entities)
- `internal/domain/product/entity.go` — add fields
- `internal/domain/engagement/entity.go` — add fields
- `internal/domain/test/entity.go` — add fields

### Create (new domains)
- `internal/domain/member/entity.go`
- `internal/domain/member/repository.go`
- `internal/domain/tool/entity.go`
- `internal/domain/tool/repository.go`

## Implementation Spec

### `internal/domain/product/entity.go` — Add fields to existing Product struct

```go
// ADD these fields to existing Product struct
type Criticality string
const (
    CriticalityVeryHigh Criticality = "very high"
    CriticalityHigh     Criticality = "high"
    CriticalityMedium   Criticality = "medium"
    CriticalityLow      Criticality = "low"
    CriticalityVeryLow  Criticality = "very low"
    CriticalityNone     Criticality = "none"
)

type Platform string
const (
    PlatformWeb     Platform = "web"
    PlatformMobile  Platform = "mobile"
    PlatformDesktop Platform = "desktop"
    PlatformAPI     Platform = "api"
    PlatformIoT     Platform = "iot"
)

type Lifecycle string
const (
    LifecycleConstruction Lifecycle = "construction"
    LifecycleProduction   Lifecycle = "production"
    LifecycleRetirement   Lifecycle = "retirement"
)

type Origin string
const (
    OriginInternal   Origin = "internal"
    OriginContractor Origin = "contractor"
    OriginOutsourced Origin = "outsourced"
    OriginOpenSource Origin = "open source"
    OriginPurchased  Origin = "purchased"
)

// Add to Product struct:
BusinessCriticality  Criticality
Platform             Platform
Lifecycle            Lifecycle
Origin               Origin
SLAConfigurationID   *string
EnableSimpleRiskAcceptance  bool
EnableFullRiskAcceptance    bool
EnableProductTagInheritance bool
Tags                 []string
```

### `internal/domain/engagement/entity.go` — Add fields

```go
type EngagementType string
const (
    EngagementTypeInteractive EngagementType = "Interactive"
    EngagementTypeCICD        EngagementType = "CI/CD"
)

type EngagementStatus string
const (
    EngagementStatusNotStarted  EngagementStatus = "Not Started"
    EngagementStatusInProgress  EngagementStatus = "In Progress"
    EngagementStatusCompleted   EngagementStatus = "Completed"
    EngagementStatusBlocked     EngagementStatus = "Blocked"
    EngagementStatusCancelled   EngagementStatus = "Cancelled"
    EngagementStatusWaitingThirdParty EngagementStatus = "Waiting for Third Party"
)

// Add to Engagement struct:
Type     EngagementType
Status   EngagementStatus
BuildID  string
CommitHash string
BranchTag  string
SourceCodeManagementURI string
DeduplicationOnEngagement bool
BuildServerID            *string   // FK ToolConfiguration
OrchestrationEngineID    *string   // FK ToolConfiguration
```

### `internal/domain/test/entity.go` — Add fields

```go
// Add to Test struct:
ScanType        string  // "Trivy Scan", "Bandit Scan", etc.
PercentComplete int
BuildID         string
CommitHash      string
BranchTag       string
Version         string
Tags            []string
```

### `internal/domain/member/entity.go` — New file

```go
package member

import "time"

type Role string
const (
    RoleOwner      Role = "Owner"
    RoleMaintainer Role = "Maintainer"
    RoleWriter     Role = "Writer"
    RoleAPIImporter Role = "API Importer"
    RoleReader     Role = "Reader"
)

// ProductMember — RBAC membership for a Product
type ProductMember struct {
    ID        string
    ProductID string
    UserID    string
    Role      Role
    CreatedAt time.Time
}

// ProductTypeMember — RBAC membership for a ProductType
type ProductTypeMember struct {
    ID            string
    ProductTypeID string
    UserID        string
    Role          Role
    CreatedAt     time.Time
}
```

### `internal/domain/member/repository.go` — New file

```go
package member

import "context"

type ProductMemberRepository interface {
    Save(ctx context.Context, member *ProductMember) error
    FindByProductAndUser(ctx context.Context, productID, userID string) (*ProductMember, error)
    ListByProduct(ctx context.Context, productID string) ([]*ProductMember, error)
    Delete(ctx context.Context, productID, userID string) error
    GetRole(ctx context.Context, productID, userID string) (*Role, error)
}

type ProductTypeMemberRepository interface {
    Save(ctx context.Context, member *ProductTypeMember) error
    FindByProductTypeAndUser(ctx context.Context, productTypeID, userID string) (*ProductTypeMember, error)
    ListByProductType(ctx context.Context, productTypeID string) ([]*ProductTypeMember, error)
    Delete(ctx context.Context, productTypeID, userID string) error
}
```

### `internal/domain/tool/entity.go` — New file

```go
package tool

import "time"

type AuthType string
const (
    AuthTypeAPIKey    AuthType = "api_key"
    AuthTypeHTTPBasic AuthType = "http_basic"
    AuthTypeSSH       AuthType = "ssh"
    AuthTypeBearer    AuthType = "bearer"
)

// ToolConfiguration — external tool credentials (scanner, tracker)
// Passwords stored AES-256-GCM encrypted
type ToolConfiguration struct {
    ID          string
    Name        string
    Description string
    ToolType    string   // "GitHub"|"GitLab"|"Jira"|"Slack"|"SonarQube"|...
    URL         string
    AuthType    AuthType
    Username    string
    PasswordEnc string   // AES-256-GCM encrypted
    APIKeyEnc   string   // AES-256-GCM encrypted
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### `internal/domain/tool/repository.go` — New file

```go
package tool

import "context"

type ToolConfigurationRepository interface {
    Save(ctx context.Context, tool *ToolConfiguration) error
    FindByID(ctx context.Context, id string) (*ToolConfiguration, error)
    List(ctx context.Context) ([]*ToolConfiguration, error)
    Delete(ctx context.Context, id string) error
}
```

## Acceptance Criteria

- [x] `Product` struct có đủ các fields: BusinessCriticality, Platform, Lifecycle, Origin, SLAConfigurationID, Tags
- [x] `Engagement` struct có đủ: Type, Status, BuildID, CommitHash, BranchTag, DeduplicationOnEngagement
- [x] `Test` struct có đủ: ScanType, PercentComplete, BuildID, Tags
- [x] `ProductMember` entity với 5 roles: Owner, Maintainer, Writer, API Importer, Reader
- [x] `ToolConfiguration` entity với encrypted credentials fields
- [x] `ProductMemberRepository` interface có: Save, FindByProductAndUser, ListByProduct, Delete, GetRole
- [x] `ToolConfigurationRepository` interface có: Save, FindByID, List, Delete
- [x] Code build thành công: `go build ./...`
- [x] Không có unused imports

## Implementation Status: ✅ DONE

> Implemented in previous session. All domain entities verified:
> - `internal/domain/product/entity.go` — Product with BusinessCriticality, Platform, Lifecycle, Origin
> - `internal/domain/engagement/entity.go` — Engagement with Type, Status, DeduplicationOnEngagement
> - `internal/domain/test/entity.go` — Test with ScanType, PercentComplete
> - `internal/domain/member/entity.go` — ProductMember + ProductTypeMember với 5 roles
> - `internal/domain/tool/entity.go` — ToolConfiguration với encrypted fields
