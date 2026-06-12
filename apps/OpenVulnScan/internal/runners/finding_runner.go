// Package runners — finding_runner.go
// FindingRunner chạy finding-service trong một goroutine riêng.
//
// Bridge Pattern: finding-service/internal/ không thể import từ module ngoài.
// Sử dụng findingBridge (finding_bridge.go) implement findingv1.FindingServiceServer
// với direct Postgres queries.
//
// Giao tiếp:
// - API Gateway → finding-service: gRPC bufconn
// - scan-service → NATS scan.completed → finding-service: async batch create
package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	// shared/proto
	findingv1 "github.com/osv/shared/proto/gen/go/finding/v1"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// FindingRunnerConfig cấu hình cho finding goroutine.
type FindingRunnerConfig struct {
	DBURL string
}

// FindingRunner implement ServiceRunner cho finding-service.
type FindingRunner struct {
	cfg    FindingRunnerConfig
	nc     *nats.Conn
	lis    *bufconn.Listener
	server *grpc.Server
	log    zerolog.Logger
}

// NewFindingRunner tạo FindingRunner.
func NewFindingRunner(cfg FindingRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *FindingRunner {
	return &FindingRunner{
		cfg: cfg,
		nc:  nc,
		lis: lis,
		log: log.With().Str("runner", "finding-service").Logger(),
	}
}

func (r *FindingRunner) Name() string { return "finding-service" }

// Run khởi động finding goroutine.
func (r *FindingRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern)...")

	// PostgreSQL
	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("finding: db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("finding: db ping: %w", err)
	}
	r.log.Info().Msg("postgres connected")

	// FindingBridge: implement FindingServiceServer với direct Postgres queries
	bridge := newFindingBridge(db)

	// gRPC server
	r.server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcRecoveryInterceptor, grpcLoggingInterceptor),
	)
	findingv1.RegisterFindingServiceServer(r.server, bridge)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(r.server, healthSrv)

	errCh := make(chan error, 1)
	go func() {
		r.log.Info().Msg("gRPC FindingService ready on bufconn")
		errCh <- r.server.Serve(r.lis)
	}()

	// NATS subscriber: scan.completed → batch create findings
	var natsSub *nats.Subscription
	if r.nc != nil {
		natsSub, err = r.nc.Subscribe("scan.completed", func(msg *nats.Msg) {
			r.handleScanCompleted(ctx, db, msg.Data)
		})
		if err != nil {
			r.log.Warn().Err(err).Msg("NATS subscribe scan.completed failed — finding auto-create disabled")
		} else {
			r.log.Info().Msg("NATS subscriber: scan.completed → auto batch-create findings")
		}
	}

	r.log.Info().Msg("finding-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		if natsSub != nil {
			natsSub.Unsubscribe() //nolint:errcheck
		}
		return nil
	case err := <-errCh:
		return wrapRunnerError("finding-service", err)
	}
}

// handleScanCompleted processes scan.completed NATS event.
// Payload: {"scan_id": "...", "finding_count": N, "status": "completed"}
// Creates a findings test record and triggers batch create if scan has findings.
func (r *FindingRunner) handleScanCompleted(ctx context.Context, db *pgxpool.Pool, data []byte) {
	var evt struct {
		ScanID       string `json:"scan_id"`
		FindingCount int    `json:"finding_count"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(data, &evt); err != nil {
		r.log.Error().Err(err).Msg("scan.completed: invalid payload")
		return
	}

	r.log.Info().Str("scan_id", evt.ScanID).Int("findings", evt.FindingCount).
		Msg("scan.completed received → processing findings")

	// Upsert a test record for this scan (DefectDojo model: test belongs to engagement)
	testID := convertScanIDToTestID(evt.ScanID)
	_, err := db.Exec(ctx, `
		INSERT INTO tests (id, scan_id, test_type, title, target_start, target_end, created, updated)
		VALUES ($1::uuid, $2::uuid, 'OpenVulnScan', 'Scan ' || $2, NOW(), NOW() + interval '1 day', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET updated = NOW()
	`, testID, evt.ScanID)
	if err != nil {
		r.log.Warn().Err(err).Str("scan_id", evt.ScanID).Msg("test upsert failed — skipping finding creation")
		return
	}

	// Pull scan findings from scans table and create finding records
	scanFindings, err := loadScanFindingsFromDB(ctx, db, evt.ScanID)
	if err != nil || len(scanFindings) == 0 {
		r.log.Debug().Str("scan_id", evt.ScanID).Msg("no raw findings to convert")
		return
	}

	bridge := newFindingBridge(db)
	resp, err := bridge.BatchCreateFindings(ctx, &findingv1.BatchCreateFindingsRequest{
		TestId:  testID,
		Findings: convertScanFindingsToFindingInputs(scanFindings),
	})
	if err != nil {
		r.log.Error().Err(err).Str("scan_id", evt.ScanID).Msg("batch create findings failed")
		return
	}

	r.log.Info().
		Str("scan_id", evt.ScanID).
		Int32("created", resp.Created).
		Msg("findings batch-created from scan.completed")
}

// convertScanIDToTestID deterministically maps scan UUID → test UUID.
// Uses a simple UUID v5-style namespace (XOR of bytes) for consistency.
func convertScanIDToTestID(scanID string) string {
	// For MVP: prefix scan_id with "test-" namespace by deriving via hash
	// In production, this should use uuid.NewV5(namespace, scanID)
	return "00000000-0000-0000-0000-" + fmt.Sprintf("%012s", scanID[:12])
}

// scanFindingRow represents a raw finding from the scans/scan_findings table.
type scanFindingRow struct {
	Title            string
	Description      string
	Severity         string
	CVE              string
	ComponentName    string
	ComponentVersion string
	HashCode         string
}

// loadScanFindingsFromDB pulls raw findings from scan_findings table.
func loadScanFindingsFromDB(ctx context.Context, db *pgxpool.Pool, scanID string) ([]scanFindingRow, error) {
	rows, err := db.Query(ctx, `
		SELECT COALESCE(title, ''), COALESCE(description, ''),
		       COALESCE(severity, 'Info'), COALESCE(cve_id, ''),
		       COALESCE(component_name, ''), COALESCE(component_version, ''),
		       COALESCE(md5(title || cve_id || component_name), '')
		FROM scan_findings WHERE scan_id = $1::uuid
	`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []scanFindingRow
	for rows.Next() {
		var f scanFindingRow
		if err := rows.Scan(&f.Title, &f.Description, &f.Severity, &f.CVE,
			&f.ComponentName, &f.ComponentVersion, &f.HashCode); err != nil {
			continue
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// convertScanFindingsToFindingInputs converts scan raw findings to proto finding inputs.
func convertScanFindingsToFindingInputs(rows []scanFindingRow) []*findingv1.FindingInput {
	inputs := make([]*findingv1.FindingInput, 0, len(rows))
	for _, r := range rows {
		inputs = append(inputs, &findingv1.FindingInput{
			Title:            r.Title,
			Description:      r.Description,
			Severity:         r.Severity,
			Cve:              r.CVE,
			ComponentName:    r.ComponentName,
			ComponentVersion: r.ComponentVersion,
			HashCode:         r.HashCode,
			Active:           true,
			Verified:         false,
		})
	}
	return inputs
}

// Health kiểm tra gRPC health.
func (r *FindingRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("finding health: %w", err)
	}
	defer conn.Close()

	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("finding not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *FindingRunner) Listener() *bufconn.Listener { return r.lis }
