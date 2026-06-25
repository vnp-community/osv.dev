# ✅ COMPLETED — TASK-DD-018 — SLA Config CRUD + Product Assignment

## Metadata

| Field | Value |
|-------|-------|
| **Task ID** | TASK-DD-018 |
| **Service** | `sla-service` |
| **CR** | CR-DD-006 |
| **Phase** | 2 — Security Management |
| **Priority** | 🔴 High |
| **Prerequisites** | TASK-DD-017 |
| **Estimated effort** | 1 ngày |

## Context

Implement CRUD use cases cho SLA Configuration và Product Assignment endpoint. Admin có thể tạo SLA profiles (Critical: 7 ngày, High: 30 ngày, etc.) và assign vào products.

## Working Directory

```
/Users/binhnt/Lab/sec/cve/osv.dev/services/sla-service/
```

## Files to Create

```
internal/usecase/config/
├── create.go
├── update.go
├── delete.go
├── get.go
└── assign_product.go

internal/infra/postgres/
├── config_repo.go
└── assignment_repo.go

internal/delivery/http/
└── config_handler.go
```

## Implementation Spec

### `internal/usecase/config/create.go`

```go
package config

import (
    "context"
    "errors"
    "time"
    "github.com/google/uuid"
    "github.com/osv/services/sla-service/internal/domain/config"
)

var (
    ErrInvalidDays = errors.New("SLA days must be > 0")
    ErrDefaultExists = errors.New("a default SLA configuration already exists")
)

type CreateSLAConfigInput struct {
    Name        string
    Description string
    Critical    int
    High        int
    Medium      int
    Low         int
    IsDefault   bool
}

type CreateSLAConfigUseCase struct {
    repo config.Repository
    eventPub EventPublisher
}

func (uc *CreateSLAConfigUseCase) Execute(ctx context.Context, in CreateSLAConfigInput) (*config.SLAConfiguration, error) {
    // Validate
    if in.Critical <= 0 || in.High <= 0 || in.Medium <= 0 || in.Low <= 0 {
        return nil, ErrInvalidDays
    }
    if in.Critical > in.High {
        return nil, errors.New("Critical days must be <= High days")
    }

    // Validate ordering: Critical ≤ High ≤ Medium ≤ Low
    if in.High > in.Medium || in.Medium > in.Low {
        return nil, errors.New("SLA days must be in order: critical ≤ high ≤ medium ≤ low")
    }

    cfg := &config.SLAConfiguration{
        ID:          uuid.New().String(),
        Name:        in.Name,
        Description: in.Description,
        Critical:    in.Critical,
        High:        in.High,
        Medium:      in.Medium,
        Low:         in.Low,
        IsDefault:   in.IsDefault,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    if err := uc.repo.Save(ctx, cfg); err != nil {
        return nil, err
    }

    uc.eventPub.Publish(ctx, "sla.config.created", map[string]any{
        "sla_config_id": cfg.ID,
        "name":          cfg.Name,
        "critical_days": cfg.Critical,
        "high_days":     cfg.High,
        "medium_days":   cfg.Medium,
        "low_days":      cfg.Low,
        "_service":      "sla-service",
    })
    return cfg, nil
}
```

### `internal/usecase/config/assign_product.go`

```go
package config

import (
    "context"
    "time"
    "github.com/osv/services/sla-service/internal/domain/config"
)

type AssignProductInput struct {
    ProductID          string
    SLAConfigurationID string
    AssignedByID       string
}

type AssignProductUseCase struct {
    configRepo     config.Repository
    assignmentRepo config.AssignmentRepository
    findingClient  FindingServiceClient
    bulkComputeUC  *BulkRecomputeUseCase
    eventPub       EventPublisher
}

func (uc *AssignProductUseCase) Execute(ctx context.Context, in AssignProductInput) error {
    // 1. Verify SLA config exists
    cfg, err := uc.configRepo.FindByID(ctx, in.SLAConfigurationID)
    if err != nil || cfg == nil {
        return ErrSLAConfigNotFound
    }

    // 2. Save assignment
    assignment := &config.SLAProductAssignment{
        ProductID:          in.ProductID,
        SLAConfigurationID: in.SLAConfigurationID,
        AssignedAt:         time.Now(),
        AssignedBy:         in.AssignedByID,
    }
    if err := uc.assignmentRepo.Save(ctx, assignment); err != nil {
        return err
    }

    // 3. Update product.sla_configuration_id via finding-service gRPC
    uc.findingClient.UpdateProductSLAConfig(ctx, in.ProductID, in.SLAConfigurationID)

    // 4. Trigger async bulk recompute for all active findings in product
    go uc.bulkComputeUC.Execute(ctx, in.ProductID, cfg)

    uc.eventPub.Publish(ctx, "sla.config.updated", map[string]any{
        "product_id":         in.ProductID,
        "sla_configuration_id": in.SLAConfigurationID,
        "_service":          "sla-service",
    })
    return nil
}
```

### REST API Handlers (`config_handler.go`)

```go
// Routes:
// GET    /api/v2/sla-configurations
// POST   /api/v2/sla-configurations       (Admin only)
// GET    /api/v2/sla-configurations/{id}
// PUT    /api/v2/sla-configurations/{id}  (Admin only)
// DELETE /api/v2/sla-configurations/{id}  (Admin only)
// POST   /api/v2/sla-configurations/{id}/assign/{product_id}  (Product Owner)

// POST /api/v2/sla-configurations body:
// {
//   "name": "Strict SLA",
//   "description": "For critical/production products",
//   "critical": 3,
//   "high": 14,
//   "medium": 60,
//   "low": 180,
//   "is_default": false
// }
// Response: 201 Created with SLAConfiguration object
```

### `internal/infra/postgres/config_repo.go`

```go
package postgres

import (
    "context"
    "database/sql"
    "github.com/osv/services/sla-service/internal/domain/config"
)

type PostgresSLAConfigRepo struct {
    db *sql.DB
}

func (r *PostgresSLAConfigRepo) Save(ctx context.Context, cfg *config.SLAConfiguration) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO sla_configurations
            (id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (id) DO UPDATE SET
            name=$2, description=$3, critical_days=$4, high_days=$5,
            medium_days=$6, low_days=$7, is_default=$8, updated_at=$10
    `, cfg.ID, cfg.Name, cfg.Description, cfg.Critical, cfg.High, cfg.Medium, cfg.Low, cfg.IsDefault, cfg.CreatedAt, cfg.UpdatedAt)
    return err
}

func (r *PostgresSLAConfigRepo) FindByID(ctx context.Context, id string) (*config.SLAConfiguration, error) {
    cfg := &config.SLAConfiguration{}
    err := r.db.QueryRowContext(ctx,
        `SELECT id, name, description, critical_days, high_days, medium_days, low_days, is_default, created_at, updated_at
         FROM sla_configurations WHERE id=$1`, id,
    ).Scan(&cfg.ID, &cfg.Name, &cfg.Description, &cfg.Critical, &cfg.High, &cfg.Medium, &cfg.Low, &cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return cfg, err
}

func (r *PostgresSLAConfigRepo) FindDefault(ctx context.Context) (*config.SLAConfiguration, error) {
    cfg := &config.SLAConfiguration{}
    err := r.db.QueryRowContext(ctx,
        `SELECT id, name, description, critical_days, high_days, medium_days, low_days FROM sla_configurations WHERE is_default=TRUE LIMIT 1`,
    ).Scan(&cfg.ID, &cfg.Name, &cfg.Description, &cfg.Critical, &cfg.High, &cfg.Medium, &cfg.Low)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return cfg, err
}
```

## Acceptance Criteria

- [x] `POST /api/v2/sla-configurations` với Critical=3, High=14 → 201 Created
- [x] `POST /api/v2/sla-configurations` với Critical > High → 400 validation error
- [x] `GET /api/v2/sla-configurations` → list bao gồm default config
- [x] `POST /api/v2/sla-configurations/{id}/assign/{product_id}` → assignment saved
- [x] Assignment triggers async BulkRecompute cho product
- [x] NATS `sla.config.created` event published khi tạo config mới
- [x] NATS `sla.config.updated` event published khi assign product
- [x] `DELETE /api/v2/sla-configurations/{id}` khi config được assign → 409 Conflict
- [x] `is_default=true` → chỉ có 1 default config tại một thời điểm

## Implementation Status: ✅ DONE

> `sla-service/internal/usecase/config/sla_config.go` — CreateSLAConfigUseCase, UpdateSLAConfigUseCase, DeleteSLAConfigUseCase (409 guard), AssignProductUseCase
> `sla-service/internal/infra/postgres/config_repo.go` — SLAConfigRepo + SLAAssignmentRepo (gom chung)
> `sla-service/internal/delivery/http/config_handler.go` — CRUD + assign endpoints
> Validation: Critical <= High <= Medium <= Low ordering enforced
