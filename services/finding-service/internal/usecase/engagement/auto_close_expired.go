package engagement_usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/osv/finding-service/internal/domain/repository"
	natsutil "github.com/osv/shared/pkg/nats"
)

// AutoCloseExpiredEngagementsUseCase — run daily at 07:00
// Closes engagements past their end_date (type=Interactive)
type AutoCloseExpiredEngagementsUseCase struct {
	engagementRepo repository.EngagementRepository
	eventPub       *natsutil.Publisher
}

func NewAutoCloseExpired(repo repository.EngagementRepository, pub *natsutil.Publisher) *AutoCloseExpiredEngagementsUseCase {
	return &AutoCloseExpiredEngagementsUseCase{
		engagementRepo: repo,
		eventPub:       pub,
	}
}

func (uc *AutoCloseExpiredEngagementsUseCase) Execute(ctx context.Context) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	expired, err := uc.engagementRepo.ListExpiredOpen(ctx, today)
	if err != nil {
		return err
	}

	for _, eng := range expired {
		eng.Close()
		if err := uc.engagementRepo.Update(ctx, eng); err != nil {
			slog.ErrorContext(ctx, "failed to close expired engagement",
				"engagement_id", eng.ID, "error", err)
			continue
		}
		_ = uc.eventPub.Publish(ctx, "defectdojo.engagement.closed", map[string]interface{}{
			"engagement_id": eng.ID,
			"product_id":    eng.ProductID,
			"reason":        "auto_expired",
		})
	}
	return nil
}
