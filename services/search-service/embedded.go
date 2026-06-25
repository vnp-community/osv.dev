// Package search provides the embeddable search-service for the OSV modular monolith.
// WireEmbedded mounts the full search-service HTTP router onto the provided ServeMux,
// enabling proper CVE full-text search, CWE/CAPEC taxonomy, and vendor browsing.
package search

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres driver for sqlx
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	deliveryhttp "github.com/osv/search-service/internal/delivery/http"
	"github.com/osv/search-service/internal/domain/repository"
	"github.com/osv/search-service/internal/infra/aigrpc"
	rediscache "github.com/osv/search-service/internal/infra/cache/redis"
	"github.com/osv/search-service/internal/infra/opensearch"
	"github.com/osv/search-service/internal/infra/pgvector"
	"github.com/osv/search-service/internal/infra/postgres"
	cvesearch "github.com/osv/search-service/internal/usecase/cvesearch"
	"github.com/osv/search-service/internal/usecase/getbyid"
)

// noopCVECache is a no-op implementation of CVECacheRepository used when Redis is not configured.
// It always returns a cache miss so the usecase falls through to Postgres.
type noopCVECache struct{}

func (noopCVECache) GetSearchResult(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("cache miss")
}
func (noopCVECache) SetSearchResult(_ context.Context, _ string, _ []byte, _ int) error { return nil }
func (noopCVECache) InvalidatePattern(_ context.Context, _ string) error                { return nil }

var _ repository.CVECacheRepository = noopCVECache{}

// WireEmbedded initializes the search-service routes on the provided ServeMux.
// It wires: Postgres CVE + Taxonomy + Vendor repositories → use cases → HTTP handler → chi router.
// Redis cache is used when REDIS_ADDR is set; otherwise a no-op cache is used.
func WireEmbedded(ctx context.Context, log zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	// 1. Postgres CVE repository (read-only full-text search)
	cveRepo := postgres.NewCVERepository(pool)

	// 2. sqlx.DB for taxonomy + vendor repos (they use sqlx, not pgx)
	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		postgresDSN = os.Getenv("DATA_DATABASE_URL")
	}

	var taxH *deliveryhttp.TaxonomyHandler
	var vendorH *deliveryhttp.VendorHandler
	var statsH *deliveryhttp.StatsHandler

	statsH = deliveryhttp.NewStatsHandler(cveRepo)

	if postgresDSN != "" {
		sqlxDB, err := sqlx.ConnectContext(ctx, "postgres", postgresDSN)
		if err == nil {
			cweRepo := postgres.NewCWERepository(sqlxDB)
			capecRepo := postgres.NewCAPECRepository(sqlxDB)
			taxH = deliveryhttp.NewTaxonomyHandler(cweRepo, capecRepo)

			vendorRepo := postgres.NewVendorRepository(sqlxDB)
			vendorH = deliveryhttp.NewVendorHandler(vendorRepo)
		} else {
			log.Warn().Err(err).Msg("search-service: sqlx connect failed, CWE/Vendor routes disabled")
		}
	}

	// 3. Cache: Redis if REDIS_ADDR is set, otherwise no-op
	var cacheRepo repository.CVECacheRepository = noopCVECache{}
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
		})
		cacheRepo = rediscache.NewCVECache(redisClient)
		log.Info().Str("redis", redisAddr).Msg("search-service: Redis cache enabled")
	}

	// 4. pgvector semantic search (optional)
	// MOCK-008 FIX: use real AI gRPC embedder instead of MockEmbedder
	var semanticUC *pgvector.UseCase
	if postgresDSN != "" {
		sqlxDB, err := sqlx.ConnectContext(ctx, "postgres", postgresDSN)
		if err == nil {
			searcher := pgvector.New(sqlxDB)

			// Attempt to connect to AI service for real embeddings
			aiAddr := os.Getenv("AI_SERVICE_GRPC")
			if aiAddr == "" {
				aiAddr = "localhost:50053"
			}
			aiEmbedder, embErr := aigrpc.New(aiAddr)
			if embErr != nil {
				log.Warn().Err(embErr).Str("ai_addr", aiAddr).
					Msg("search-service: AI embedder unavailable — semantic search disabled")
				// semanticUC remains nil → SemanticSearch handler returns graceful error
			} else {
				semanticUC = pgvector.NewUseCase(searcher, aiEmbedder)
				log.Info().Str("ai_addr", aiAddr).
					Msg("search-service: semantic search enabled via AI service gRPC")
			}
		} else {
			log.Warn().Err(err).Msg("search-service: pgvector connect failed, semantic search disabled")
		}
	}

	// FIX MOCK-009: Wire OpenSearch client
	var osClient *opensearch.Client
	osURL := os.Getenv("OPENSEARCH_URL")
	if osURL != "" {
		osClient = opensearch.NewClient(osURL, "cves", log)
		log.Info().Str("url", osURL).Msg("search-service: OpenSearch enabled")
	} else {
		log.Info().Msg("search-service: OPENSEARCH_URL not set, using Postgres FTS only")
	}

	// 5. Wire use cases
	searchUC := cvesearch.New(cveRepo, cacheRepo, osClient, log)
	getByIDUC := getbyid.New(cveRepo)

	// FIX MOCK-010: Wire InternalHandler
	var internalH *deliveryhttp.InternalHandler
	if osClient != nil {
		internalH = deliveryhttp.NewInternalHandler(osClient, cveRepo)
		log.Info().Msg("search-service: OpenSearch indexing routes enabled")
	}

	// 6. Wire HTTP handler and chi router (with all optional handlers)
	h := deliveryhttp.NewHandler(searchUC, getByIDUC, semanticUC, osClient, cveRepo, log)
	router := deliveryhttp.NewRouter(h, taxH, vendorH, internalH, statsH, log)

	// 7. Mount router on mux — handles all /api/v2/cves/*, /api/v2/epss/*, /api/v2/cwe/*, /api/v2/vendors, etc.
	mux.Handle("/api/v1/search/", router)
	mux.Handle("/api/v1/search/recent", router)
	mux.Handle("/api/v1/search/suggested", router)
	mux.Handle("/api/v2/cves/", router)
	mux.Handle("/api/v2/cves", router)
	mux.Handle("/api/v2/epss/", router)
	mux.Handle("/api/v2/epss", router)
	mux.Handle("/api/v2/cwe/", router)
	mux.Handle("/api/v2/cwe", router)
	mux.Handle("/api/v2/capec/", router)
	mux.Handle("/api/v2/capec", router)
	mux.Handle("/api/v2/vendors", router)
	mux.Handle("/api/v2/vendors/", router)
	mux.Handle("/api/v2/products", router)
	mux.Handle("/api/v2/stats/", router)
	mux.Handle("/internal/cves/", router)

	log.Info().Msg("search-service: embedded routes mounted (full-text + semantic search)")
	return nil
}
