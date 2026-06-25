// Package slaconfigdomain defines the SLA configuration domain entities.
package slaconfigdomain

import (
	"time"

	"github.com/google/uuid"
)

// SLAConfiguration defines remediation deadlines per severity.
// Each product can be assigned one SLA configuration.
type SLAConfiguration struct {
	ID          uuid.UUID
	Name        string
	Description string

	// Days to remediate by severity
	CriticalDays int // typically 7
	HighDays     int // typically 30
	MediumDays   int // typically 90
	LowDays      int // typically 365

	// If true, products without an explicit assignment use this config
	IsDefault bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new SLAConfiguration with the given name and days.
func New(name, description string, critical, high, medium, low int) *SLAConfiguration {
	now := time.Now().UTC()
	return &SLAConfiguration{
		ID:           uuid.New(),
		Name:         name,
		Description:  description,
		CriticalDays: critical,
		HighDays:     high,
		MediumDays:   medium,
		LowDays:      low,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// DaysForSeverity returns the remediation days for a given severity string.
func (c *SLAConfiguration) DaysForSeverity(severity string) int {
	switch severity {
	case "Critical":
		return c.CriticalDays
	case "High":
		return c.HighDays
	case "Medium":
		return c.MediumDays
	case "Low":
		return c.LowDays
	default:
		return 0 // Info = no SLA
	}
}

// SLAProductAssignment links a product to an SLA configuration.
type SLAProductAssignment struct {
	ProductID          uuid.UUID
	SLAConfigurationID uuid.UUID
	AssignedAt         time.Time
	AssignedBy         uuid.UUID
}
