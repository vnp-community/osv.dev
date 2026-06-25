package scheduler

import (
	"context"
	"time"

	"github.com/osv/notification-service/internal/domain/repository"
	"github.com/osv/notification-service/internal/usecase"
	"github.com/rs/zerolog"
)

// RetryWorker polls for failed deliveries and retries them.
type RetryWorker struct {
	webhookRepo repository.WebhookRepository
	deliverer   *usecase.WebhookDeliverer
	logger      zerolog.Logger
	interval    time.Duration
}

func NewRetryWorker(repo repository.WebhookRepository, deliverer *usecase.WebhookDeliverer, log zerolog.Logger) *RetryWorker {
	return &RetryWorker{
		webhookRepo: repo,
		deliverer:   deliverer,
		logger:      log,
		interval:    1 * time.Minute,
	}
}

// Run starts the retry worker. Blocks until ctx is cancelled.
func (w *RetryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info().Msg("retry worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("retry worker stopped")
			return
		case <-ticker.C:
			w.processRetries(ctx)
		}
	}
}

func (w *RetryWorker) processRetries(ctx context.Context) {
	deliveries, err := w.webhookRepo.GetPendingRetries(ctx)
	if err != nil {
		w.logger.Error().Err(err).Msg("retry worker: get pending retries failed")
		return
	}
	w.logger.Debug().Int("count", len(deliveries)).Msg("retry worker: processing")

	for _, d := range deliveries {
		select {
		case <-ctx.Done():
			return
		default:
			w.deliverer.Retry(ctx, d)
		}
	}
}
