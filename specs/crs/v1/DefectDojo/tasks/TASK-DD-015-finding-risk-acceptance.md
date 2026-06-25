# ✅ COMPLETED — TASK-DD-015 — Risk Acceptance Domain + Use Cases

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-015 |
| **Service** | `finding-service` |
| **CR** | CR-DD-005 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-003 (Product API), TASK-DD-007 (migrations) |
| **Estimated effort** | 1.5 ngày |

## Context

Implement Risk Acceptance feature: cho phép Owners tạm thời chấp nhận rủi ro cho một hoặc nhiều findings. Risk Acceptance có expiration date — khi hết hạn, findings bị reactivated (hoặc giữ nguyên nếu `reactivate_expired=false`).

## Reference

- Solution: [`sol-finding-service.md § CR-DD-005`](../solutions/sol-finding-service.md)

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/finding-service/
```

## Files to Create

```
internal/domain/riskacceptance/
├── entity.go
└── repository.go

internal/usecase/riskacceptance/
├── create.go
├── expire.go
├── add_findings.go
└── remove_finding.go

internal/delivery/http/
└── risk_acceptance_handler.go

internal/infra/postgres/
└── risk_acceptance_repo.go
```

## Implementation Spec

### `internal/domain/riskacceptance/entity.go`

```go
package riskacceptance

import "time"

type RiskAcceptance struct {
    ID            string
    Name          string    // descriptive name
    ProductID     string

    AcceptedByID  string    // Owner user ID
    ExpirationDate *time.Time

    Notes         string
    ProofFileKey  string    // MinIO key for proof document

    // On expiry behavior
    ReactivateExpired           bool   // if true, findings reactivated on expiry
    ReactivateNoteText          string // note added when reactivating
    RestartSLAOnReactivation    bool   // if true, reset SLA date on reactivation

    IsExpired  bool
    FindingIDs []string  // findings under this risk acceptance

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### `internal/domain/riskacceptance/repository.go`

```go
package riskacceptance

import "context"

type Repository interface {
    Save(ctx context.Context, ra *RiskAcceptance) error
    FindByID(ctx context.Context, id string) (*RiskAcceptance, error)
    ListByProduct(ctx context.Context, productID string) ([]*RiskAcceptance, error)
    ListExpiring(ctx context.Context, before time.Time) ([]*RiskAcceptance, error)
    Delete(ctx context.Context, id string) error
    UpdateExpired(ctx context.Context, id string) error
    AddFinding(ctx context.Context, raID, findingID string) error
    RemoveFinding(ctx context.Context, raID, findingID string) error
}
```

### `internal/usecase/riskacceptance/create.go`

```go
package riskacceptance

import (
    "context"
    "errors"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/finding-service/internal/domain/member"
    "github.com/osv/services/finding-service/internal/domain/riskacceptance"
    findinguc "github.com/osv/services/finding-service/internal/usecase/finding"
)

var (
    ErrNotOwner         = errors.New("only product owner can create risk acceptances")
    ErrFindingNotInProduct = errors.New("finding does not belong to this product")
    ErrSimpleRiskDisabled  = errors.New("simple risk acceptance not enabled for this product")
)

type CreateRiskAcceptanceInput struct {
    Name                        string
    ProductID                   string
    RequesterUserID             string
    FindingIDs                  []string
    ExpirationDate              *time.Time
    Notes                       string
    ProofFileKey                string
    ReactivateExpired           bool
    ReactivateNoteText          string
    RestartSLAOnReactivation    bool
}

type CreateRiskAcceptanceUseCase struct {
    raRepo      riskacceptance.Repository
    memberRepo  member.ProductMemberRepository
    findingRepo finding.Repository
    acceptRiskUC *findinguc.AcceptRiskUseCase
    eventPub    EventPublisher
}

func (uc *CreateRiskAcceptanceUseCase) Execute(ctx context.Context, in CreateRiskAcceptanceInput) (*riskacceptance.RiskAcceptance, error) {
    // 1. Check requester is Owner or Maintainer
    role, err := uc.memberRepo.GetRole(ctx, in.ProductID, in.RequesterUserID)
    if err != nil || role == nil {
        return nil, ErrNotOwner
    }
    if *role != member.RoleOwner && *role != member.RoleMaintainer {
        return nil, ErrNotOwner
    }

    // 2. Create risk acceptance record
    ra := &riskacceptance.RiskAcceptance{
        ID:                          uuid.New().String(),
        Name:                        in.Name,
        ProductID:                   in.ProductID,
        AcceptedByID:                in.RequesterUserID,
        ExpirationDate:              in.ExpirationDate,
        Notes:                       in.Notes,
        ProofFileKey:                in.ProofFileKey,
        ReactivateExpired:           in.ReactivateExpired,
        ReactivateNoteText:          in.ReactivateNoteText,
        RestartSLAOnReactivation:    in.RestartSLAOnReactivation,
        FindingIDs:                  in.FindingIDs,
        CreatedAt:                   time.Now(),
        UpdatedAt:                   time.Now(),
    }
    if err := uc.raRepo.Save(ctx, ra); err != nil {
        return nil, err
    }

    // 3. Transition each finding to RiskAccepted state
    for _, fid := range in.FindingIDs {
        uc.acceptRiskUC.Execute(ctx, findinguc.AcceptRiskInput{
            FindingID:   fid,
            RequesterID: in.RequesterUserID,
        })
    }

    uc.eventPub.Publish(ctx, "risk_acceptance.created", map[string]any{
        "risk_acceptance_id": ra.ID,
        "product_id":         in.ProductID,
        "finding_count":      len(in.FindingIDs),
        "expiration_date":    formatDate(in.ExpirationDate),
        "_service":           "finding-service",
    })
    return ra, nil
}
```

### `internal/usecase/riskacceptance/expire.go`

```go
package riskacceptance

import (
    "context"
    "log/slog"
    "time"
    "github.com/osv/services/finding-service/internal/domain/riskacceptance"
    findinguc "github.com/osv/services/finding-service/internal/usecase/finding"
)

// ExpireRiskAcceptancesUseCase — run daily at 06:00 via cron scheduler
type ExpireRiskAcceptancesUseCase struct {
    raRepo      riskacceptance.Repository
    findingRepo finding.Repository
    reopenUC    *findinguc.ReopenFindingUseCase
    noteAddUC   *findinguc.AddNoteUseCase
    eventPub    EventPublisher
}

func (uc *ExpireRiskAcceptancesUseCase) Execute(ctx context.Context) error {
    today := time.Now().UTC().Truncate(24 * time.Hour)
    expiring, err := uc.raRepo.ListExpiring(ctx, today)
    if err != nil {
        return err
    }

    for _, ra := range expiring {
        if err := uc.raRepo.UpdateExpired(ctx, ra.ID); err != nil {
            slog.ErrorContext(ctx, "failed to mark RA as expired", "ra_id", ra.ID, "error", err)
            continue
        }

        uc.eventPub.Publish(ctx, "risk_acceptance.expired", map[string]any{
            "risk_acceptance_id": ra.ID,
            "product_id":         ra.ProductID,
            "finding_ids":        ra.FindingIDs,
            "reactivate":         ra.ReactivateExpired,
            "_service":           "finding-service",
        })

        if !ra.ReactivateExpired {
            continue // findings stay in RiskAccepted state
        }

        // Reactivate findings
        for _, fid := range ra.FindingIDs {
            if err := uc.reopenUC.Execute(ctx, findinguc.ReopenFindingInput{
                FindingID:   fid,
                RequesterID: "system",
            }); err != nil {
                slog.ErrorContext(ctx, "failed to reactivate finding after RA expiry", "finding_id", fid)
                continue
            }
            // Add note
            if ra.ReactivateNoteText != "" {
                uc.noteAddUC.Execute(ctx, findinguc.AddNoteInput{
                    FindingID: fid,
                    AuthorID:  "system",
                    Content:   ra.ReactivateNoteText,
                })
            }
        }
    }
    return nil
}
```

### `internal/delivery/http/risk_acceptance_handler.go`

```go
package http

// RegisterRoutes:
// GET    /api/v2/risk-acceptances
// POST   /api/v2/risk-acceptances
// GET    /api/v2/risk-acceptances/{id}
// PUT    /api/v2/risk-acceptances/{id}
// DELETE /api/v2/risk-acceptances/{id}
// POST   /api/v2/risk-acceptances/{id}/findings/{fid}/remove

// POST /api/v2/risk-acceptances body:
// {
//   "name": "Accept Log4Shell risk until patch",
//   "product_id": "uuid",
//   "findings": ["uuid1", "uuid2"],
//   "expiration_date": "2026-12-31",
//   "reactivate_expired": true,
//   "reactivate_note": "Risk acceptance expired — finding reactivated",
//   "restart_sla_on_reactivation": true,
//   "notes": "Mitigated by WAF rule until library updated"
// }
```

## Acceptance Criteria

- [x] `POST /api/v2/risk-acceptances` bởi Owner → 201 Created
- [x] `POST /api/v2/risk-acceptances` bởi Reader → 403
- [x] Tất cả findings trong request → state = RiskAccepted
- [x] NATS `risk_acceptance.created` event published
- [x] `ExpireRiskAcceptancesUseCase` chạy daily: RAs với expiration_date <= today → marked expired
- [x] `reactivate_expired=true` → findings reactivated sau khi RA expired
- [x] `reactivate_note` được thêm vào mỗi finding khi reactivated
- [x] `reactivate_expired=false` → findings KHÔNG bị reactivate sau expiry
- [x] NATS `risk_acceptance.expired` published với finding_ids
- [x] `DELETE /api/v2/risk-acceptances/{id}/findings/{fid}/remove` → finding removed từ RA

## Implementation Status: ✅ DONE

> `finding-service/internal/domain/riskacceptance/entity.go` — RiskAcceptance entity + riskacceptance.Repository interface
> `finding-service/internal/usecase/riskacceptance/risk_acceptance.go` — CreateRiskAcceptanceUseCase, ExpireRiskAcceptancesUseCase, RemoveFindingFromRAUseCase
> `finding-service/internal/delivery/http/router.go` — routes: GET/POST /risk-acceptances, GET/PUT/DELETE /{id}, POST /{id}/findings/{fid}/remove
