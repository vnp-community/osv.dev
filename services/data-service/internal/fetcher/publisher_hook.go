// Package fetcher — publisher_hook.go
// PublisherHook wraps NVDCVEFetcher to publish NATS events after each fetch.
//
// This file is ADDITIVE — it does NOT modify NVDCVEFetcher.
// Instead it wraps it with an optional NATS publisher layer.
//
// Usage in main.go:
//
//	nvdFetcher := fetcher.NewNVDCVEFetcher(db, apiKey, startYear)
//	if cvePublisher != nil {
//	    nvdFetcher = fetcher.WithCVEPublisher(nvdFetcher, cvePublisher, logger)
//	}
package fetcher

import (
	"context"

	"github.com/rs/zerolog"

	natspkg "github.com/osv/data-service/internal/infra/messaging/nats"
	"time"
)

// PublishingFetcher wraps any Fetcher to publish NATS events after each fetch.
// If publisher is nil it behaves identically to the wrapped fetcher.
type PublishingFetcher struct {
	inner     Fetcher
	publisher *natspkg.CVEEventPublisher
	log       zerolog.Logger
}

// WithCVEPublisher wraps a Fetcher with NATS event publishing.
// publisher may be nil — in that case the wrapper is a no-op passthrough.
func WithCVEPublisher(f Fetcher, publisher *natspkg.CVEEventPublisher, log zerolog.Logger) Fetcher {
	if publisher == nil {
		return f // no-op passthrough: don't wrap if no publisher
	}
	return &PublishingFetcher{inner: f, publisher: publisher, log: log}
}

// Name delegates to the wrapped fetcher.
func (p *PublishingFetcher) Name() string { return p.inner.Name() }

// FetchAndStore delegates to the wrapped fetcher, then publishes a CVEImportedEvent.
func (p *PublishingFetcher) FetchAndStore(ctx context.Context, opts FetchOptions) (int, error) {
	count, err := p.inner.FetchAndStore(ctx, opts)
	if err != nil {
		return count, err
	}

	// Publish non-blocking after successful fetch
	if count > 0 {
		p.publisher.PublishImported(ctx, natspkg.CVEImportedEvent{
			ID:       p.inner.Name(), // fetcher-level event (not per-CVE)
			Source:   p.inner.Name(),
			SyncedAt: time.Now().UTC(),
		})
		p.log.Debug().
			Str("source", p.inner.Name()).
			Int("count", count).
			Msg("cve_publisher: published import event")
	}

	return count, nil
}
