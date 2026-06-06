// Package register provides the webhook registration use case.
package register

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/globalcve/notification-service/internal/domain/aggregate/webhook"
	"github.com/globalcve/notification-service/internal/domain/repository"
)

// Request holds registration input.
type Request struct {
	URL     string
	Events  []string
	Secret  string
	OwnerID string
}

// UseCase handles webhook registration.
type UseCase struct {
	repo repository.WebhookRepository
	log  zerolog.Logger
}

// New creates a register UseCase.
func New(repo repository.WebhookRepository, log zerolog.Logger) *UseCase {
	return &UseCase{repo: repo, log: log.With().Str("usecase", "webhook.register").Logger()}
}

// Execute validates and persists a new webhook.
func (uc *UseCase) Execute(ctx context.Context, req *Request) (*webhook.Webhook, error) {
	w, err := webhook.New(req.URL, req.Events, req.Secret, req.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("register webhook: %w", err)
	}
	if err := uc.repo.Save(ctx, w); err != nil {
		return nil, fmt.Errorf("register webhook save: %w", err)
	}
	uc.log.Info().Str("id", w.ID()).Str("owner", req.OwnerID).Msg("webhook registered")
	return w, nil
}
