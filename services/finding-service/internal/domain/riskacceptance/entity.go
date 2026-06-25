// Package riskacceptance defines the RiskAcceptance domain entity.
// A risk acceptance allows product owners to formally accept known risks
// for a set of findings, optionally with an expiration date.
package riskacceptance

import (
	"time"

	"github.com/google/uuid"
)

// RiskAcceptance represents a formal risk acceptance for a set of findings.
type RiskAcceptance struct {
	ID           uuid.UUID
	Name         string    // descriptive name, e.g. "Accept Log4Shell risk until patch"
	ProductID    uuid.UUID
	AcceptedByID uuid.UUID  // Owner user who accepted the risk

	// Optional expiration — when nil, risk acceptance never expires
	ExpirationDate *time.Time

	// Documentation
	Notes        string
	ProofFileKey string // MinIO object key for supporting document

	// Expiry behavior
	ReactivateExpired        bool   // if true, findings are reactivated when RA expires
	ReactivateNoteText       string // note added to each finding on reactivation
	RestartSLAOnReactivation bool   // if true, reset SLA start date on reactivation

	// State
	IsExpired  bool

	// Linked findings
	FindingIDs []uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new RiskAcceptance.
func New(productID, acceptedByID uuid.UUID, name string) *RiskAcceptance {
	now := time.Now().UTC()
	return &RiskAcceptance{
		ID:           uuid.New(),
		Name:         name,
		ProductID:    productID,
		AcceptedByID: acceptedByID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// IsActive returns true if the risk acceptance has not expired.
func (ra *RiskAcceptance) IsActive() bool {
	if ra.IsExpired {
		return false
	}
	if ra.ExpirationDate == nil {
		return true // no expiry
	}
	return time.Now().UTC().Before(*ra.ExpirationDate)
}
