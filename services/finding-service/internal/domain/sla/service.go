package sla

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/osv/finding-service/internal/domain/finding"
)

// FindingRepository defines storage needed for SLA operations.
type FindingRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*finding.Finding, error)
    Update(ctx context.Context, f *finding.Finding) error
    FindActiveDueSLA(ctx context.Context, before time.Time) ([]*finding.Finding, error)
}

// EventBus publishes SLA events.
type EventBus interface {
    Publish(ctx context.Context, subject string, data interface{}) error
}

// Service manages SLA deadline computation and breach detection.
type Service struct {
    repo     FindingRepository
    eventBus EventBus
    logger   zerolog.Logger
}

// New creates the SLA Service.
func New(repo FindingRepository, eventBus EventBus, logger zerolog.Logger) *Service {
    return &Service{repo: repo, eventBus: eventBus, logger: logger}
}

// SetDeadline computes and stores the SLA deadline for a finding.
// Called when a finding is created.
func (s *Service) SetDeadline(ctx context.Context, f *finding.Finding) error {
    days, ok := DefaultSLADays[string(f.Severity)]
    if !ok {
        return nil // No SLA for this severity
    }

    deadline := f.Date.UTC().Add(time.Duration(days) * 24 * time.Hour)
    f.SLAExpirationDate = &deadline
    f.UpdatedAt = time.Now().UTC()

    return s.repo.Update(ctx, f)
}

// CheckBreaches scans for findings that have breached their SLA deadline.
// Run periodically via cron (e.g., every hour).
func (s *Service) CheckBreaches(ctx context.Context) (int, error) {
    now := time.Now().UTC()

    // Find all active findings with SLA deadline in the past
    overdue, err := s.repo.FindActiveDueSLA(ctx, now)
    if err != nil {
        return 0, fmt.Errorf("find overdue findings: %w", err)
    }

    breachCount := 0
    for _, f := range overdue {
        if f.SLAExpirationDate == nil || now.Before(*f.SLAExpirationDate) {
            continue
        }

        daysOverdue := int(now.Sub(*f.SLAExpirationDate).Hours() / 24)

        s.logger.Warn().
            Str("finding_id", f.ID.String()).
            Str("severity", string(f.Severity)).
            Int("days_overdue", daysOverdue).
            Msg("SLA breach detected")

        // Publish SLA breach event
        s.eventBus.Publish(ctx, "finding.sla.breached", map[string]interface{}{
            "finding_id":   f.ID,
            "title":        f.Title,
            "severity":     f.Severity,
            "days_overdue": daysOverdue,
            "product_id":   f.ProductID,
        })

        breachCount++
    }

    return breachCount, nil
}

// DaysUntilSLA computes how many days remain before the SLA deadline.
// Returns nil if no SLA is set.
func (s *Service) DaysUntilSLA(f *finding.Finding) *int {
    return f.DaysUntilSLA()
}

// SLASummary returns SLA status for a batch of findings.
type SLASummary struct {
    TotalActive  int
    SLABreached  int
    SLADueIn7d   int
    SLADueIn30d  int
    NoSLA        int
}

// Summarize computes SLA status across a list of findings.
func Summarize(findings []*finding.Finding) SLASummary {
    now := time.Now().UTC()
    summary := SLASummary{TotalActive: len(findings)}

    for _, f := range findings {
        if f.SLAExpirationDate == nil {
            summary.NoSLA++
            continue
        }

        diff := f.SLAExpirationDate.Sub(now)
        days := int(diff.Hours() / 24)

        switch {
        case days < 0:
            summary.SLABreached++
        case days <= 7:
            summary.SLADueIn7d++
        case days <= 30:
            summary.SLADueIn30d++
        }
    }

    return summary
}
