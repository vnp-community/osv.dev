// Package runners — query_runner.go
// QueryRunner chạy query-service goroutine.
//
// Bridge Pattern THUẦN: không import query-service/internal/* (Go policy).
// Implement browse handler trực tiếp trong monolith với:
//   - Redis L1 cache (BrowseCache: vendors/products)
//   - MongoDB L2 fallback (CPERepo: distinct vendor/product queries)
//
// Endpoints được serve:
//   GET /browse/              → list vendors (CPE type "a" default)
//   GET /browse/{vendor}      → list products for vendor
//   GET /vendors/search       → search vendors by query
//   GET /search/{vendor}/{product}          → list versions
//   GET /search/{vendor}/{product}/{version}→ list versions (filtered)
package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// QueryRunnerConfig cấu hình cho query-service goroutine.
type QueryRunnerConfig struct {
	MongoURI string
	MongoDB  string // "cvedb"
	RedisURL string
}

// QueryRunner implement ServiceRunner cho query-service (browse CPE data).
type QueryRunner struct {
	cfg         QueryRunnerConfig
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger
	HTTPHandler http.Handler
}

// NewQueryRunner tạo QueryRunner.
func NewQueryRunner(cfg QueryRunnerConfig, lis *bufconn.Listener) *QueryRunner {
	if cfg.MongoDB == "" {
		cfg.MongoDB = "cvedb"
	}
	return &QueryRunner{
		cfg: cfg,
		lis: lis,
		log: log.With().Str("runner", "query-service").Logger(),
	}
}

func (r *QueryRunner) Name() string { return "query-service" }

// Run khởi động query goroutine.
func (r *QueryRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern — browse handler)...")

	// 1. Kết nối MongoDB (L2 — cpe collection)
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(r.cfg.MongoURI))
	if err != nil {
		r.log.Warn().Err(err).Msg("MongoDB connect failed — browse degraded")
		mongoClient = nil
	} else if pingErr := mongoClient.Ping(ctx, nil); pingErr != nil {
		r.log.Warn().Err(pingErr).Msg("MongoDB ping failed — browse degraded")
	}

	var mongoDB *mongo.Database
	if mongoClient != nil {
		mongoDB = mongoClient.Database(r.cfg.MongoDB)
	}

	// 2. Kết nối Redis (L1)
	var redisClient *redis.Client
	if redisOpts, err := redis.ParseURL(r.cfg.RedisURL); err == nil {
		redisClient = redis.NewClient(redisOpts)
		if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
			r.log.Warn().Err(pingErr).Msg("Redis ping failed — L1 cache disabled")
			redisClient = nil
		}
	} else {
		r.log.Warn().Err(err).Msg("Redis URL parse failed")
	}

	// 3. Build browse HTTP handler
	bridge := &queryBridge{
		mongoDB: mongoDB,
		redis:   redisClient,
		log:     r.log,
	}
	r.HTTPHandler = bridge.router()

	// 4. gRPC health server trên bufconn
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC health ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	r.log.Info().
		Str("mongo", r.cfg.MongoDB).
		Msg("query-service ready: /browse/, /vendors/search, /search/{vendor}/{product}")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		if mongoClient != nil {
			mongoClient.Disconnect(context.Background()) //nolint:errcheck
		}
		if redisClient != nil {
			redisClient.Close() //nolint:errcheck
		}
		return nil
	case err := <-errCh:
		return wrapRunnerError("query-service", err)
	}
}

// Health kiểm tra health via bufconn gRPC.
func (r *QueryRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("query health: %w", err)
	}
	defer conn.Close()
	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("query not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *QueryRunner) Listener() *bufconn.Listener { return r.lis }

// ── Query Bridge (Browse CPE Handler) ─────────────────────────────────────────

type queryBridge struct {
	mongoDB *mongo.Database
	redis   *redis.Client
	log     zerolog.Logger
}

const (
	queryRedisTTL = 24 * time.Hour
)

func (b *queryBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	// Browse CPE vendor/product tree
	r.Get("/browse/", b.handleListVendors)
	r.Get("/browse/{vendor}", b.handleListProducts)
	r.Get("/vendors/search", b.handleSearchVendors)

	// Version browsing
	r.Get("/search/{vendor}/{product}", b.handleListVersions)
	r.Get("/search/{vendor}/{product}/{version}", b.handleListVersions)

	return r
}

// handleListVendors returns distinct CPE vendors.
// Query: ?type=a|o|h (default "a" = application)
func (b *queryBridge) handleListVendors(w http.ResponseWriter, r *http.Request) {
	cpeType := r.URL.Query().Get("type")
	if cpeType == "" {
		cpeType = "a"
	}

	// L1: Redis
	if vendors := b.getVendorsFromRedis(r.Context(), cpeType); vendors != nil {
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendors": vendors, "source": "redis"})
		return
	}

	// L2: MongoDB
	vendors, err := b.listVendorsMongo(r.Context(), cpeType)
	if err != nil {
		b.log.Warn().Err(err).Str("cpe_type", cpeType).Msg("listVendors failed")
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendors": []string{}, "source": "error"})
		return
	}

	// Cache result
	if len(vendors) > 0 {
		b.setVendorsInRedis(r.Context(), cpeType, vendors)
	}

	writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendors": vendors, "total": len(vendors), "source": "mongo"})
}

// handleListProducts returns distinct products for a vendor.
func (b *queryBridge) handleListProducts(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	if vendor == "" {
		writeJSONQuery(w, http.StatusBadRequest, map[string]string{"error": "vendor required"})
		return
	}

	// L1: Redis
	if products := b.getProductsFromRedis(r.Context(), vendor); products != nil {
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendor": vendor, "products": products, "source": "redis"})
		return
	}

	// L2: MongoDB
	products, err := b.listProductsMongo(r.Context(), vendor)
	if err != nil {
		b.log.Warn().Err(err).Str("vendor", vendor).Msg("listProducts failed")
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendor": vendor, "products": []string{}, "source": "error"})
		return
	}

	if len(products) > 0 {
		b.setProductsInRedis(r.Context(), vendor, products)
	}

	writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendor": vendor, "products": products, "total": len(products)})
}

// handleSearchVendors searches vendors by pattern.
func (b *queryBridge) handleSearchVendors(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSONQuery(w, http.StatusBadRequest, map[string]string{"error": "q parameter required"})
		return
	}

	vendors, err := b.searchVendorsMongo(r.Context(), query)
	if err != nil {
		b.log.Warn().Err(err).Str("query", query).Msg("searchVendors failed")
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendors": []string{}, "query": query})
		return
	}
	writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendors": vendors, "query": query, "total": len(vendors)})
}

// handleListVersions returns distinct versions for vendor/product.
func (b *queryBridge) handleListVersions(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	product := chi.URLParam(r, "product")
	version := chi.URLParam(r, "version") // optional filter

	if b.mongoDB == nil {
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"versions": []string{}, "message": "MongoDB not available"})
		return
	}

	col := b.mongoDB.Collection("cpe")
	filter := bson.M{
		"vendor":  vendor,
		"product": product,
	}
	if version != "" {
		filter["version"] = bson.M{"$regex": version, "$options": "i"}
	}

	vals, err := col.Distinct(r.Context(), "version", filter)
	if err != nil {
		writeJSONQuery(w, http.StatusOK, map[string]interface{}{"vendor": vendor, "product": product, "versions": []string{}})
		return
	}

	versions := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" && s != "*" && s != "-" {
			versions = append(versions, s)
		}
	}
	sort.Strings(versions)
	writeJSONQuery(w, http.StatusOK, map[string]interface{}{
		"vendor": vendor, "product": product,
		"versions": versions, "total": len(versions),
	})
}

// ── MongoDB helpers ────────────────────────────────────────────────────────────

func (b *queryBridge) listVendorsMongo(ctx context.Context, cpeType string) ([]string, error) {
	if b.mongoDB == nil {
		return []string{}, nil
	}
	col := b.mongoDB.Collection("cpe")
	filter := bson.M{
		"cpeName": bson.M{
			"$regex":   fmt.Sprintf(`^cpe:2\.3:%s:`, cpeType),
			"$options": "i",
		},
	}
	vals, err := col.Distinct(ctx, "vendor", filter)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" && s != "*" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (b *queryBridge) listProductsMongo(ctx context.Context, vendor string) ([]string, error) {
	if b.mongoDB == nil {
		return []string{}, nil
	}
	col := b.mongoDB.Collection("cpe")
	filter := bson.M{"vendor": vendor}
	vals, err := col.Distinct(ctx, "product", filter)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" && s != "*" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (b *queryBridge) searchVendorsMongo(ctx context.Context, query string) ([]string, error) {
	if b.mongoDB == nil {
		return []string{}, nil
	}
	col := b.mongoDB.Collection("cpe")
	filter := bson.M{
		"vendor": bson.M{
			"$regex":   strings.ToLower(query),
			"$options": "i",
		},
	}
	vals, err := col.Distinct(ctx, "vendor", filter, &options.DistinctOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" && s != "*" {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result, nil
}

// ── Redis L1 helpers ──────────────────────────────────────────────────────────

func (b *queryBridge) getVendorsFromRedis(ctx context.Context, cpeType string) []string {
	if b.redis == nil {
		return nil
	}
	members, err := b.redis.SMembers(ctx, "ovs:vendors:"+cpeType).Result()
	if err != nil || len(members) == 0 {
		return nil
	}
	sort.Strings(members)
	return members
}

func (b *queryBridge) setVendorsInRedis(ctx context.Context, cpeType string, vendors []string) {
	if b.redis == nil || len(vendors) == 0 {
		return
	}
	args := make([]interface{}, len(vendors))
	for i, v := range vendors {
		args[i] = v
	}
	b.redis.SAdd(ctx, "ovs:vendors:"+cpeType, args...) //nolint:errcheck
	b.redis.Expire(ctx, "ovs:vendors:"+cpeType, queryRedisTTL) //nolint:errcheck
}

func (b *queryBridge) getProductsFromRedis(ctx context.Context, vendor string) []string {
	if b.redis == nil {
		return nil
	}
	members, err := b.redis.SMembers(ctx, "ovs:products:"+vendor).Result()
	if err != nil || len(members) == 0 {
		return nil
	}
	sort.Strings(members)
	return members
}

func (b *queryBridge) setProductsInRedis(ctx context.Context, vendor string, products []string) {
	if b.redis == nil || len(products) == 0 {
		return
	}
	args := make([]interface{}, len(products))
	for i, v := range products {
		args[i] = v
	}
	b.redis.SAdd(ctx, "ovs:products:"+vendor, args...) //nolint:errcheck
	b.redis.Expire(ctx, "ovs:products:"+vendor, queryRedisTTL) //nolint:errcheck
}

func writeJSONQuery(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
