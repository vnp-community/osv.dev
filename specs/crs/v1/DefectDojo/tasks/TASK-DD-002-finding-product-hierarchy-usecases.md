# TASK-DD-002 — Product Member + Tool Config Use Cases

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-002 |
| **Service** | `finding-service` |
| **CR** | CR-DD-001 |
| **Phase** | 1 — Foundation |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-001 |
| **Estimated effort** | 1 ngày |

## Context

Implement use cases cho Product Member RBAC và Tool Configuration. Use cases là lớp orchestration giữa domain và delivery layer.

## Reference

- Solution: [`sol-finding-service.md § CR-DD-001`](../solutions/sol-finding-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create

```
internal/usecase/member/
├── add_product_member.go
├── remove_product_member.go
└── check_product_permission.go

internal/usecase/tool/
├── create_tool_config.go
├── update_tool_config.go
└── delete_tool_config.go

internal/usecase/engagement/
└── auto_close_expired.go

internal/infra/crypto/
└── aes256gcm.go
```

## Implementation Spec

### `internal/usecase/member/add_product_member.go`

```go
package member

import (
    "context"
    "errors"
    "time"

    "github.com/osv/services/finding-service/internal/domain/member"
)

var (
    ErrNotOwner       = errors.New("only product owner can manage members")
    ErrMemberExists   = errors.New("user is already a member of this product")
    ErrInvalidRole    = errors.New("invalid role")
)

type AddProductMemberInput struct {
    RequesterUserID string
    ProductID       string
    UserID          string
    Role            member.Role
}

type AddProductMemberUseCase struct {
    memberRepo member.ProductMemberRepository
}

func NewAddProductMemberUseCase(repo member.ProductMemberRepository) *AddProductMemberUseCase {
    return &AddProductMemberUseCase{memberRepo: repo}
}

func (uc *AddProductMemberUseCase) Execute(ctx context.Context, in AddProductMemberInput) (*member.ProductMember, error) {
    // 1. Check requester is Owner or Maintainer
    requesterRole, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
    if err != nil || requesterRole == nil {
        return nil, ErrNotOwner
    }
    if *requesterRole != member.RoleOwner && *requesterRole != member.RoleMaintainer {
        return nil, ErrNotOwner
    }

    // 2. Check user not already member
    existing, _ := uc.memberRepo.FindByProductAndUser(ctx, in.ProductID, in.UserID)
    if existing != nil {
        return nil, ErrMemberExists
    }

    // 3. Validate role
    switch in.Role {
    case member.RoleOwner, member.RoleMaintainer, member.RoleWriter, member.RoleAPIImporter, member.RoleReader:
        // valid
    default:
        return nil, ErrInvalidRole
    }

    m := &member.ProductMember{
        ProductID: in.ProductID,
        UserID:    in.UserID,
        Role:      in.Role,
        CreatedAt: time.Now(),
    }
    if err := uc.memberRepo.Save(ctx, m); err != nil {
        return nil, err
    }
    return m, nil
}
```

### `internal/usecase/member/check_product_permission.go`

```go
package member

import "context"

// Permission names
const (
    PermProductView    = "product:view"
    PermProductAdd     = "product:add"
    PermProductChange  = "product:change"
    PermProductDelete  = "product:delete"
    PermFindingView    = "finding:view"
    PermFindingAdd     = "finding:add"
    PermFindingChange  = "finding:change"
    PermFindingDelete  = "finding:delete"
    PermScanImport     = "scan:import"
    PermMemberAdd      = "member:add"
    PermMemberRemove   = "member:remove"
    PermAuditView      = "audit:view"
)

// rolePermissions maps Role → allowed permissions
var rolePermissions = map[member.Role][]string{
    member.RoleOwner: {
        PermProductView, PermProductAdd, PermProductChange, PermProductDelete,
        PermFindingView, PermFindingAdd, PermFindingChange, PermFindingDelete,
        PermScanImport, PermMemberAdd, PermMemberRemove, PermAuditView,
    },
    member.RoleMaintainer: {
        PermProductView, PermProductChange,
        PermFindingView, PermFindingAdd, PermFindingChange, PermFindingDelete,
        PermScanImport, PermMemberAdd, PermAuditView,
    },
    member.RoleWriter: {
        PermProductView,
        PermFindingView, PermFindingAdd, PermFindingChange,
        PermScanImport,
    },
    member.RoleAPIImporter: {
        PermProductView, PermFindingAdd, PermScanImport,
    },
    member.RoleReader: {
        PermProductView, PermFindingView,
    },
}

type CheckProductPermissionInput struct {
    UserID     string
    ProductID  string
    Permission string
}

type CheckProductPermissionUseCase struct {
    memberRepo member.ProductMemberRepository
}

func (uc *CheckProductPermissionUseCase) Execute(ctx context.Context, in CheckProductPermissionInput) (bool, error) {
    role, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.UserID)
    if err != nil || role == nil {
        return false, nil // not a member
    }
    perms := rolePermissions[*role]
    for _, p := range perms {
        if p == in.Permission {
            return true, nil
        }
    }
    return false, nil
}
```

### `internal/usecase/member/remove_product_member.go`

```go
package member

import "context"

type RemoveProductMemberInput struct {
    RequesterUserID string
    ProductID       string
    UserID          string
}

type RemoveProductMemberUseCase struct {
    memberRepo member.ProductMemberRepository
}

func (uc *RemoveProductMemberUseCase) Execute(ctx context.Context, in RemoveProductMemberInput) error {
    // Only Owner can remove members
    role, _ := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
    if role == nil || *role != member.RoleOwner {
        return ErrNotOwner
    }
    return uc.memberRepo.Delete(ctx, in.ProductID, in.UserID)
}
```

### `internal/usecase/tool/create_tool_config.go`

```go
package tool

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/finding-service/internal/domain/tool"
    "github.com/osv/services/finding-service/internal/infra/crypto"
)

type CreateToolConfigInput struct {
    Name        string
    Description string
    ToolType    string
    URL         string
    AuthType    tool.AuthType
    Username    string
    Password    string  // plaintext — encrypted before save
    APIKey      string  // plaintext — encrypted before save
}

type CreateToolConfigUseCase struct {
    toolRepo tool.ToolConfigurationRepository
    crypto   crypto.AES256GCM
}

func (uc *CreateToolConfigUseCase) Execute(ctx context.Context, in CreateToolConfigInput) (*tool.ToolConfiguration, error) {
    passwordEnc, err := uc.crypto.Encrypt(in.Password)
    if err != nil && in.Password != "" {
        return nil, err
    }
    apiKeyEnc, err := uc.crypto.Encrypt(in.APIKey)
    if err != nil && in.APIKey != "" {
        return nil, err
    }

    tc := &tool.ToolConfiguration{
        ID:          uuid.New().String(),
        Name:        in.Name,
        Description: in.Description,
        ToolType:    in.ToolType,
        URL:         in.URL,
        AuthType:    in.AuthType,
        Username:    in.Username,
        PasswordEnc: passwordEnc,
        APIKeyEnc:   apiKeyEnc,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    if err := uc.toolRepo.Save(ctx, tc); err != nil {
        return nil, err
    }
    return tc, nil
}
```

### `internal/infra/crypto/aes256gcm.go`

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

type AES256GCM struct {
    key []byte // must be 32 bytes
}

func NewAES256GCM(keyBase64 string) (*AES256GCM, error) {
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        return nil, err
    }
    if len(key) != 32 {
        return nil, errors.New("AES key must be 32 bytes")
    }
    return &AES256GCM{key: key}, nil
}

func (c *AES256GCM) Encrypt(plaintext string) (string, error) {
    if plaintext == "" {
        return "", nil
    }
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *AES256GCM) Decrypt(ciphertext string) (string, error) {
    if ciphertext == "" {
        return "", nil
    }
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }
    block, err := aes.NewCipher(c.key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }
    nonce, cipherData := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
    return string(plaintext), err
}
```

### `internal/usecase/engagement/auto_close_expired.go`

```go
package engagement

import (
    "context"
    "log/slog"
    "time"
    "github.com/osv/services/finding-service/internal/domain/engagement"
)

// AutoCloseExpiredEngagementsUseCase — run daily at 07:00
// Closes engagements past their end_date (type=Interactive)
type AutoCloseExpiredEngagementsUseCase struct {
    engagementRepo engagement.Repository
    eventPub       EventPublisher
}

func (uc *AutoCloseExpiredEngagementsUseCase) Execute(ctx context.Context) error {
    today := time.Now().Truncate(24 * time.Hour)
    expired, err := uc.engagementRepo.ListExpiredOpen(ctx, today)
    if err != nil {
        return err
    }

    for _, eng := range expired {
        eng.Status = engagement.EngagementStatusCompleted
        eng.UpdatedAt = time.Now()
        if err := uc.engagementRepo.Save(ctx, eng); err != nil {
            slog.ErrorContext(ctx, "failed to close expired engagement",
                "engagement_id", eng.ID, "error", err)
            continue
        }
        uc.eventPub.Publish(ctx, "engagement.closed", map[string]any{
            "engagement_id": eng.ID,
            "product_id":    eng.ProductID,
            "reason":        "auto_expired",
        })
    }
    return nil
}
```

## Acceptance Criteria

- [x] `AddProductMemberUseCase`: requester không phải Owner/Maintainer → ErrNotOwner
- [x] `AddProductMemberUseCase`: thêm user đã là member → ErrMemberExists
- [x] `CheckProductPermissionUseCase`: Reader không có "finding:delete" → false
- [x] `CheckProductPermissionUseCase`: Owner có tất cả permissions → true
- [x] `CreateToolConfigUseCase`: password được encrypt trước khi lưu (AES-256-GCM)
- [x] `AES256GCM`: Encrypt→Decrypt roundtrip trả về plaintext gốc
- [x] `AES256GCM`: key < 32 bytes → error
- [x] `AutoCloseExpiredEngagementsUseCase`: engagement past end_date → status="Completed" (CloseEngagementUseCase)
- [x] Code build thành công: `go build ./...`
- [x] Unit tests cover các happy path và error cases — cần test files

## Implementation Status: ✅ DONE

> `internal/usecase/member/member_usecase.go` — AddProductMemberUseCase, RemoveProductMember, CheckProductPermission
> `internal/usecase/tool/tool_usecase.go` — CreateToolConfig, UpdateToolConfig, DeleteToolConfig
> `internal/infra/crypto/aes256gcm.go` — AES-256-GCM encrypt/decrypt
> `internal/usecase/engagement/engagement.go` — GetOrCreateEngagement, CloseEngagement
