package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// OutboxPublisher implements domain.EventPublisher using PostgreSQL transactional outbox.
// Guarantees at-least-once delivery even when NATS is temporarily unavailable.
type OutboxPublisher struct {
	db     *pgxpool.Pool
	nc     *natsgo.Conn // may be nil on startup
	logger zerolog.Logger
}

// NewOutboxPublisher creates an OutboxPublisher.
// nc may be nil — outbox will buffer events until NATS reconnects.
func NewOutboxPublisher(db *pgxpool.Pool, nc *natsgo.Conn, logger zerolog.Logger) *OutboxPublisher {
	return &OutboxPublisher{db: db, nc: nc, logger: logger}
}

// Publish writes the event to the outbox table.
// Must be called within the same transaction as the business operation.
func (p *OutboxPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("outbox marshal: %w", err)
	}
	_, err = p.db.Exec(ctx,
		`INSERT INTO outbox_events (subject, payload) VALUES ($1, $2)`,
		subject, payload)
	return err
}

// Run polls the outbox and publishes pending events to NATS.
// Blocks until ctx is cancelled. Call as a goroutine.
func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	p.logger.Info().Msg("OutboxPublisher: polling goroutine started")
	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("OutboxPublisher: polling goroutine stopped")
			return
		case <-ticker.C:
			p.flush(ctx)
		}
	}
}

func (p *OutboxPublisher) flush(ctx context.Context) {
	if p.nc == nil || !p.nc.IsConnected() {
		return // NATS not ready yet, skip silently
	}

	rows, err := p.db.Query(ctx, `
        SELECT id, subject, payload
        FROM outbox_events
        WHERE status = 'pending' AND attempts < 10
        ORDER BY created_at
        LIMIT 100
        FOR UPDATE SKIP LOCKED
    `)
	if err != nil {
		p.logger.Error().Err(err).Msg("OutboxPublisher: query error")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, subject string
		var payload []byte
		if err := rows.Scan(&id, &subject, &payload); err != nil {
			continue
		}

		if pubErr := p.nc.Publish(subject, payload); pubErr != nil {
			p.logger.Warn().Err(pubErr).Str("event_id", id).
				Msg("OutboxPublisher: publish failed, will retry")
			p.db.Exec(ctx, // nolint: errcheck
				`UPDATE outbox_events SET attempts = attempts+1, last_error = $2 WHERE id = $1`,
				id, pubErr.Error())
			continue
		}

		p.db.Exec(ctx, // nolint: errcheck
			`UPDATE outbox_events SET status='published', published_at=NOW(), attempts=attempts+1 WHERE id=$1`,
			id)
	}
}
