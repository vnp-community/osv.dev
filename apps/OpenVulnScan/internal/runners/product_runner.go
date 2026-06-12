// Package runners — product_runner.go
// ProductRunner chạy product-service (DefectDojo product/engagement/test) trong goroutine riêng.
//
// product-service cung cấp:
//   - ProductService gRPC (GetOrCreate Product/Engagement/Test)
//   - HTTP REST cho product CRUD (ProductHandler)
//
// Bridge Pattern: product-service/internal/ không thể import từ module ngoài.
// Implement ProductBridge với direct Postgres để expose gRPC + HTTP.
//
// Module: github.com/defectdojo/product-service
package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/nats-io/nats.go"
	productv1 "github.com/osv/shared/proto/gen/go/product/v1"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// ProductRunnerConfig cấu hình cho product-service goroutine.
type ProductRunnerConfig struct {
	DBURL string
}

// ProductRunner implement ServiceRunner cho product-service.
type ProductRunner struct {
	cfg         ProductRunnerConfig
	nc          *nats.Conn
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger
	HTTPHandler http.Handler
}

// NewProductRunner tạo ProductRunner.
func NewProductRunner(cfg ProductRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *ProductRunner {
	return &ProductRunner{
		cfg: cfg,
		nc:  nc,
		lis: lis,
		log: log.With().Str("runner", "product-service").Logger(),
	}
}

func (r *ProductRunner) Name() string { return "product-service" }

// Run khởi động product goroutine.
func (r *ProductRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern)...")

	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("product: db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("product: db ping: %w", err)
	}
	r.log.Info().Msg("postgres connected")

	bridge := newProductBridge(db, r.log)
	r.HTTPHandler = bridge.router()

	// NATS subscriber: scan.completed → upsert asset record
	var natsSub *nats.Subscription
	if r.nc != nil {
		var err2 error
		natsSub, err2 = r.nc.Subscribe("scan.completed", func(msg *nats.Msg) {
			bridge.upsertAssetFromScan(ctx, db, msg.Data)
		})
		if err2 != nil {
			r.log.Warn().Err(err2).Msg("NATS subscribe scan.completed failed — asset upsert disabled")
		} else {
			r.log.Info().Msg("NATS subscriber: scan.completed → asset upsert")
		}
	}

	// gRPC server — expose ProductServiceServer
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	productv1.RegisterProductServiceServer(r.server, bridge)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC ProductService ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	r.log.Info().Msg("product-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		if natsSub != nil {
			natsSub.Unsubscribe() //nolint:errcheck
		}
		return nil
	case err := <-errCh:
		return wrapRunnerError("product-service", err)
	}
}

// Health kiểm tra gRPC health.
func (r *ProductRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("product health: %w", err)
	}
	defer conn.Close()

	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("product not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener for gRPC client connections.
func (r *ProductRunner) Listener() *bufconn.Listener { return r.lis }

// ── Product Bridge ────────────────────────────────────────────────────────────

// productBridge implement productv1.ProductServiceServer + HTTP handler với direct Postgres.
type productBridge struct {
	productv1.UnimplementedProductServiceServer
	db  *pgxpool.Pool
	log zerolog.Logger
}

func newProductBridge(db *pgxpool.Pool, l zerolog.Logger) *productBridge {
	return &productBridge{db: db, log: l}
}

// upsertAssetFromScan creates or updates an asset record when a scan completes.
// Payload: {"scan_id": "...", "finding_count": N, "status": "completed"}
func (b *productBridge) upsertAssetFromScan(ctx context.Context, db *pgxpool.Pool, data []byte) {
	var evt struct {
		ScanID string `json:"scan_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &evt); err != nil || evt.ScanID == "" {
		return
	}

	// Get scan targets
	var targetsJSON []byte
	if err := db.QueryRow(ctx, `SELECT targets FROM scans WHERE id = $1::uuid`, evt.ScanID).
		Scan(&targetsJSON); err != nil {
		return
	}

	var targets []string
	json.Unmarshal(targetsJSON, &targets) //nolint:errcheck

	// Upsert asset records for each target
	for _, target := range targets {
		assetID := uuid.New()
		db.Exec(ctx, `
			INSERT INTO assets (id, hostname, ip_address, asset_type, last_seen, created_at, updated_at)
			VALUES ($1, $2, $2, 'host', NOW(), NOW(), NOW())
			ON CONFLICT (hostname) DO UPDATE SET
				last_seen = NOW(), updated_at = NOW()
		`, assetID, target) //nolint:errcheck
	}

	if len(targets) > 0 {
		b.log.Debug().Str("scan_id", evt.ScanID).Int("assets", len(targets)).Msg("assets upserted")
	}
}

// ── gRPC Methods ──────────────────────────────────────────────────────────────

// GetOrCreateProduct tìm hoặc tạo Product theo name+productType.
func (b *productBridge) GetOrCreateProduct(ctx context.Context, req *productv1.GetOrCreateProductRequest) (*productv1.GetOrCreateProductResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// 1. Get or create ProductType
	var ptID uuid.UUID
	err := b.db.QueryRow(ctx,
		`SELECT id FROM product_types WHERE name = $1`, req.ProductTypeName).Scan(&ptID)
	if err != nil {
		// Create it
		ptID = uuid.New()
		_, err = b.db.Exec(ctx, `
			INSERT INTO product_types (id, name, description, created_at, updated_at)
			VALUES ($1, $2, '', NOW(), NOW())
			ON CONFLICT (name) DO UPDATE SET updated_at = NOW()
			RETURNING id
		`, ptID, req.ProductTypeName)
		if err != nil {
			// Try to get after conflict
			b.db.QueryRow(ctx, `SELECT id FROM product_types WHERE name = $1`, req.ProductTypeName).Scan(&ptID) //nolint:errcheck
		}
	}

	// 2. Get or create Product
	var productID uuid.UUID
	var created bool
	err = b.db.QueryRow(ctx,
		`SELECT id FROM products WHERE name = $1 AND prod_type_id = $2`, req.Name, ptID).Scan(&productID)
	if err != nil {
		productID = uuid.New()
		_, err = b.db.Exec(ctx, `
			INSERT INTO products (id, prod_type_id, name, description, created_at, updated_at)
			VALUES ($1, $2, $3, '', NOW(), NOW())
		`, productID, ptID, req.Name)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create product: %v", err)
		}
		created = true
	}

	return &productv1.GetOrCreateProductResponse{
		ProductId:     productID.String(),
		ProductTypeId: ptID.String(),
		Created:       created,
	}, nil
}

// GetOrCreateEngagement tìm hoặc tạo Engagement cho Product.
func (b *productBridge) GetOrCreateEngagement(ctx context.Context, req *productv1.GetOrCreateEngagementRequest) (*productv1.GetOrCreateEngagementResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	var engID uuid.UUID
	var created bool
	name := req.Name
	if name == "" {
		name = "Default Engagement"
	}

	err = b.db.QueryRow(ctx,
		`SELECT id FROM engagements WHERE product_id = $1 AND name = $2 AND status = 'In Progress'`,
		productID, name).Scan(&engID)
	if err != nil {
		engID = uuid.New()
		_, err = b.db.Exec(ctx, `
			INSERT INTO engagements (id, product_id, name, description, status, engagement_type, start_date, created_at, updated_at)
			VALUES ($1, $2, $3, '', 'In Progress', $4, CURRENT_DATE, NOW(), NOW())
		`, engID, productID, name, req.EngagementType)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create engagement: %v", err)
		}
		created = true
	}

	return &productv1.GetOrCreateEngagementResponse{
		EngagementId: engID.String(),
		Created:      created,
	}, nil
}

// GetOrCreateTest tìm hoặc tạo Test trong Engagement.
func (b *productBridge) GetOrCreateTest(ctx context.Context, req *productv1.GetOrCreateTestRequest) (*productv1.GetOrCreateTestResponse, error) {
	engID, err := uuid.Parse(req.EngagementId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engagement_id")
	}

	var testID uuid.UUID
	var created bool

	err = b.db.QueryRow(ctx,
		`SELECT id FROM tests WHERE engagement_id = $1 AND scan_type = $2`,
		engID, req.ScanType).Scan(&testID)
	if err != nil {
		testID = uuid.New()
		title := req.Title
		if title == "" {
			title = req.ScanType
		}
		_, err = b.db.Exec(ctx, `
			INSERT INTO tests (id, engagement_id, title, scan_type, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`, testID, engID, title, req.ScanType)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create test: %v", err)
		}
		created = true
	}

	return &productv1.GetOrCreateTestResponse{
		TestId:  testID.String(),
		Created: created,
	}, nil
}

// UpdateEngagementTimestamps cập nhật timestamps cho engagement (close, etc.).
func (b *productBridge) UpdateEngagementTimestamps(ctx context.Context, req *productv1.UpdateEngagementTimestampsRequest) (*productv1.UpdateEngagementTimestampsResponse, error) {
	if req.EngagementId == "" {
		return nil, status.Error(codes.InvalidArgument, "engagement_id is required")
	}
	_, err := b.db.Exec(ctx,
		`UPDATE engagements SET status = 'Completed', end_date = CURRENT_DATE, updated_at = NOW() WHERE id = $1::uuid`,
		req.EngagementId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update engagement: %v", err)
	}
	return &productv1.UpdateEngagementTimestampsResponse{}, nil
}

// ── HTTP Routes ───────────────────────────────────────────────────────────────

func (b *productBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		// Products
		r.Get("/products", b.listProducts)
		r.Post("/products", b.createProduct)
		r.Get("/products/{id}", b.getProduct)
		r.Put("/products/{id}", b.updateProduct)
		r.Delete("/products/{id}", b.deleteProduct)

		// Engagements
		r.Post("/engagements", b.createEngagement)
		r.Get("/engagements/{id}", b.getEngagement)
		r.Get("/products/{id}/engagements", b.listProductEngagements)

		// Tests (within engagement)
		r.Get("/engagements/{id}/tests", b.listTests)
	})
	return r
}

func (b *productBridge) listProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := b.db.Query(r.Context(), `
		SELECT p.id::text, p.name, p.description, pt.name as product_type, p.created_at
		FROM products p
		LEFT JOIN product_types pt ON p.prod_type_id = pt.id
		ORDER BY p.created_at DESC LIMIT 50
	`)
	if err != nil {
		writeJSONProduct(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var products []map[string]interface{}
	for rows.Next() {
		var id, name, desc, ptName string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &desc, &ptName, &createdAt); err != nil {
			continue
		}
		products = append(products, map[string]interface{}{
			"id": id, "name": name, "description": desc,
			"product_type": ptName, "created_at": createdAt,
		})
	}
	if products == nil {
		products = []map[string]interface{}{}
	}
	writeJSONProduct(w, http.StatusOK, map[string]interface{}{"products": products, "total": len(products)})
}

func (b *productBridge) createProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		ProductTypeID string `json:"product_type_id"`
		ProductType   string `json:"product_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeJSONProduct(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Resolve ProductType
	var ptID uuid.UUID
	if req.ProductTypeID != "" {
		ptID, _ = uuid.Parse(req.ProductTypeID)
	} else {
		ptName := req.ProductType
		if ptName == "" {
			ptName = "General"
		}
		ptID = uuid.New()
		b.db.Exec(r.Context(), `
			INSERT INTO product_types (id, name, description, created_at, updated_at)
			VALUES ($1, $2, '', NOW(), NOW()) ON CONFLICT (name) DO NOTHING
		`, ptID, ptName) //nolint:errcheck
		b.db.QueryRow(r.Context(), `SELECT id FROM product_types WHERE name = $1`, ptName).Scan(&ptID) //nolint:errcheck
	}

	id := uuid.New()
	_, err := b.db.Exec(r.Context(), `
		INSERT INTO products (id, prod_type_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, id, ptID, req.Name, req.Description)
	if err != nil {
		writeJSONProduct(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONProduct(w, http.StatusCreated, map[string]interface{}{
		"id": id.String(), "name": req.Name, "description": req.Description,
	})
}

func (b *productBridge) getProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var name, desc string
	var createdAt time.Time
	err := b.db.QueryRow(r.Context(),
		`SELECT name, description, created_at FROM products WHERE id = $1::uuid`, id).
		Scan(&name, &desc, &createdAt)
	if err != nil {
		writeJSONProduct(w, http.StatusNotFound, map[string]string{"error": "product not found"})
		return
	}
	writeJSONProduct(w, http.StatusOK, map[string]interface{}{
		"id": id, "name": name, "description": desc, "created_at": createdAt,
	})
}

func (b *productBridge) updateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
	b.db.Exec(r.Context(), `UPDATE products SET name=$1, description=$2, updated_at=NOW() WHERE id=$3::uuid`,
		req.Name, req.Description, id) //nolint:errcheck
	writeJSONProduct(w, http.StatusOK, map[string]string{"message": "updated"})
}

func (b *productBridge) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	b.db.Exec(r.Context(), `DELETE FROM products WHERE id = $1::uuid`, id) //nolint:errcheck
	writeJSONProduct(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (b *productBridge) createEngagement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID      string `json:"product_id"`
		Name           string `json:"name"`
		Description    string `json:"description"`
		EngagementType string `json:"engagement_type"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		writeJSONProduct(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}
	if req.Name == "" {
		req.Name = "Default Engagement"
	}
	if req.EngagementType == "" {
		req.EngagementType = "Interactive"
	}
	id := uuid.New()
	_, err = b.db.Exec(r.Context(), `
		INSERT INTO engagements (id, product_id, name, description, status, engagement_type, start_date, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'In Progress',$5,CURRENT_DATE,NOW(),NOW())
	`, id, productID, req.Name, req.Description, req.EngagementType)
	if err != nil {
		writeJSONProduct(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONProduct(w, http.StatusCreated, map[string]interface{}{
		"id": id.String(), "name": req.Name, "product_id": req.ProductID,
	})
}

func (b *productBridge) getEngagement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var name, desc, status string
	var createdAt time.Time
	err := b.db.QueryRow(r.Context(),
		`SELECT name, description, status, created_at FROM engagements WHERE id = $1::uuid`, id).
		Scan(&name, &desc, &status, &createdAt)
	if err != nil {
		writeJSONProduct(w, http.StatusNotFound, map[string]string{"error": "engagement not found"})
		return
	}
	writeJSONProduct(w, http.StatusOK, map[string]interface{}{
		"id": id, "name": name, "description": desc, "status": status, "created_at": createdAt,
	})
}

func (b *productBridge) listProductEngagements(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, name, status, created_at FROM engagements
		WHERE product_id = $1::uuid ORDER BY created_at DESC LIMIT 20
	`, productID)
	if err != nil {
		writeJSONProduct(w, http.StatusOK, map[string]interface{}{"engagements": []interface{}{}})
		return
	}
	defer rows.Close()
	var engs []map[string]interface{}
	for rows.Next() {
		var id, name, st string
		var ca time.Time
		if err := rows.Scan(&id, &name, &st, &ca); err != nil {
			continue
		}
		engs = append(engs, map[string]interface{}{"id": id, "name": name, "status": st, "created_at": ca})
	}
	if engs == nil {
		engs = []map[string]interface{}{}
	}
	writeJSONProduct(w, http.StatusOK, map[string]interface{}{"engagements": engs, "total": len(engs)})
}

func (b *productBridge) listTests(w http.ResponseWriter, r *http.Request) {
	engID := chi.URLParam(r, "id")
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, title, scan_type, created_at FROM tests
		WHERE engagement_id = $1::uuid ORDER BY created_at DESC LIMIT 50
	`, engID)
	if err != nil {
		writeJSONProduct(w, http.StatusOK, map[string]interface{}{"tests": []interface{}{}})
		return
	}
	defer rows.Close()
	var tests []map[string]interface{}
	for rows.Next() {
		var id, title, scanType string
		var ca time.Time
		if err := rows.Scan(&id, &title, &scanType, &ca); err != nil {
			continue
		}
		tests = append(tests, map[string]interface{}{"id": id, "title": title, "scan_type": scanType, "created_at": ca})
	}
	if tests == nil {
		tests = []map[string]interface{}{}
	}
	writeJSONProduct(w, http.StatusOK, map[string]interface{}{"tests": tests, "total": len(tests)})
}

func writeJSONProduct(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
