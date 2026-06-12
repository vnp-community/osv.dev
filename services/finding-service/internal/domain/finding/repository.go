package finding

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines all persistence operations for findings.
type Repository interface {
	Create(ctx context.Context, f *Finding) error
	BulkCreate(ctx context.Context, findings []*Finding) ([]string, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Finding, error)
	FindByHashCode(ctx context.Context, hashCode string, testID uuid.UUID, onEngagement bool, engagementID *uuid.UUID, productID *uuid.UUID) (*Finding, error)
	FindActiveByTest(ctx context.Context, testID uuid.UUID, excludeIDs []uuid.UUID) ([]*Finding, error)
	List(ctx context.Context, filter FindingFilter) ([]*Finding, int, error)
	Save(ctx context.Context, f *Finding) error
	Delete(ctx context.Context, id uuid.UUID) error
	BulkSetMitigated(ctx context.Context, ids []uuid.UUID, mitigatedByID uuid.UUID) error
	BulkUpdateSLADates(ctx context.Context, updates []SLADateUpdate) error
	ListForSLACheck(ctx context.Context, ids []uuid.UUID, activeOnly, hasSLADate bool) ([]*Finding, error)
	// ListForReport returns a channel of findings for streaming (avoids OOM).
	ListForReport(ctx context.Context, filter FindingFilter) (<-chan *Finding, error)
}

// FindingFilter defines query parameters for listing findings.
type FindingFilter struct {
	ProductID    *uuid.UUID
	EngagementID *uuid.UUID
	TestID       *uuid.UUID
	Severity     []Severity
	ActiveOnly   bool
	VerifiedOnly bool
	Search       string
	Limit        int
	Offset       int
}

// SLADateUpdate holds an SLA expiration date update for one finding.
type SLADateUpdate struct {
	FindingID      uuid.UUID
	ExpirationDate interface{} // *time.Time or protobuf Timestamp
}
