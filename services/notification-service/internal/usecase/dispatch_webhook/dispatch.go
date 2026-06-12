// Package dispatch provides the webhook event dispatch use case.
package dispatch

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/notification-service/internal/adapter/dispatcher"
	"github.com/osv/notification-service/internal/domain/aggregate/webhook"
	"github.com/osv/notification-service/internal/domain/repository"
)

// UseCase handles dispatching CVE events to all subscribed active webhooks.
type UseCase struct {
	repo       repository.WebhookRepository
	disp       *dispatcher.HTTPDispatcher
	log        zerolog.Logger
}

// New creates a dispatch UseCase.
func New(repo repository.WebhookRepository, d *dispatcher.HTTPDispatcher, log zerolog.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		disp: d,
		log:  log.With().Str("usecase", "webhook.dispatch").Logger(),
	}
}

// DispatchEvent finds all active webhooks subscribed to eventType and delivers
// the event concurrently. Failures are logged but not propagated (best-effort).
func (uc *UseCase) DispatchEvent(ctx context.Context, eventType string, data interface{}) error {
	webhooks, err := uc.repo.FindActiveByEvent(ctx, eventType)
	if err != nil {
		return err
	}
	if len(webhooks) == 0 {
		uc.log.Debug().Str("event", eventType).Msg("no active webhooks for event")
		return nil
	}

	event := &dispatcher.CVEEvent{
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	var wg sync.WaitGroup
	for _, wh := range webhooks {
		wg.Add(1)
		go func(w *webhook.Webhook) {
			defer wg.Done()
			dispCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := uc.disp.Dispatch(dispCtx, w, event); err != nil {
				uc.log.Warn().
					Err(err).
					Str("webhook_id", w.ID()).
					Str("event", eventType).
					Msg("dispatch failed (best-effort)")
			}
		}(wh)
	}
	wg.Wait()
	return nil
}
