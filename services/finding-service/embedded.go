package finding

import (
	"context"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	httpdelivery "github.com/osv/finding-service/internal/delivery/http"
	"github.com/osv/finding-service/internal/domain/report"
	"github.com/osv/finding-service/internal/infra/crypto"
	miniorepo "github.com/osv/finding-service/internal/infra/minio"
	mynats "github.com/osv/finding-service/internal/infra/nats"
	"github.com/osv/finding-service/internal/infra/postgres"
	stats_uc "github.com/osv/finding-service/internal/usecase"
	enguc "github.com/osv/finding-service/internal/usecase/engagement"
	findinguc "github.com/osv/finding-service/internal/usecase/finding"
	member_uc "github.com/osv/finding-service/internal/usecase/member"
	reportuc "github.com/osv/finding-service/internal/usecase/report"
	tool_uc "github.com/osv/finding-service/internal/usecase/tool"
	rauc "github.com/osv/finding-service/internal/usecase/riskacceptance"
	natsutil "github.com/osv/shared/pkg/nats"
)

// WireEmbedded initializes the Finding service routes on the provided ServeMux.
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	// Initialize Repositories
	productTypeRepo := postgres.NewProductTypeRepo(pool)
	engagementRepo := postgres.NewEngagementRepo(pool)
	testRepo := postgres.NewTestRepo(pool)
	productRepo := postgres.NewProductRepo(pool)
	findingRepo := postgres.NewFindingRepo(pool)
	findingGroupRepo := postgres.NewFindingGroupRepo(pool)
	noteRepo := postgres.NewNoteRepo(pool)

	// FIX MOCK-005: OutboxPublisher for EventBus (at-least-once delivery)
	// FIX MOCK-003: Wire outboxPub as EventBus for BulkHandler
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	var nc *natsgo.Conn
	nc, err := natsgo.Connect(natsURL,
		natsgo.RetryOnFailedConnect(true),
		natsgo.MaxReconnects(-1), // reconnect forever
	)
	if err != nil {
		logger.Warn().Err(err).Str("url", natsURL).
			Msg("NATS unreachable at startup — OutboxPublisher will buffer events")
		nc = nil
	}

	// OutboxPublisher for BulkHandler EventBus (satisfies EventBus interface)
	outboxPub := mynats.NewOutboxPublisher(pool, nc, logger)
	go outboxPub.Run(ctx) // start background poller

	// natsutil.Publisher for usecases (they require the concrete type)
	pub := mynats.NewNoopPublisher() // safe default
	if nc != nil {
		js, jsErr := jetstream.New(nc)
		if jsErr == nil {
			pub = natsutil.NewPublisher(js, "finding-service/v1")
			logger.Info().Msg("finding-service: NATS JetStream publisher connected")
		} else {
			logger.Warn().Err(jsErr).Msg("finding-service: NATS JetStream init failed, using noop publisher")
		}
	}

	// Initialize UseCases (pub can be nil — they guard with eventPub.Publish only)
	bulkUC := findinguc.NewBulkUpdate(findingRepo, pub)
	statusUC := findinguc.NewStatusTransition(findingRepo, pub)

	// Member repo — declared here because rauc.NewCreate needs it (TASK-005 FIX: was declared after first use)
	memberRepo := postgres.NewMemberRepo(pool)

	// Initialize Handlers
	productSeed := httpdelivery.NewProductSeedHandler(productTypeRepo, productRepo, engagementRepo, testRepo, logger)
	findingSeed := httpdelivery.NewFindingSeedHandler(findingRepo, testRepo, engagementRepo, logger)
	findingGroup := httpdelivery.NewFindingGroupHandler(findingGroupRepo, logger)
	// FIX MOCK-003: pass outboxPub as EventBus (was nil)
	bulkHandler := httpdelivery.NewBulkHandler(bulkUC, findingRepo, outboxPub, logger)
	productHandler := httpdelivery.NewProductHandler(productRepo, logger)
	findingHandler := httpdelivery.NewFindingHandler(findingRepo, statusUC, logger)
	noteHandler := httpdelivery.NewNoteHandler(findingRepo, noteRepo)

	// TASK-005 FIX: Wire RiskAcceptanceHandler (was nil — caused 404 on all /risk-acceptances routes)
	raRepo := postgres.NewRiskAcceptanceRepo(pool)
	// TASK-005 FIX: bridge *natsutil.Publisher → rauc.EventPublisher (map[string]any vs interface{})
	rapub := &raEventPublisher{pub: pub}
	createRAUC := rauc.NewCreate(raRepo, memberRepo, findingRepo, rapub)
	removeRAUC := rauc.NewRemoveFinding(raRepo, memberRepo, findingRepo)
	riskAcceptanceHandler := httpdelivery.NewRiskAcceptanceHandler(createRAUC, removeRAUC).WithRepo(raRepo)

	// FIX MOCK-004: Wire the missing handlers
	
	// Engagement
	engagementHandler := httpdelivery.NewEngagementHandler(
		engagementRepo,
		enguc.NewGetOrCreate(engagementRepo, pub),
		enguc.NewClose(engagementRepo, pub),
		logger,
	)

	// Test
	testHandler := httpdelivery.NewTestHandler(testRepo, logger)

	// Member (already declared above — just build the handler here)
	memberHandler := httpdelivery.NewMemberHandler(
		member_uc.NewAddProductMember(memberRepo),
		member_uc.NewRemoveProductMember(memberRepo),
		member_uc.NewListProductMembers(memberRepo),
		member_uc.NewCheckProductPermission(memberRepo),
		logger,
	)

	// Tool Config
	toolConfigRepo := postgres.NewToolConfigRepo(pool)
	cryptoKey := os.Getenv("TOOL_ENCRYPTION_KEY")
	if cryptoKey == "" {
		cryptoKey = "00000000000000000000000000000000000000000000" // 32-byte key
	}
	cryptoSvc, _ := crypto.NewAES256GCM(cryptoKey)
	toolHandler := httpdelivery.NewToolHandler(
		tool_uc.NewCreateToolConfig(toolConfigRepo, cryptoSvc),
		tool_uc.NewUpdateToolConfig(toolConfigRepo, cryptoSvc),
		tool_uc.NewDeleteToolConfig(toolConfigRepo),
		tool_uc.NewGetToolConfig(toolConfigRepo),
		tool_uc.NewListToolConfigs(toolConfigRepo),
		logger,
	)

	// Internal Stats
	statsUC := stats_uc.NewStatsUseCase(&statsFindingAdapter{repo: findingRepo})
	internalHandler := httpdelivery.NewInternalHandler(statsUC)

	// SLA Dashboard
	slaHandler := httpdelivery.NewSLAHandler(findingRepo)

	// CR-010: ReportHandler — GetTemplates is static (no DB/storage needed).
	// Storage is config-driven: wired when MINIO_ENDPOINT is set.
	var reportRepo report.Repository = postgres.NewReportRepo(pool)
	
	var generateUC *reportuc.GenerateUseCase
	var reportStorage reportuc.Storage

	minioEndpoint  := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioBucket    := os.Getenv("MINIO_REPORT_BUCKET")
	if minioBucket == "" {
		minioBucket = "osv-reports"
	}

	if minioEndpoint != "" && minioAccessKey != "" {
		storage, err := miniorepo.NewReportStorage(
			minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, false,
		)
		if err != nil {
			logger.Warn().Err(err).Msg("MinIO storage init failed, report generation disabled")
		} else {
			reportStorage = storage
			
			fAdapter := &reportFindingAdapter{repo: findingRepo}
			pAdapter := &reportPublisherAdapter{pub: pub}
			generateUC = reportuc.NewGenerate(reportRepo, fAdapter, nil, storage, pAdapter)
			
			logger.Info().Str("bucket", minioBucket).Msg("Report generation enabled via MinIO")
		}
	} else {
		logger.Warn().Msg("MINIO_ENDPOINT not set — report generation disabled")
	}

	reportHandler := httpdelivery.NewReportHandler(generateUC, reportRepo, reportStorage)

	// Build the router with available handlers
	router := httpdelivery.NewRouter(
		findingHandler,     // handler *FindingHandler
		bulkHandler,        // bulk *BulkHandler
		noteHandler,        // note *NoteHandler
		engagementHandler,  // engagement *EngagementHandler
		testHandler,        // test *TestHandler
		memberHandler,      // member *MemberHandler
		toolHandler,        // tool *ToolHandler
		reportHandler,      // report *ReportHandler
		riskAcceptanceHandler, // riskAcceptance *RiskAcceptanceHandler — TASK-005 FIX
		internalHandler,    // internal *InternalHandler
		slaHandler,         // sla *SLAHandler
		productHandler,     // product *ProductHandler
		productSeed,        // productSeed *ProductSeedHandler
		findingSeed,        // findingSeed *FindingSeedHandler
		findingGroup,       // findingGroup *FindingGroupHandler
		logger,
	)

	mux.Handle("/", router)
	return nil
}

// raEventPublisher adapts *natsutil.Publisher to rauc.EventPublisher.
// rauc.EventPublisher.Publish takes map[string]any; natsutil.Publisher takes interface{}.
// This adapter satisfies the interface by type-converting the payload.
type raEventPublisher struct {
	pub interface {
		Publish(ctx context.Context, subject string, payload interface{}) error
	}
}

func (r *raEventPublisher) Publish(ctx context.Context, subject string, payload map[string]any) error {
	if r.pub == nil {
		return nil
	}
	return r.pub.Publish(ctx, subject, payload)
}
