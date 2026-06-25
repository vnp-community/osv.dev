package consumer

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"

    "github.com/osv/finding-service/internal/domain/finding"
    "github.com/osv/finding-service/internal/usecase/dedup"
)

const (
    subject       = "scan.scan.completed"
    consumerGroup = "finding-service"
    maxRetries    = 3
)

// ScanCompletedEvent is the event payload from scan-service.
type ScanCompletedEvent struct {
    ScanID      uuid.UUID       `json:"scan_id"`
    ProductID   uuid.UUID       `json:"product_id"`
    EngagementID uuid.UUID      `json:"engagement_id"`
    TestID      uuid.UUID       `json:"test_id"`
    Findings    []RawFinding    `json:"findings"`
    CompletedAt time.Time       `json:"completed_at"`
}

// RawFinding is a finding as reported by scan-service.
type RawFinding struct {
    Title            string   `json:"title"`
    Severity         string   `json:"severity"`
    CVE              string   `json:"cve"`
    CVSSv3Score      *float64 `json:"cvss_v3_score"`
    ComponentName    string   `json:"component_name"`
    ComponentVersion string   `json:"component_version"`
    Description      string   `json:"description"`
    DataSource       string   `json:"data_source"` // "nmap"|"zap"|"agent"
}

// FindingRepository defines storage for persisting new findings.
type FindingRepository interface {
    Save(ctx context.Context, f *finding.Finding) error
    FindByHashInProduct(ctx context.Context, hash string, productID uuid.UUID) (*finding.Finding, error)
    FindByHashInEngagement(ctx context.Context, hash string, engagementID uuid.UUID) (*finding.Finding, error)
    FindByHashGlobal(ctx context.Context, hash string) (*finding.Finding, error)
}

// NATSConsumer consumes scan.scan.completed events and creates findings.
type NATSConsumer struct {
    nc          *nats.Conn
    findingRepo FindingRepository
    dedupSvc    *dedup.Service
    logger      zerolog.Logger
}

// New creates a NATS consumer for scan completion events.
func New(nc *nats.Conn, findingRepo FindingRepository, dedupSvc *dedup.Service, logger zerolog.Logger) *NATSConsumer {
    return &NATSConsumer{
        nc:          nc,
        findingRepo: findingRepo,
        dedupSvc:    dedupSvc,
        logger:      logger,
    }
}

// Subscribe starts consuming scan.scan.completed events from NATS JetStream.
func (c *NATSConsumer) Subscribe(ctx context.Context) error {
    js, err := c.nc.JetStream()
    if err != nil {
        return fmt.Errorf("jetstream: %w", err)
    }

    _, err = js.QueueSubscribe(subject, consumerGroup, func(msg *nats.Msg) {
        if err := c.handleMessage(ctx, msg); err != nil {
            c.logger.Error().Err(err).Msg("handle scan.completed failed")
            // NAK with delay for retry
            msg.NakWithDelay(5 * time.Second)
            return
        }
        msg.Ack()
    }, nats.Durable(consumerGroup), nats.AckExplicit())

    if err != nil {
        return fmt.Errorf("subscribe: %w", err)
    }

    c.logger.Info().Str("subject", subject).Msg("NATS consumer started")
    return nil
}

// handleMessage processes a single scan.completed event.
func (c *NATSConsumer) handleMessage(ctx context.Context, msg *nats.Msg) error {
    var event ScanCompletedEvent
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        return fmt.Errorf("unmarshal event: %w", err)
    }

    c.logger.Info().
        Str("scan_id", event.ScanID.String()).
        Int("findings", len(event.Findings)).
        Msg("processing scan.completed event")

    findings := make([]*finding.Finding, 0, len(event.Findings))

    for _, raw := range event.Findings {
        f, err := finding.NewFinding(
            raw.Title,
            finding.Severity(raw.Severity),
            event.TestID,
            event.EngagementID,
            event.ProductID,
            raw.ComponentName,
            raw.ComponentVersion,
            raw.CVE,
        )
        if err != nil {
            c.logger.Warn().Err(err).Str("title", raw.Title).Msg("skip invalid finding")
            continue
        }
        f.CVSSv3Score = raw.CVSSv3Score
        f.Description = raw.Description
        findings = append(findings, f)
    }

    // Run dedup
    scope := dedup.ScopeProduct
    stats, err := c.dedupSvc.ProcessBatch(ctx, findings, scope)
    if err != nil {
        return fmt.Errorf("dedup batch: %w", err)
    }

    // Persist findings
    savedCount := 0
    for _, f := range findings {
        if err := c.findingRepo.Save(ctx, f); err != nil {
            c.logger.Error().Err(err).Str("id", f.ID.String()).Msg("save finding failed")
            continue
        }
        savedCount++
    }

    c.logger.Info().
        Int("total", stats.TotalProcessed).
        Int("new", stats.NewFindings).
        Int("duplicate", stats.Duplicates).
        Int("saved", savedCount).
        Msg("scan findings processed")

    return nil
}
