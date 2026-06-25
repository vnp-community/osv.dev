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
	List(ctx context.Context, filter FindingFilter) (*FindingListResult, error)
	Save(ctx context.Context, f *Finding) error
	Delete(ctx context.Context, id uuid.UUID) error
	BulkUpdateAssignee(ctx context.Context, findingIDs []string, assignedTo string) (int, error)
	GetStats(ctx context.Context, productID string) (map[string]interface{}, error)
	BulkSetMitigated(ctx context.Context, ids []uuid.UUID, mitigatedByID uuid.UUID) error
	BulkUpdateSLADates(ctx context.Context, updates []SLADateUpdate) error
	BulkReactivate(ctx context.Context, ids []uuid.UUID) error
	ListForSLACheck(ctx context.Context, ids []uuid.UUID, activeOnly, hasSLADate bool) ([]*Finding, error)
	// ListForReport returns a channel of findings for streaming (avoids OOM).
	ListForReport(ctx context.Context, filter FindingFilter) (<-chan *Finding, error)
	// ExistsFalsePositiveByHash checks if a false-positive finding exists with this hash code.
	// Used by the dedup engine to auto-mark new findings as FP when a previous FP with the same hash exists.
	ExistsFalsePositiveByHash(ctx context.Context, hashCode string, productID uuid.UUID) (bool, error)
	// GetSeverityCounts returns the count of findings grouped by severity for a product.
	GetSeverityCounts(ctx context.Context, productID uuid.UUID, activeOnly bool) (map[string]int, error)
}


// FindingFilter defines query parameters for listing findings.
type FindingFilter struct {
	ProductID    *uuid.UUID
	EngagementID *uuid.UUID
	TestID       *uuid.UUID
	Severity     []Severity
	// Status filters by derived status string: "active", "resolved", "false_positive",
	// "accepted", "out_of_scope", "duplicate". Maps to boolean flag combinations.
	// Supports multi-value: ?status[]=active&status[]=resolved
	Status       []string
	ActiveOnly   bool
	VerifiedOnly bool
	Search       string
	Limit        int
	Offset       int
}

// FindingListResult holds the paginated findings and aggregated metadata
type FindingListResult struct {
	Findings        []*FindingWithMeta
	Total           int
	SevCritical     int
	SevHigh         int
	SevMedium       int
	SevLow          int
	StatusActive    int
	StatusMitigated int
	StatusFP        int
	StatusRisk      int
	SLABreached     int
	SLAAtRisk       int
}

// FindingWithMeta includes the finding and joined metadata
type FindingWithMeta struct {
	Finding      *Finding
	ProductName  string
	JiraIssueKey *string
	JiraURL      *string
}

// SLADateUpdate holds an SLA expiration date update for one finding.
type SLADateUpdate struct {
	FindingID      uuid.UUID
	ExpirationDate interface{} // *time.Time or protobuf Timestamp
}
