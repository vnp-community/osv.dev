// Package slaconfig_uc provides use cases for SLA configuration management.
package slaconfig_uc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ─── Domain types ─────────────────────────────────────────────────────────────

// SLAConfiguration holds SLA days per severity.
type SLAConfiguration struct {
	ID          uuid.UUID
	Name        string
	Description string
	Critical    int
	High        int
	Medium      int
	Low         int
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SLAProductAssignment links a product to an SLA config.
type SLAProductAssignment struct {
	ProductID          uuid.UUID
	SLAConfigurationID uuid.UUID
	AssignedAt         time.Time
	AssignedBy         uuid.UUID
}

// ─── Repository interfaces ────────────────────────────────────────────────────

// Repository is the SLA config persistence layer.
type Repository interface {
	Save(ctx context.Context, cfg *SLAConfiguration) error
	FindByID(ctx context.Context, id uuid.UUID) (*SLAConfiguration, error)
	FindDefault(ctx context.Context) (*SLAConfiguration, error)
	List(ctx context.Context) ([]*SLAConfiguration, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// AssignmentRepository manages product ↔ SLA config mappings.
type AssignmentRepository interface {
	Save(ctx context.Context, a *SLAProductAssignment) error
	FindByProductID(ctx context.Context, productID uuid.UUID) (*SLAProductAssignment, error)
	CountAssignments(ctx context.Context, slaConfigID uuid.UUID) (int, error)
}

// EventPublisher publishes NATS events.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload map[string]any) error
}

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrInvalidDays       = errors.New("SLA days must be > 0")
	ErrInvalidOrdering   = errors.New("SLA days must be in order: critical ≤ high ≤ medium ≤ low")
	ErrSLAConfigNotFound = errors.New("SLA configuration not found")
	ErrConfigInUse       = errors.New("SLA configuration is assigned to products and cannot be deleted")
)

// ─── Create ───────────────────────────────────────────────────────────────────

// CreateSLAConfigInput is the request for creating an SLA configuration.
type CreateSLAConfigInput struct {
	Name        string
	Description string
	Critical    int
	High        int
	Medium      int
	Low         int
	IsDefault   bool
}

// CreateSLAConfigUseCase creates a new SLA configuration.
type CreateSLAConfigUseCase struct {
	repo     Repository
	eventPub EventPublisher
}

// NewCreate creates a new CreateSLAConfigUseCase.
func NewCreate(r Repository, ep EventPublisher) *CreateSLAConfigUseCase {
	return &CreateSLAConfigUseCase{repo: r, eventPub: ep}
}

// Execute validates and creates a new SLA configuration.
func (uc *CreateSLAConfigUseCase) Execute(ctx context.Context, in CreateSLAConfigInput) (*SLAConfiguration, error) {
	// Validate: all days > 0
	if in.Critical <= 0 || in.High <= 0 || in.Medium <= 0 || in.Low <= 0 {
		return nil, ErrInvalidDays
	}
	// Validate: severity ordering critical ≤ high ≤ medium ≤ low
	if in.Critical > in.High || in.High > in.Medium || in.Medium > in.Low {
		return nil, ErrInvalidOrdering
	}

	now := time.Now().UTC()
	cfg := &SLAConfiguration{
		ID:          uuid.New(),
		Name:        in.Name,
		Description: in.Description,
		Critical:    in.Critical,
		High:        in.High,
		Medium:      in.Medium,
		Low:         in.Low,
		IsDefault:   in.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.repo.Save(ctx, cfg); err != nil {
		return nil, err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.sla.config.created", map[string]any{
		"sla_config_id": cfg.ID.String(),
		"name":          cfg.Name,
		"critical_days": cfg.Critical,
		"high_days":     cfg.High,
		"medium_days":   cfg.Medium,
		"low_days":      cfg.Low,
		"_service":      "sla-service",
	})
	return cfg, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

// UpdateSLAConfigInput is the request for updating an SLA configuration.
type UpdateSLAConfigInput struct {
	ID          uuid.UUID
	Name        string
	Description string
	Critical    int
	High        int
	Medium      int
	Low         int
	IsDefault   bool
}

// UpdateSLAConfigUseCase updates an existing SLA configuration.
type UpdateSLAConfigUseCase struct {
	repo     Repository
	eventPub EventPublisher
}

// NewUpdate creates a new UpdateSLAConfigUseCase.
func NewUpdate(r Repository, ep EventPublisher) *UpdateSLAConfigUseCase {
	return &UpdateSLAConfigUseCase{repo: r, eventPub: ep}
}

// Execute updates the SLA configuration.
func (uc *UpdateSLAConfigUseCase) Execute(ctx context.Context, in UpdateSLAConfigInput) (*SLAConfiguration, error) {
	if in.Critical > in.High || in.High > in.Medium || in.Medium > in.Low {
		return nil, ErrInvalidOrdering
	}

	existing, err := uc.repo.FindByID(ctx, in.ID)
	if err != nil || existing == nil {
		return nil, ErrSLAConfigNotFound
	}

	existing.Name = in.Name
	existing.Description = in.Description
	existing.Critical = in.Critical
	existing.High = in.High
	existing.Medium = in.Medium
	existing.Low = in.Low
	existing.IsDefault = in.IsDefault
	existing.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Save(ctx, existing); err != nil {
		return nil, err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.sla.config.updated", map[string]any{
		"sla_config_id": existing.ID.String(),
	})
	return existing, nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

// DeleteSLAConfigUseCase deletes an SLA configuration.
type DeleteSLAConfigUseCase struct {
	repo           Repository
	assignmentRepo AssignmentRepository
}

// NewDelete creates a new DeleteSLAConfigUseCase.
func NewDelete(r Repository, ar AssignmentRepository) *DeleteSLAConfigUseCase {
	return &DeleteSLAConfigUseCase{repo: r, assignmentRepo: ar}
}

// Execute deletes the SLA configuration if it has no product assignments.
func (uc *DeleteSLAConfigUseCase) Execute(ctx context.Context, id uuid.UUID) error {
	// Guard: cannot delete if assigned to products
	count, err := uc.assignmentRepo.CountAssignments(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConfigInUse
	}
	return uc.repo.Delete(ctx, id)
}

// ─── Assign Product ───────────────────────────────────────────────────────────

// AssignProductInput is the request for assigning a product to an SLA config.
type AssignProductInput struct {
	ProductID          uuid.UUID
	SLAConfigurationID uuid.UUID
	AssignedByID       uuid.UUID
}

// AssignProductUseCase assigns a product to a specific SLA configuration.
type AssignProductUseCase struct {
	configRepo     Repository
	assignmentRepo AssignmentRepository
	eventPub       EventPublisher
}

// NewAssignProduct creates a new AssignProductUseCase.
func NewAssignProduct(cr Repository, ar AssignmentRepository, ep EventPublisher) *AssignProductUseCase {
	return &AssignProductUseCase{configRepo: cr, assignmentRepo: ar, eventPub: ep}
}

// Execute saves the product ↔ SLA config assignment.
func (uc *AssignProductUseCase) Execute(ctx context.Context, in AssignProductInput) error {
	// Verify config exists
	cfg, err := uc.configRepo.FindByID(ctx, in.SLAConfigurationID)
	if err != nil || cfg == nil {
		return ErrSLAConfigNotFound
	}

	assignment := &SLAProductAssignment{
		ProductID:          in.ProductID,
		SLAConfigurationID: in.SLAConfigurationID,
		AssignedAt:         time.Now().UTC(),
		AssignedBy:         in.AssignedByID,
	}
	if err := uc.assignmentRepo.Save(ctx, assignment); err != nil {
		return err
	}

	_ = uc.eventPub.Publish(ctx, "defectdojo.sla.config.updated", map[string]any{
		"product_id":           in.ProductID.String(),
		"sla_configuration_id": in.SLAConfigurationID.String(),
		"_service":             "sla-service",
	})
	return nil
}
