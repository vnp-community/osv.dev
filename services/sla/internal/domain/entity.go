// Package domain defines the SLA configuration entity and computation logic.
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SLAConfiguration defines remediation time limits per severity for a product.
type SLAConfiguration struct {
	ID        uuid.UUID
	ProductID *uuid.UUID // nil = global default config
	Critical  int        // days to remediate Critical findings
	High      int        // days to remediate High findings
	Medium    int        // days to remediate Medium findings
	Low       int        // days to remediate Low findings
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DefaultSLADays are the industry-standard fallback SLA days.
var DefaultSLADays = map[string]int{
	"Critical": 7,
	"High":     30,
	"Medium":   90,
	"Low":      180,
	"Info":     0, // no SLA for Info
}

// ComputeExpirationDate calculates the SLA deadline for a finding.
// Returns nil if there is no SLA requirement for the given severity.
func (c *SLAConfiguration) ComputeExpirationDate(severity string, findingDate time.Time) *time.Time {
	days := c.daysForSeverity(severity)
	if days == 0 {
		return nil
	}
	exp := findingDate.AddDate(0, 0, days)
	return &exp
}

func (c *SLAConfiguration) daysForSeverity(severity string) int {
	switch severity {
	case "Critical":
		return c.Critical
	case "High":
		return c.High
	case "Medium":
		return c.Medium
	case "Low":
		return c.Low
	default:
		return 0
	}
}

// NewDefault creates a global default SLAConfiguration using industry-standard days.
func NewDefault() *SLAConfiguration {
	return &SLAConfiguration{
		ID:       uuid.New(),
		Critical: DefaultSLADays["Critical"],
		High:     DefaultSLADays["High"],
		Medium:   DefaultSLADays["Medium"],
		Low:      DefaultSLADays["Low"],
	}
}

// SLAConfigRepository defines persistence for SLA configurations.
type SLAConfigRepository interface {
	FindByProductID(ctx context.Context, productID uuid.UUID) (*SLAConfiguration, error)
	FindGlobal(ctx context.Context) (*SLAConfiguration, error)
	Create(ctx context.Context, cfg *SLAConfiguration) error
	Update(ctx context.Context, cfg *SLAConfiguration) error
	List(ctx context.Context) ([]*SLAConfiguration, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
