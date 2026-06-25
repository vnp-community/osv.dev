package repository

import (
	"context"

	"github.com/osv/notification-service/internal/domain/subscription"
)

type SubscriptionRepository interface {
	Save(ctx context.Context, s *subscription.AlertSubscription) error
	FindByOwner(ctx context.Context, ownerID string) ([]*subscription.AlertSubscription, error)
	FindByVendor(ctx context.Context, vendor string) ([]*subscription.AlertSubscription, error)
	Delete(ctx context.Context, id, ownerID string) error
}
