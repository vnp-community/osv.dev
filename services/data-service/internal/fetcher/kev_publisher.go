// Package fetcher — NATS JetStream event publisher for KEV sync events.
package fetcher

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/nats-io/nats.go"
    "github.com/rs/zerolog"
    "github.com/osv/data-service/internal/domain/kev"
)

// KEVPublisher publishes kev.new events to NATS JetStream.
type KEVPublisher struct {
    js     nats.JetStreamContext
    logger zerolog.Logger
}

func NewKEVPublisher(nc *nats.Conn, log zerolog.Logger) (*KEVPublisher, error) {
    js, err := nc.JetStream()
    if err != nil {
        return nil, fmt.Errorf("kev publisher: jetstream: %w", err)
    }

    // Ensure stream exists (idempotent)
    _, err = js.AddStream(&nats.StreamConfig{
        Name:     "KEV_EVENTS",
        Subjects: []string{"kev.>"},
        MaxMsgs:  10000,
    })
    if err != nil && !isStreamExistsError(err) {
        return nil, fmt.Errorf("kev publisher: add stream: %w", err)
    }

    return &KEVPublisher{js: js, logger: log}, nil
}

// PublishNewKEVBatch publishes kev.new events for each new KEV entry.
// Deduplicates using NATS message ID (CVE ID).
func (p *KEVPublisher) PublishNewKEVBatch(ctx context.Context, entries []*kev.KEVEntry) error {
    for _, entry := range entries {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        payload := map[string]interface{}{
            "event":         "kev.new",
            "cve_id":        entry.CVEID,
            "product":       entry.Product,
            "vendor":        entry.VendorProject,
            "date_added":    entry.DateAdded.Format("2006-01-02"),
            "is_ransomware": entry.IsRansomware(),
        }
        data, _ := json.Marshal(payload)

        _, err := p.js.Publish("kev.new", data,
            nats.Context(ctx),
            nats.MsgId(entry.CVEID), // deduplication key
        )
        if err != nil {
            p.logger.Warn().Err(err).Str("cve_id", entry.CVEID).Msg("NATS publish failed")
        }
    }
    return nil
}

func isStreamExistsError(err error) bool {
    return err != nil && err.Error() == nats.ErrStreamNameAlreadyInUse.Error()
}
