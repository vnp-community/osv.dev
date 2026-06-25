package notification

import (
	"context"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/google/uuid"

	deliverhttp "github.com/osv/notification-service/internal/delivery/http"
	infrapostgres "github.com/osv/notification-service/internal/infra/postgres"
	"github.com/osv/notification-service/internal/infra/persistence/postgres"
	"github.com/osv/notification-service/internal/usecase"
)

// AlertRepoAdapter adapts PostgresAlertRepo to satisfy delivery/http.AlertRepository.
type AlertRepoAdapter struct {
	repo *postgres.PostgresAlertRepo
}

func (a *AlertRepoAdapter) ListByUser(ctx context.Context, userID, isRead string, limit, offset int) ([]*deliverhttp.Alert, int, int, error) {
	rows, total, unread, err := a.repo.ListAlertsByUser(ctx, userID, isRead, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	alerts := make([]*deliverhttp.Alert, len(rows))
	for i, r := range rows {
		alerts[i] = &deliverhttp.Alert{
			ID:         uuid.MustParse(r.ID),
			Type:       r.Type,
			Title:      r.Title,
			Message:    r.Message,
			Severity:   r.Severity,
			IsRead:     r.IsRead,
			EntityType: r.EntityType,
			EntityID:   r.EntityID,
			CreatedAt:  r.CreatedAt,
			ReadAt:     r.ReadAt,
		}
	}
	return alerts, total, unread, nil
}

func (a *AlertRepoAdapter) CountUnread(ctx context.Context, userID string) (int, error) {
	return a.repo.CountUnreadByStringID(ctx, userID)
}

func (a *AlertRepoAdapter) MarkRead(ctx context.Context, alertID, userID string) error {
	return a.repo.MarkReadByStringIDs(ctx, alertID, userID)
}

func (a *AlertRepoAdapter) MarkAllRead(ctx context.Context, userID string) (int, error) {
	return a.repo.MarkAllReadByStringID(ctx, userID)
}

// WireEmbedded initializes the Notification service routes on the provided ServeMux.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	opt, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(opt)

	// Initialize Repositories
	webhookRepo := infrapostgres.NewWebhookRepository(pool)
	subRepo := infrapostgres.NewSubscriptionRepository(pool)

	// Initialize UseCases
	registerUC := usecase.NewRegisterWebhookUseCase(webhookRepo)
	deliverer := usecase.NewWebhookDeliverer(webhookRepo, redisClient)
	dispatcher := usecase.NewAlertDispatcher(webhookRepo, subRepo, deliverer)

	// Initialize HTTP Handlers & Router
	whHandler := deliverhttp.NewWebhookHandler(registerUC, deliverer, webhookRepo)
	shHandler := deliverhttp.NewSubscriptionHandler(subRepo)
	ihHandler := deliverhttp.NewInternalHandler(dispatcher)
	ruleRepo := postgres.NewRuleRepo(pool, logger)
	rhHandler := deliverhttp.NewRuleHandler(ruleRepo, logger)

	alertRepo := postgres.NewAlertRepo(pool, logger)
	alertAdapter := &AlertRepoAdapter{repo: alertRepo}
	ahHandler := deliverhttp.NewAlertsHandler(alertAdapter)

	// CR-009: DeliveryHandler — flat delivery list, retry, hourly stats
	// Uses pgxpool directly to query webhook_deliveries table.
	dhHandler := deliverhttp.NewDeliveryHandler(pool)

	// SetupRouter mounts all routes including CR-009 delivery endpoints.
	r := deliverhttp.SetupRouter(whHandler, shHandler, ihHandler, ahHandler, nil, rhHandler, dhHandler)

	mux.Handle("/", r)

	return nil
}

