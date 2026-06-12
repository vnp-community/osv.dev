// Package runners — vuln_runner.go
// VulnRunner chạy vulnerability-service (CVE search) trong một goroutine riêng.
//
// Bridge Pattern: vulnerability-service/internal/ không thể import từ module ngoài.
// Implement minimal CVE HTTP handler với direct MongoDB queries.
// vulnerability-service dùng MongoDB cho CVE data.
//
// Module: github.com/osv/vulnerability-service
package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

// VulnRunnerConfig cấu hình cho vulnerability goroutine.
type VulnRunnerConfig struct {
	MongoURI  string // e.g. "mongodb://localhost:27017"
	MongoDB   string // database name, e.g. "cvedb"
}

// VulnRunner implement ServiceRunner cho vulnerability-service.
type VulnRunner struct {
	cfg         VulnRunnerConfig
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger

	// HTTPHandler expose cho API Gateway mount
	HTTPHandler http.Handler
}

// NewVulnRunner tạo VulnRunner.
func NewVulnRunner(cfg VulnRunnerConfig, lis *bufconn.Listener) *VulnRunner {
	return &VulnRunner{
		cfg: cfg,
		lis: lis,
		log: log.With().Str("runner", "vulnerability-service").Logger(),
	}
}

func (r *VulnRunner) Name() string { return "vulnerability-service" }

// Run khởi động vulnerability goroutine.
func (r *VulnRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern)...")

	// MongoDB connection
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(r.cfg.MongoURI))
	if err != nil {
		// Non-fatal: warn và tiếp tục với empty responses
		r.log.Warn().Err(err).Msg("mongodb connect failed — CVE search unavailable")
		mongoClient = nil
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if pingErr := mongoClient.Ping(pingCtx, nil); pingErr != nil {
			r.log.Warn().Err(pingErr).Msg("mongodb ping failed — CVE search will return empty results")
		} else {
			r.log.Info().Str("uri", r.cfg.MongoURI).Msg("mongodb connected")
		}
		cancel()
	}

	// CVE Bridge HTTP handler
	var cveDB *mongo.Database
	if mongoClient != nil {
		cveDB = mongoClient.Database(r.cfg.MongoDB)
	}
	bridge := newVulnBridge(cveDB, r.log)
	r.HTTPHandler = bridge.router()

	// gRPC health server trên bufconn
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

	r.log.Info().Msg("vulnerability-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		if mongoClient != nil {
			mongoClient.Disconnect(context.Background()) //nolint:errcheck
		}
		return nil
	case err := <-errCh:
		return wrapRunnerError("vulnerability-service", err)
	}
}

// Health kiểm tra gRPC health.
func (r *VulnRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("vuln health: %w", err)
	}
	defer conn.Close()

	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("vuln not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *VulnRunner) Listener() *bufconn.Listener { return r.lis }

// ── Vuln Bridge ───────────────────────────────────────────────────────────────

// vulnBridge implement CVE HTTP handler với direct MongoDB queries.
type vulnBridge struct {
	db  *mongo.Database // nil if MongoDB unavailable
	log zerolog.Logger
}

func newVulnBridge(db *mongo.Database, l zerolog.Logger) *vulnBridge {
	return &vulnBridge{db: db, log: l}
}

func (b *vulnBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// CVE routes (tương thích với vulnerability-service)
	r.Get("/cve/last/{n}", b.getLast)
	r.Get("/cve/recent/{timeframe}", b.getRecent)
	r.Get("/cve/search", b.searchByCPE)
	r.Get("/cve/{id}", b.getByID)

	return r
}

// getByID handles GET /cve/{id}
func (b *vulnBridge) getByID(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "id")

	if b.db == nil {
		writeJSONVuln(w, http.StatusServiceUnavailable, map[string]string{"error": "cve database unavailable"})
		return
	}

	col := b.db.Collection("cves")
	var result bson.M
	err := col.FindOne(r.Context(), bson.M{"_id": cveID}).Decode(&result)
	if err != nil {
		writeJSONVuln(w, http.StatusNotFound, map[string]string{"error": "CVE not found", "id": cveID})
		return
	}
	writeJSONVuln(w, http.StatusOK, result)
}

// getLast handles GET /cve/last/{n}
func (b *vulnBridge) getLast(w http.ResponseWriter, r *http.Request) {
	if b.db == nil {
		writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": []interface{}{}})
		return
	}
	col := b.db.Collection("cves")
	limit := int64(10)
	opts := options.Find().SetSort(bson.M{"published": -1}).SetLimit(limit)
	cursor, err := col.Find(r.Context(), bson.M{}, opts)
	if err != nil {
		writeJSONVuln(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer cursor.Close(r.Context())

	var cves []bson.M
	cursor.All(r.Context(), &cves) //nolint:errcheck
	if cves == nil {
		cves = []bson.M{}
	}
	writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": cves, "total": len(cves)})
}

// getRecent handles GET /cve/recent/{timeframe}
func (b *vulnBridge) getRecent(w http.ResponseWriter, r *http.Request) {
	if b.db == nil {
		writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": []interface{}{}})
		return
	}
	// 7 days default
	since := time.Now().AddDate(0, 0, -7)
	col := b.db.Collection("cves")
	opts := options.Find().SetSort(bson.M{"published": -1}).SetLimit(50)
	cursor, err := col.Find(r.Context(), bson.M{"published": bson.M{"$gte": since}}, opts)
	if err != nil {
		writeJSONVuln(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer cursor.Close(r.Context())

	var cves []bson.M
	cursor.All(r.Context(), &cves) //nolint:errcheck
	if cves == nil {
		cves = []bson.M{}
	}
	writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": cves, "total": len(cves)})
}

// searchByCPE handles GET /cve/search?cpe=...
func (b *vulnBridge) searchByCPE(w http.ResponseWriter, r *http.Request) {
	cpe := r.URL.Query().Get("cpe")
	if cpe == "" {
		writeJSONVuln(w, http.StatusBadRequest, map[string]string{"error": "cpe parameter required"})
		return
	}
	if b.db == nil {
		writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": []interface{}{}, "query": cpe})
		return
	}

	col := b.db.Collection("cves")
	opts := options.Find().SetLimit(50)
	cursor, err := col.Find(r.Context(), bson.M{
		"$or": []bson.M{
			{"configurations": bson.M{"$elemMatch": bson.M{"cpe23Uri": bson.M{"$regex": cpe}}}},
			{"affected_packages": bson.M{"$elemMatch": bson.M{"package": bson.M{"$regex": cpe}}}},
		},
	}, opts)
	if err != nil {
		writeJSONVuln(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer cursor.Close(r.Context())

	var cves []bson.M
	cursor.All(r.Context(), &cves) //nolint:errcheck
	if cves == nil {
		cves = []bson.M{}
	}
	writeJSONVuln(w, http.StatusOK, map[string]interface{}{"cves": cves, "total": len(cves), "query": cpe})
}

func writeJSONVuln(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
