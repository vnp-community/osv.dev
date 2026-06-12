// Package nats provides a NATS JetStream consumer for cache invalidation.
// Subscribes to osv.vuln.updated and osv.vuln.withdrawn to evict stale cache entries.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/osv/query-service/internal/domain/repository"
	"github.com/rs/zerolog"
)

const (
	streamName     = "OSV_EVENTS"
	consumerName   = "vuln-query-cache-invalidator"
	subjectPattern = "osv.vuln.>"
)

// invalidationEvent is the minimal shape of vuln domain events.
type invalidationEvent struct {
	VulnID    string `json:"vuln_id"`
	EventType string `json:"event_type"`
}

// CacheInvalidationConsumer subscribes to vulnerability update events and
// removes stale entries from the query cache.
type CacheInvalidationConsumer struct {
	js    jetstream.JetStream
	cache repository.VulnerabilityCache
	log   zerolog.Logger
}

// NewCacheInvalidationConsumer creates a cache invalidation consumer.
func NewCacheInvalidationConsumer(
	js jetstream.JetStream,
	cache repository.VulnerabilityCache,
	log zerolog.Logger,
) *CacheInvalidationConsumer {
	return &CacheInvalidationConsumer{js: js, cache: cache, log: log}
}

// Start subscribes and begins processing cache invalidation messages.
// Blocks until ctx is cancelled.
func (c *CacheInvalidationConsumer) Start(ctx context.Context) error {
	consumer, err := c.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:           consumerName,
		Durable:        consumerName,
		FilterSubject:  subjectPattern,
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     5,
		AckWait:        30 * time.Second,
		DeliverPolicy:  jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s: %w", consumerName, err)
	}

	msgs, err := consumer.Messages()
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subjectPattern, err)
	}

	c.log.Info().Str("subject", subjectPattern).Msg("cache invalidation consumer started")

	go func() {
		<-ctx.Done()
		msgs.Stop()
	}()

	for {
		msg, err := msgs.Next()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			c.log.Error().Err(err).Msg("NATS messages error")
			return err
		}

		if err := c.handleMsg(ctx, msg); err != nil {
			c.log.Warn().Err(err).Msg("cache invalidation failed — nacking")
			msg.Nak() //nolint:errcheck
		} else {
			msg.Ack() //nolint:errcheck
		}
	}
}

func (c *CacheInvalidationConsumer) handleMsg(ctx context.Context, msg jetstream.Msg) error {
	var evt invalidationEvent
	if err := json.Unmarshal(msg.Data(), &evt); err != nil {
		// Not our format — ack and skip
		c.log.Debug().Str("subject", msg.Subject()).Msg("skip non-vuln event")
		return nil
	}

	if evt.VulnID == "" {
		return nil
	}

	cacheKey := "vuln:" + evt.VulnID
	if err := c.cache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("evict cache %s: %w", evt.VulnID, err)
	}

	c.log.Debug().Str("vuln_id", evt.VulnID).Str("event", evt.EventType).Msg("cache evicted")
	return nil
}
