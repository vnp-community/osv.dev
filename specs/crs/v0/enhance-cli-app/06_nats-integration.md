# 06 — NATS Integration (Event Flow)

> **Mục đích**: Mô tả chi tiết NATS JetStream event flow giữa apps và services.

---

## 1. Tổng quan Event Flow

```
apps/cli/cmd/importer
    │
    │  PUBLISH: osv.vuln.imported
    ▼
NATS JetStream
    │
    ├──► data-service/consumer.go          (upsert vuln to MongoDB/PostgreSQL)
    │        │
    │        │  PUBLISH: osv.vuln.updated
    │        ▼
    │    NATS JetStream
    │        │
    │        ├──► search-service/consumer.go   (reindex in OpenSearch/pgvector)
    │        │
    │        └──► ai-service/consumer.go       (trigger enrichment if needed)
    │                 │
    │                 │  PUBLISH: osv.ai.enrichment.completed
    │                 ▼
    │             NATS JetStream
    │                 │
    │                 └──► search-service/consumer.go  (update vector index)
    │
    └──► ai-service/consumer.go            (pre-enrich on import)


apps/cli/cmd/worker (scan result processing)
    │
    │  PUBLISH: osv.scan.completed
    ▼
NATS JetStream
    │
    └──► finding-service/consumer.go       (create findings from scan)
             │
             │  PUBLISH: defectdojo.finding.created
             ▼
         NATS JetStream
             │
             └──► notification-service/consumer.go  (alert matching rules)
                      │
                      │  PUBLISH: finding.sla.breached (if SLA violated)
                      ▼
                  notification-service/inapp_handler.go  (SSE push)
```

---

## 2. JetStream Streams Configuration

```go
// services/shared/pkg/nats/streams.go  ← NEW file

package nats

import natsio "github.com/nats-io/nats.go"

// StreamConfig defines all JetStream streams for the OSV platform.
var StreamConfigs = []natsio.StreamConfig{
    {
        Name:      "OSV_VULN",
        Subjects:  []string{"osv.vuln.*"},
        Storage:   natsio.FileStorage,
        Retention: natsio.LimitsPolicy,
        MaxAge:    7 * 24 * time.Hour,  // 7 days retention
        Replicas:  1,
    },
    {
        Name:     "OSV_AI",
        Subjects: []string{"osv.ai.*"},
        Storage:  natsio.FileStorage,
        MaxAge:   24 * time.Hour,
    },
    {
        Name:     "OSV_SCAN",
        Subjects: []string{"osv.scan.*"},
        Storage:  natsio.FileStorage,
        MaxAge:   72 * time.Hour,
    },
    {
        Name:     "DEFECTDOJO",
        Subjects: []string{"defectdojo.*", "finding.*"},
        Storage:  natsio.FileStorage,
        MaxAge:   30 * 24 * time.Hour,  // 30 days
    },
}

// SetupStreams creates all streams if they don't exist.
func SetupStreams(js natsio.JetStreamContext) error {
    for _, cfg := range StreamConfigs {
        _, err := js.AddStream(&cfg)
        if err != nil && !errors.Is(err, natsio.ErrStreamNameAlreadyInUse) {
            return fmt.Errorf("stream %s: %w", cfg.Name, err)
        }
    }
    return nil
}
```

---

## 3. apps/cli/importer — NATS Publisher

### Thêm `internal/importer/nats_publisher.go`

```go
package importer

import (
    "context"
    "encoding/json"
    "time"

    "github.com/nats-io/nats.go"
    sharedEvents "github.com/osv/shared/pkg/events"
)

// NATSPublisher replaces GCPPublisher when CLI_BACKEND=microservices.
// Publishes to NATS JetStream subject "osv.vuln.imported".
type NATSPublisher struct {
    js nats.JetStreamContext
}

// NewNATSPublisher creates a publisher connected to NATS JetStream.
func NewNATSPublisher(nc *nats.Conn) (*NATSPublisher, error) {
    js, err := nc.JetStream()
    if err != nil {
        return nil, fmt.Errorf("jetstream context: %w", err)
    }
    // Ensure stream exists
    if err := natsSetup.SetupStreams(js); err != nil {
        return nil, err
    }
    return &NATSPublisher{js: js}, nil
}

// Publish sends a vulnerability import event to NATS.
// Converts from GCP PubSub message format to NATS CloudEvent.
func (p *NATSPublisher) Publish(ctx context.Context, id string, data []byte) error {
    event := sharedEvents.VulnImportedEvent{
        ID:         id,
        Source:     "cli-importer",
        ImportedAt: time.Now().UTC(),
        OSVData:    data,
    }
    payload, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }
    _, err = p.js.Publish(sharedEvents.SubjectVulnImported, payload,
        nats.Context(ctx),
    )
    return err
}
```

---

## 4. data-service — Vuln Event Consumer

Hiện đã có `data-service/internal/infra/messaging/nats/alias_consumer.go`.

**Thêm** `cve_import_consumer.go` để handle vuln import:

```go
// data-service/internal/infra/messaging/nats/cve_import_consumer.go  ← NEW
package nats

// CVEImportConsumer subscribes to osv.vuln.imported and upserts to storage.
type CVEImportConsumer struct {
    js       nats.JetStreamContext
    ingestUC *ingest.UseCase
    log      zerolog.Logger
}

func (c *CVEImportConsumer) Start(ctx context.Context) error {
    sub, err := c.js.Subscribe(events.SubjectVulnImported, func(msg *nats.Msg) {
        var event events.VulnImportedEvent
        if err := json.Unmarshal(msg.Data, &event); err != nil {
            c.log.Error().Err(err).Msg("invalid vuln import event")
            msg.Nak()
            return
        }
        
        if err := c.ingestUC.IngestOSVRecord(ctx, event.ID, event.OSVData); err != nil {
            c.log.Error().Err(err).Str("id", event.ID).Msg("ingest failed")
            msg.NakWithDelay(5 * time.Second)
            return
        }
        msg.Ack()
    }, nats.Durable("data-service-import"), nats.AckExplicit())
    
    if err != nil {
        return err
    }
    defer sub.Unsubscribe()
    
    <-ctx.Done()
    return nil
}
```

---

## 5. ai-service — CVE Updated Consumer

Hiện đã có `ai-service/internal/infra/messaging/nats/consumer.go`.

**Thêm handler** cho `osv.vuln.updated` subject:

```go
// Thêm vào consumer.go (hoặc thêm file mới cve_updated_consumer.go):

func (c *Consumer) handleVulnUpdated(msg *nats.Msg) {
    var event events.VulnUpdatedEvent
    json.Unmarshal(msg.Data, &event)
    
    // Trigger re-enrichment if description changed
    if event.DescriptionChanged {
        go c.enrichUC.Execute(context.Background(), event.CVEID)
    }
    msg.Ack()
}
```

---

## 6. NATS Event Subjects Reference

| Subject | Publisher | Consumer | Description |
|---------|-----------|----------|-------------|
| `osv.vuln.imported` | cli/importer | data-service | New vuln from external source |
| `osv.vuln.updated` | data-service | search-service, ai-service | Vuln data changed |
| `osv.vuln.withdrawn` | data-service | search-service | Vuln retracted |
| `osv.ai.enrichment.completed` | ai-service | search-service | Enrichment done → reindex |
| `osv.scan.completed` | scan-service | finding-service | Scan result available |
| `defectdojo.finding.created` | finding-service | notification-service | New security finding |
| `defectdojo.finding.status_changed` | finding-service | notification-service | Finding state change |
| `finding.sla.breached` | finding-service | notification-service | SLA violation alert |
| `finding.risk.accepted` | finding-service | notification-service | Risk accepted event |
