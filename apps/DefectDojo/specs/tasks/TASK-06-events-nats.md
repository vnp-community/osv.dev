# TASK-06: NATS JetStream Event System

**Phase**: 6 — Event Bus  
**Ước tính**: 8 giờ  
**Phụ thuộc**: TASK-02 (NATS connection)  
**Output**: JetStream setup, event types, publishers, consumers

---

## Mục tiêu

Implement NATS JetStream event bus cho tất cả async workflows: scan completion, finding creation, SLA breach, AI triage, JIRA sync, v.v.

---

## T-06.1: JetStream Setup

**File**: `apps/DefectDojo/internal/events/setup.go`  
**Ước tính**: 1h

```go
// Package events manages NATS JetStream streams and consumer configurations.
package events

import (
    "errors"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
)

const (
    StreamName    = "DD_EVENTS"
    StreamSubject = "dd.>"
)

// SetupJetStream creates the DD_EVENTS stream if it doesn't exist.
func SetupJetStream(js nats.JetStreamContext) error {
    _, err := js.AddStream(&nats.StreamConfig{
        Name:       StreamName,
        Subjects:   []string{StreamSubject},
        Storage:    nats.FileStorage,
        Replicas:   1,
        MaxAge:     7 * 24 * time.Hour,   // 7-day retention
        MaxMsgs:    5_000_000,
        MaxBytes:   10 * 1024 * 1024 * 1024, // 10GB
        Discard:    nats.DiscardOld,
        Retention:  nats.LimitsPolicy,
        Duplicates: 5 * time.Minute,      // Dedup window
    })

    if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
        return fmt.Errorf("create stream %s: %w", StreamName, err)
    }
    return nil
}
```

**Tasks**:
- [ ] Stream creation với retention policy
- [ ] Dedup window để prevent duplicate processing
- [ ] `go build` pass

---

## T-06.2: Event Types

**File**: `apps/DefectDojo/internal/events/types.go`  
**Ước tính**: 1.5h

```go
package events

import "time"

// Subject constants — all events must use these.
const (
    // Scan lifecycle
    SubjScanSubmitted  = "dd.scan.submitted"
    SubjScanCompleted  = "dd.scan.completed"
    SubjScanFailed     = "dd.scan.failed"

    // Finding lifecycle
    SubjFindingCreated  = "dd.finding.created"
    SubjFindingUpdated  = "dd.finding.updated"
    SubjFindingClosed   = "dd.finding.closed"
    SubjFindingCritical = "dd.finding.severity.critical"

    // SLA
    SubjSLAWarning = "dd.finding.sla.warning"
    SubjSLABreach  = "dd.finding.sla.breach"

    // AI
    SubjAITriageRequest   = "dd.ai.triage.request"
    SubjAITriageCompleted = "dd.ai.triage.completed"

    // Reports
    SubjReportRequested  = "dd.report.requested"
    SubjReportCompleted  = "dd.report.completed"

    // Vulnerabilities
    SubjVulnIngested = "dd.vuln.ingested"

    // Integrations
    SubjJIRACreate  = "dd.integration.jira.create"
    SubjJIRAUpdate  = "dd.integration.jira.update"
    SubjJIRAClose   = "dd.integration.jira.close"
    SubjGitHubIssue = "dd.integration.github.issue"
)

// ── Scan Events ────────────────────────────────────────────────────────────

type ScanSubmittedEvent struct {
    ScanID       string    `json:"scan_id"`
    TestID       string    `json:"test_id"`
    EngagementID string    `json:"engagement_id"`
    ProductID    string    `json:"product_id"`
    ScanType     string    `json:"scan_type"`
    FileSize     int64     `json:"file_size"`
    SubmittedAt  time.Time `json:"submitted_at"`
}

type ScanCompletedEvent struct {
    ScanID         string    `json:"scan_id"`
    TestID         string    `json:"test_id"`
    EngagementID   string    `json:"engagement_id"`
    ProductID      string    `json:"product_id"`
    FindingsCreated int      `json:"findings_created"`
    FindingsClosed  int      `json:"findings_closed"`
    Duration        float64  `json:"duration_seconds"`
    CompletedAt     time.Time `json:"completed_at"`
}

// ── Finding Events ─────────────────────────────────────────────────────────

type FindingCreatedEvent struct {
    FindingID    string `json:"finding_id"`
    Title        string `json:"title"`
    Severity     string `json:"severity"`
    CVE          string `json:"cve,omitempty"`
    ProductID    string `json:"product_id"`
    EngagementID string `json:"engagement_id"`
    TestID       string `json:"test_id"`
    Active       bool   `json:"active"`
}

type FindingClosedEvent struct {
    FindingID string    `json:"finding_id"`
    Reason    string    `json:"reason"` // "mitigated"|"false_positive"|"risk_accepted"
    ClosedAt  time.Time `json:"closed_at"`
    ClosedBy  string    `json:"closed_by_id"`
}

// ── SLA Events ─────────────────────────────────────────────────────────────

type SLAEvent struct {
    FindingID     string    `json:"finding_id"`
    Severity      string    `json:"severity"`
    ProductID     string    `json:"product_id"`
    BreachType    string    `json:"breach_type"` // "warning"|"breached"
    DaysRemaining int       `json:"days_remaining"`
    ExpiresAt     time.Time `json:"expires_at"`
}

// ── AI Events ──────────────────────────────────────────────────────────────

type AITriageRequest struct {
    FindingID   string `json:"finding_id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Severity    string `json:"severity"`
    CVE         string `json:"cve,omitempty"`
}

type AITriageCompleted struct {
    FindingID      string  `json:"finding_id"`
    PriorityScore  float64 `json:"priority_score"`
    AISeverity     string  `json:"ai_severity"`
    Reasoning      string  `json:"reasoning"`
    Remediation    string  `json:"remediation"`
}

// ── Report Events ──────────────────────────────────────────────────────────

type ReportRequestedEvent struct {
    ReportID   string `json:"report_id"`
    Type       string `json:"type"`       // "product"|"engagement"|"findings_list"
    Format     string `json:"format"`     // "pdf"|"html"|"csv"|"json"
    Scope      string `json:"scope_id"`   // product_id or engagement_id
    RequestedBy string `json:"requested_by_id"`
}

type ReportCompletedEvent struct {
    ReportID   string `json:"report_id"`
    URL        string `json:"download_url"`
    Size       int64  `json:"size_bytes"`
    CompletedAt time.Time `json:"completed_at"`
}

// ── Integration Events ─────────────────────────────────────────────────────

type JIRACreateEvent struct {
    FindingID    string `json:"finding_id"`
    ProjectID    string `json:"jira_project_id"`
    Priority     string `json:"priority"`
    Title        string `json:"title"`
    Description  string `json:"description"`
}

type VulnIngestedEvent struct {
    VulnID     string `json:"vuln_id"`
    Source     string `json:"source"` // "nvd"|"osv"|"ghsa"
    Severity   string `json:"severity"`
    AffectedCount int `json:"affected_package_count"`
}
```

**Tasks**:
- [ ] Tất cả subject constants
- [ ] Event structs với JSON tags
- [ ] `go build` pass

---

## T-06.3: Publisher

**File**: `apps/DefectDojo/internal/events/publisher.go`  
**Ước tính**: 2h

```go
package events

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/defectdojo/apps/defectdojo/internal/metrics"
    "github.com/nats-io/nats.go"
)

// Publisher publishes events to NATS JetStream.
type Publisher struct {
    js nats.JetStreamContext
}

func NewPublisher(nc *nats.Conn) (*Publisher, error) {
    js, err := nc.JetStream()
    if err != nil {
        return nil, fmt.Errorf("publisher: jetstream: %w", err)
    }
    return &Publisher{js: js}, nil
}

// publish is the internal publish method.
func (p *Publisher) publish(subject string, v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        metrics.NATSMessages.WithLabelValues(subject, "marshal_error").Inc()
        return fmt.Errorf("marshal %s: %w", subject, err)
    }
    if _, err := p.js.Publish(subject, data); err != nil {
        metrics.NATSMessages.WithLabelValues(subject, "failed").Inc()
        return fmt.Errorf("publish %s: %w", subject, err)
    }
    metrics.NATSMessages.WithLabelValues(subject, "published").Inc()
    return nil
}

// ScanSubmitted publishes when a scan is submitted for processing.
func (p *Publisher) ScanSubmitted(ctx context.Context, evt ScanSubmittedEvent) error {
    return p.publish(SubjScanSubmitted, evt)
}

// ScanCompleted publishes when scan processing is done.
func (p *Publisher) ScanCompleted(ctx context.Context, evt ScanCompletedEvent) error {
    return p.publish(SubjScanCompleted, evt)
}

// FindingCreated publishes for each new finding.
func (p *Publisher) FindingCreated(ctx context.Context, evt FindingCreatedEvent) error {
    if err := p.publish(SubjFindingCreated, evt); err != nil {
        return err
    }
    // Also publish to severity-specific subject
    if evt.Severity == "Critical" {
        return p.publish(SubjFindingCritical, evt)
    }
    return nil
}

// FindingClosed publishes when a finding is closed.
func (p *Publisher) FindingClosed(ctx context.Context, evt FindingClosedEvent) error {
    return p.publish(SubjFindingClosed, evt)
}

// SLAWarning publishes SLA warning events.
func (p *Publisher) SLAWarning(ctx context.Context, evt SLAEvent) error {
    return p.publish(SubjSLAWarning, evt)
}

// SLABreach publishes SLA breach events.
func (p *Publisher) SLABreach(ctx context.Context, evt SLAEvent) error {
    return p.publish(SubjSLABreach, evt)
}

// AITriageRequest requests AI triage for a finding.
func (p *Publisher) AITriageRequest(ctx context.Context, evt AITriageRequest) error {
    return p.publish(SubjAITriageRequest, evt)
}

// AITriageCompleted publishes AI triage results.
func (p *Publisher) AITriageCompleted(ctx context.Context, evt AITriageCompleted) error {
    return p.publish(SubjAITriageCompleted, evt)
}

// ReportRequested requests report generation.
func (p *Publisher) ReportRequested(ctx context.Context, evt ReportRequestedEvent) error {
    return p.publish(SubjReportRequested, evt)
}

// ReportCompleted notifies report is ready for download.
func (p *Publisher) ReportCompleted(ctx context.Context, evt ReportCompletedEvent) error {
    return p.publish(SubjReportCompleted, evt)
}

// JIRACreate requests JIRA issue creation.
func (p *Publisher) JIRACreate(ctx context.Context, evt JIRACreateEvent) error {
    return p.publish(SubjJIRACreate, evt)
}

// VulnIngested notifies a new vulnerability was ingested.
func (p *Publisher) VulnIngested(ctx context.Context, evt VulnIngestedEvent) error {
    return p.publish(SubjVulnIngested, evt)
}
```

**Tasks**:
- [ ] Publisher với all event methods
- [ ] Metrics counters (published/failed)
- [ ] Critical finding → dual publish (finding.created + finding.severity.critical)
- [ ] `go build` pass

---

## T-06.4: Consumer Builder

**File**: `apps/DefectDojo/internal/events/consumer.go`  
**Ước tính**: 2h

```go
package events

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog/log"
)

// ConsumerConfig holds NATS JetStream consumer configuration.
type ConsumerConfig struct {
    Subject       string
    QueueGroup    string        // Durable queue group name
    DurableName   string        // Durable consumer name
    MaxDeliver    int           // Max delivery attempts
    AckWait       time.Duration // How long to wait for ack
    MaxAckPending int           // Max concurrent unacked messages
}

// Subscribe creates a durable JetStream queue subscriber.
func Subscribe[T any](
    js nats.JetStreamContext,
    cfg ConsumerConfig,
    handler func(ctx context.Context, evt T) error,
) (func(), error) {

    sub, err := js.QueueSubscribe(
        cfg.Subject,
        cfg.QueueGroup,
        func(msg *nats.Msg) {
            var evt T
            if err := json.Unmarshal(msg.Data, &evt); err != nil {
                log.Error().
                    Str("subject", cfg.Subject).
                    Err(err).
                    Msg("unmarshal event failed")
                msg.Nak()
                return
            }

            ctx := context.Background()
            if err := handler(ctx, evt); err != nil {
                log.Error().
                    Str("subject", cfg.Subject).
                    Err(err).
                    Msg("event handler failed")
                msg.NakWithDelay(5 * time.Second)
                return
            }
            msg.Ack()
        },
        nats.Durable(cfg.DurableName),
        nats.DeliverNew(),
        nats.AckExplicit(),
        nats.MaxDeliver(cfg.MaxDeliver),
        nats.AckWait(cfg.AckWait),
        nats.MaxAckPending(cfg.MaxAckPending),
    )
    if err != nil {
        return nil, fmt.Errorf("subscribe %s: %w", cfg.Subject, err)
    }

    drain := func() {
        sub.Drain()
    }
    return drain, nil
}

// Standard consumer configs per service/subject combination.
var (
    ScanCompletedForFinding = ConsumerConfig{
        Subject:       SubjScanCompleted,
        QueueGroup:    "finding-service",
        DurableName:   "finding-svc-scan-completed",
        MaxDeliver:    5,
        AckWait:       30 * time.Second,
        MaxAckPending: 10,
    }

    FindingCreatedForNotification = ConsumerConfig{
        Subject:       SubjFindingCreated,
        QueueGroup:    "notification-service",
        DurableName:   "notif-svc-finding-created",
        MaxDeliver:    3,
        AckWait:       10 * time.Second,
        MaxAckPending: 50,
    }

    FindingCriticalForAI = ConsumerConfig{
        Subject:       SubjFindingCritical,
        QueueGroup:    "ai-service",
        DurableName:   "ai-svc-finding-critical",
        MaxDeliver:    3,
        AckWait:       60 * time.Second,
        MaxAckPending: 5,
    }

    SLABreachForNotification = ConsumerConfig{
        Subject:       SubjSLABreach,
        QueueGroup:    "notification-svc-sla",
        DurableName:   "notif-svc-sla-breach",
        MaxDeliver:    3,
        AckWait:       10 * time.Second,
        MaxAckPending: 20,
    }

    FindingCriticalForIntegration = ConsumerConfig{
        Subject:       SubjFindingCritical,
        QueueGroup:    "integration-service",
        DurableName:   "integ-svc-finding-critical",
        MaxDeliver:    5,
        AckWait:       30 * time.Second,
        MaxAckPending: 10,
    }

    VulnIngestedForSearch = ConsumerConfig{
        Subject:       SubjVulnIngested,
        QueueGroup:    "search-service",
        DurableName:   "search-svc-vuln-ingested",
        MaxDeliver:    3,
        AckWait:       15 * time.Second,
        MaxAckPending: 20,
    }
)
```

**Tasks**:
- [ ] Generic `Subscribe[T]` function
- [ ] All consumer configs defined
- [ ] Metrics for consumed/failed
- [ ] Drain function returned for cleanup
- [ ] `go build` pass

---

## T-06.5: Usage in Runners

**Ước tính**: 1.5h

Update từng runner để sử dụng Publisher và Consumer.

### Finding Runner integration:
```go
// In FindingRunner.Run():
pub := events.NewPublisher(nc)

// Subscribe to scan.completed
drain, err := events.Subscribe(js, events.ScanCompletedForFinding,
    func(ctx context.Context, evt events.ScanCompletedEvent) error {
        return findingUC.ProcessScanCompleted(ctx, evt)
    })
defer drain()

// When finding is created, publish event
// (finding usecase calls pub.FindingCreated internally)
```

### Notification Runner integration:
```go
// Subscribe to multiple subjects
drain1, _ := events.Subscribe(js, events.FindingCreatedForNotification,
    func(ctx context.Context, evt events.FindingCreatedEvent) error {
        return notifUC.HandleFindingCreated(ctx, evt)
    })
defer drain1()

drain2, _ := events.Subscribe(js, events.SLABreachForNotification,
    func(ctx context.Context, evt events.SLAEvent) error {
        return notifUC.HandleSLABreach(ctx, evt)
    })
defer drain2()
```

**Tasks**:
- [ ] Finding runner uses Publisher để publish FindingCreated events
- [ ] Scan runner publishes ScanCompleted sau khi process xong
- [ ] Notification runner subscribes to FindingCreated + SLABreach
- [ ] AI runner subscribes to FindingCritical
- [ ] Integration runner subscribes to FindingCritical + SLABreach
- [ ] SLA checker (finding-service) publishes SLAWarning + SLABreach

---

## Definition of Done — TASK-06

- [ ] T-06.1 JetStream stream created và persistent
- [ ] T-06.2 Tất cả event types + subjects defined
- [ ] T-06.3 Publisher với tất cả event methods
- [ ] T-06.4 Generic consumer với standard configs
- [ ] T-06.5 Tất cả runners integrated với pub/sub
- [ ] Event flow E2E: scan import → finding created → notification triggered
- [ ] `go build ./...` pass
- [ ] Unit tests cho publisher và consumer
