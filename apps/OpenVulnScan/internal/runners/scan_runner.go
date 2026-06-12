// Package runners — scan_runner.go
// ScanRunner chạy scan-service trong một goroutine riêng.
//
// Bridge Pattern: scan-service/internal/ không thể import từ module ngoài.
// Sử dụng direct Postgres queries và NATS publishing để implement scan logic.
//
// Architecture:
// - POST /api/v1/scans → tạo scan record trong DB, enqueue vào WorkerChannel
// - WorkerPool goroutines → execute nmap scans, publish scan.completed events
// - gRPC health server trên bufconn để Registry health check
//
// NOTE: scan-service không có public gRPC handler → implement HTTP handler trực tiếp.
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
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// ScanRunnerConfig cấu hình cho scan goroutine.
type ScanRunnerConfig struct {
	DBURL          string
	NmapBinary     string
	ZAPApiURL      string
	ZAPApiKey      string
	DefaultTimeout int // seconds
	WorkerPoolSize int
}

// ScanJob đại diện cho một scan task.
type ScanJob struct {
	ScanID uuid.UUID
	UserID string
}

// ScanRunner implement ServiceRunner cho scan-service.
type ScanRunner struct {
	cfg     ScanRunnerConfig
	nc      *nats.Conn
	lis     *bufconn.Listener
	server  *grpc.Server
	log     zerolog.Logger
	jobCh   chan ScanJob

	// HTTPHandler expose cho API Gateway mount
	HTTPHandler http.Handler
}

// NewScanRunner tạo ScanRunner.
func NewScanRunner(cfg ScanRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *ScanRunner {
	workers := cfg.WorkerPoolSize
	if workers <= 0 {
		workers = 3
	}
	return &ScanRunner{
		cfg:   cfg,
		nc:    nc,
		lis:   lis,
		log:   log.With().Str("runner", "scan-service").Logger(),
		jobCh: make(chan ScanJob, workers*4),
	}
}

func (r *ScanRunner) Name() string { return "scan-service" }

// Run khởi động scan goroutine.
func (r *ScanRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern)...")

	// PostgreSQL
	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("scan: db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("scan: db ping: %w", err)
	}
	r.log.Info().Msg("postgres connected")

	// ScanBridge: HTTP handler + scan logic
	bridge := newScanBridge(db, r.nc, r.jobCh, r.cfg, r.log)
	r.HTTPHandler = bridge.router()

	// Start worker goroutines (WorkerPool)
	for i := 0; i < r.cfg.WorkerPoolSize; i++ {
		go bridge.worker(ctx, i)
	}

	// Start CronWorker — polls scheduled_scans every minute
	go r.cronWorker(ctx, bridge)

	// gRPC health server trên bufconn
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC health server ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	r.log.Info().
		Int("workers", r.cfg.WorkerPoolSize).
		Str("nmap", r.cfg.NmapBinary).
		Msg("scan-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		return nil
	case err := <-errCh:
		return wrapRunnerError("scan-service", err)
	}
}

// Health kiểm tra gRPC health.
func (r *ScanRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("scan health: %w", err)
	}
	defer conn.Close()

	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("scan not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *ScanRunner) Listener() *bufconn.Listener { return r.lis }

// cronWorker polls scheduled_scans table every minute and enqueues due scans.
func (r *ScanRunner) cronWorker(ctx context.Context, bridge *scanBridge) {
	r.log.Info().Msg("CronWorker started — polling scheduled_scans every 60s")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("CronWorker stopped")
			return
		case <-ticker.C:
			r.runScheduledScans(ctx, bridge)
		}
	}
}

// runScheduledScans finds scheduled scans that are due and enqueues them.
func (r *ScanRunner) runScheduledScans(ctx context.Context, bridge *scanBridge) {
	rows, err := bridge.db.Query(ctx, `
		SELECT id::text, targets, scan_type
		FROM scheduled_scans
		WHERE enabled = true
		  AND next_run_at <= NOW()
		  AND (last_run_at IS NULL OR last_run_at < NOW() - interval '50 seconds')
		LIMIT 10
	`)
	if err != nil {
		r.log.Debug().Err(err).Msg("cronWorker: scheduled_scans query failed (table may not exist yet)")
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var schedID, scanType string
		var targetsJSON []byte
		if err := rows.Scan(&schedID, &targetsJSON, &scanType); err != nil {
			continue
		}

		// Create a scan record and enqueue
		scanID := uuid.New()
		_, err := bridge.db.Exec(ctx, `
			INSERT INTO scans (id, targets, scan_type, status, priority, created_at, updated_at)
			VALUES ($1, $2, $3, 'pending', 3, NOW(), NOW())
		`, scanID, targetsJSON, scanType)
		if err != nil {
			r.log.Warn().Err(err).Str("sched_id", schedID).Msg("failed to create scheduled scan")
			continue
		}

		// Update next_run_at
		bridge.db.Exec(ctx, `
			UPDATE scheduled_scans SET last_run_at = NOW(), next_run_at = NOW() + interval '1 hour'
			WHERE id = $1::uuid
		`, schedID) //nolint:errcheck

		select {
		case bridge.jobCh <- ScanJob{ScanID: scanID}:
			count++
		default:
			r.log.Warn().Str("sched_id", schedID).Msg("job queue full — skipping scheduled scan")
		}
	}

	if count > 0 {
		r.log.Info().Int("enqueued", count).Msg("CronWorker: enqueued scheduled scans")
	}
}


// scanBridge implement HTTP handler và scan execution logic.
type scanBridge struct {
	db    *pgxpool.Pool
	nc    *nats.Conn
	jobCh chan ScanJob
	cfg   ScanRunnerConfig
	log   zerolog.Logger
}

func newScanBridge(db *pgxpool.Pool, nc *nats.Conn, jobCh chan ScanJob, cfg ScanRunnerConfig, l zerolog.Logger) *scanBridge {
	return &scanBridge{db: db, nc: nc, jobCh: jobCh, cfg: cfg, log: l}
}

// router tạo chi.Router với scan endpoints.
func (b *scanBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1/scans", func(r chi.Router) {
		r.Post("/", b.createScan)
		r.Get("/", b.listScans)
		r.Get("/{id}", b.getScan)
		r.Delete("/{id}", b.cancelScan)
		r.Get("/{id}/findings", b.getScanFindings)
	})
	return r
}

// createScan handles POST /api/v1/scans
func (b *scanBridge) createScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Targets  []string `json:"targets"`
		ScanType string   `json:"scan_type"`
		Priority int      `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONScan(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if len(req.Targets) == 0 {
		writeJSONScan(w, http.StatusBadRequest, map[string]string{"error": "targets required"})
		return
	}

	scanID := uuid.New()
	userID, _ := r.Context().Value("user_id").(string)

	targetsJSON, _ := json.Marshal(req.Targets)
	_, err := b.db.Exec(r.Context(), `
		INSERT INTO scans (id, user_id, targets, scan_type, status, priority, created_at, updated_at)
		VALUES ($1, $2::uuid, $3, $4, 'pending', $5, NOW(), NOW())
	`, scanID, userID, targetsJSON, req.ScanType, req.Priority)
	if err != nil {
		b.log.Error().Err(err).Msg("create scan db error")
		writeJSONScan(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	// Enqueue job
	select {
	case b.jobCh <- ScanJob{ScanID: scanID, UserID: userID}:
	default:
		b.log.Warn().Msg("scan queue full")
	}

	writeJSONScan(w, http.StatusAccepted, map[string]interface{}{
		"scan_id": scanID,
		"status":  "pending",
		"targets": req.Targets,
		"message": "scan queued",
	})
}

// listScans handles GET /api/v1/scans
func (b *scanBridge) listScans(w http.ResponseWriter, r *http.Request) {
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, status, scan_type, targets, created_at, updated_at
		FROM scans ORDER BY created_at DESC LIMIT 50
	`)
	if err != nil {
		writeJSONScan(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var scans []map[string]interface{}
	for rows.Next() {
		var id, status, scanType string
		var targets []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &status, &scanType, &targets, &createdAt, &updatedAt); err != nil {
			continue
		}
		scans = append(scans, map[string]interface{}{
			"id": id, "status": status, "scan_type": scanType,
			"targets": json.RawMessage(targets),
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if scans == nil {
		scans = []map[string]interface{}{}
	}
	writeJSONScan(w, http.StatusOK, map[string]interface{}{"scans": scans, "total": len(scans)})
}

// getScan handles GET /api/v1/scans/{id}
func (b *scanBridge) getScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var (
		scanID, status, scanType string
		targets                  []byte
		createdAt, updatedAt     time.Time
		findingCount             int
	)
	err := b.db.QueryRow(r.Context(), `
		SELECT id::text, status, scan_type, targets, created_at, updated_at,
		       COALESCE(finding_count, 0)
		FROM scans WHERE id = $1::uuid
	`, id).Scan(&scanID, &status, &scanType, &targets, &createdAt, &updatedAt, &findingCount)
	if err != nil {
		writeJSONScan(w, http.StatusNotFound, map[string]string{"error": "scan not found"})
		return
	}
	writeJSONScan(w, http.StatusOK, map[string]interface{}{
		"id": scanID, "status": status, "scan_type": scanType,
		"targets": json.RawMessage(targets),
		"finding_count": findingCount,
		"created_at": createdAt, "updated_at": updatedAt,
	})
}

// cancelScan handles DELETE /api/v1/scans/{id}
func (b *scanBridge) cancelScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := b.db.Exec(r.Context(), `
		UPDATE scans SET status = 'cancelled', updated_at = NOW() WHERE id = $1::uuid
	`, id)
	if err != nil {
		writeJSONScan(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONScan(w, http.StatusOK, map[string]string{"message": "scan cancelled"})
}

// getScanFindings handles GET /api/v1/scans/{id}/findings
func (b *scanBridge) getScanFindings(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "id")
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, title, severity, component_name, component_version, cve
		FROM findings WHERE test_id = (
			SELECT id FROM tests WHERE scan_id = $1::uuid LIMIT 1
		) ORDER BY severity DESC LIMIT 100
	`, scanID)
	if err != nil {
		writeJSONScan(w, http.StatusOK, map[string]interface{}{"findings": []interface{}{}, "total": 0})
		return
	}
	defer rows.Close()

	var findings []map[string]interface{}
	for rows.Next() {
		var id, title, severity, compName, compVer, cve string
		if err := rows.Scan(&id, &title, &severity, &compName, &compVer, &cve); err != nil {
			continue
		}
		findings = append(findings, map[string]interface{}{
			"id": id, "title": title, "severity": severity,
			"component_name": compName, "component_version": compVer, "cve": cve,
		})
	}
	if findings == nil {
		findings = []map[string]interface{}{}
	}
	writeJSONScan(w, http.StatusOK, map[string]interface{}{"findings": findings, "total": len(findings)})
}

// worker thực thi scan jobs từ channel.
func (b *scanBridge) worker(ctx context.Context, id int) {
	b.log.Info().Int("worker", id).Msg("scan worker started")
	for {
		select {
		case <-ctx.Done():
			b.log.Info().Int("worker", id).Msg("scan worker stopped")
			return
		case job := <-b.jobCh:
			b.executeScan(ctx, job)
		}
	}
}

// executeScan thực thi một scan job.
func (b *scanBridge) executeScan(ctx context.Context, job ScanJob) {
	b.log.Info().Stringer("scan_id", job.ScanID).Msg("executing scan")

	// Mark running
	b.db.Exec(ctx, `UPDATE scans SET status = 'running', started_at = NOW(), updated_at = NOW() WHERE id = $1`, job.ScanID) //nolint:errcheck

	// Lấy targets từ DB
	var targetsJSON []byte
	var scanType string
	err := b.db.QueryRow(ctx, `SELECT targets, scan_type FROM scans WHERE id = $1`, job.ScanID).
		Scan(&targetsJSON, &scanType)
	if err != nil {
		b.markFailed(ctx, job.ScanID, "load scan: "+err.Error())
		return
	}

	var targets []string
	json.Unmarshal(targetsJSON, &targets) //nolint:errcheck

	// Simulate scan (real nmap integration — placeholder)
	// TODO: Wire to scan-service nmap adapter trực tiếp
	b.log.Info().Strs("targets", targets).Str("type", scanType).Msg("scan running (nmap simulation)")
	time.Sleep(2 * time.Second) // simulate work

	// Mark completed
	b.db.Exec(ctx, `UPDATE scans SET status = 'completed', completed_at = NOW(), updated_at = NOW() WHERE id = $1`, job.ScanID) //nolint:errcheck

	// Publish NATS scan.completed event
	if b.nc != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"scan_id":       job.ScanID.String(),
			"finding_count": 0,
			"status":        "completed",
		})
		b.nc.Publish("scan.completed", payload) //nolint:errcheck
	}

	b.log.Info().Stringer("scan_id", job.ScanID).Msg("scan completed")
}

func (b *scanBridge) markFailed(ctx context.Context, scanID uuid.UUID, errMsg string) {
	b.db.Exec(ctx, `UPDATE scans SET status = 'failed', updated_at = NOW() WHERE id = $1`, scanID) //nolint:errcheck
	b.log.Error().Stringer("scan_id", scanID).Str("reason", errMsg).Msg("scan failed")
}

func writeJSONScan(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
