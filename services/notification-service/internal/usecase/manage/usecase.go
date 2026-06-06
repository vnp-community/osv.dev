// Package manage provides the webhook management use case (list, delete, stats).
package manage

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/globalcve/notification-service/internal/domain/aggregate/webhook"
	"github.com/globalcve/notification-service/internal/domain/repository"
)

// UseCase handles webhook management operations.
type UseCase struct {
	repo repository.WebhookRepository
	log  zerolog.Logger
}

// New creates a manage UseCase.
func New(repo repository.WebhookRepository, log zerolog.Logger) *UseCase {
	return &UseCase{repo: repo, log: log.With().Str("usecase", "webhook.manage").Logger()}
}

// ListByOwner returns all webhooks belonging to ownerID.
func (uc *UseCase) ListByOwner(ctx context.Context, ownerID string) ([]*webhook.Webhook, error) {
	webhooks, err := uc.repo.FindByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	return webhooks, nil
}

// Delete removes a webhook by id, verifying ownerID.
func (uc *UseCase) Delete(ctx context.Context, id, ownerID string) error {
	if err := uc.repo.Delete(ctx, id, ownerID); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	uc.log.Info().Str("id", id).Str("owner", ownerID).Msg("webhook deleted")
	return nil
}

// GetStats returns aggregate statistics.
func (uc *UseCase) GetStats(ctx context.Context) (map[string]interface{}, error) {
	n, err := uc.repo.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("webhook stats: %w", err)
	}
	return map[string]interface{}{"active_webhooks": n}, nil
}
