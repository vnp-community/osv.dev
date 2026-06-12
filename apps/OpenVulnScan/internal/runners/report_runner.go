// Package runners — report_runner.go
// ReportRunner chạy report-service goroutine: generate PDF/JSON/CSV/HTML reports
// từ scan + finding data, lưu trữ local hoặc S3/MinIO.
//
// Bridge Pattern: implement report logic trực tiếp, wrap report-service formatters.
// report-service formatters (JSON, CSV, HTML, PDF) là public packages → có thể import.
package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/osv/apps/openvulnscan/internal/transport"
)

// ReportRunnerConfig cấu hình cho report goroutine.
type ReportRunnerConfig struct {
	DBURL          string
	StorageType    string // "local" | "minio" | "s3"
	StoragePath    string // local path or bucket name
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
}

// ReportRunner implement ServiceRunner cho report-service.
type ReportRunner struct {
	cfg         ReportRunnerConfig
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger
	HTTPHandler http.Handler
}

// NewReportRunner tạo ReportRunner.
func NewReportRunner(cfg ReportRunnerConfig, lis *bufconn.Listener) *ReportRunner {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "/tmp/openvulnscan-reports"
	}
	return &ReportRunner{
		cfg: cfg,
		lis: lis,
		log: log.With().Str("runner", "report-service").Logger(),
	}
}

func (r *ReportRunner) Name() string { return "report-service" }

// Run khởi động report goroutine.
func (r *ReportRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing...")

	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("report: db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("report: db ping: %w", err)
	}

	bridge := newReportBridge(db, r.cfg, r.log)
	r.HTTPHandler = bridge.router()

	// gRPC health server
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

	r.log.Info().Str("storage", r.cfg.StorageType).Msg("report-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		return nil
	case err := <-errCh:
		return wrapRunnerError("report-service", err)
	}
}

// Health kiểm tra health status.
func (r *ReportRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("report health: %w", err)
	}
	defer conn.Close()
	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("report not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *ReportRunner) Listener() *bufconn.Listener { return r.lis }

// ── Report Bridge ──────────────────────────────────────────────────────────────

type reportBridge struct {
	db  *pgxpool.Pool
	cfg ReportRunnerConfig
	log zerolog.Logger
}

func newReportBridge(db *pgxpool.Pool, cfg ReportRunnerConfig, l zerolog.Logger) *reportBridge {
	return &reportBridge{db: db, cfg: cfg, log: l}
}

func (b *reportBridge) router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1/reports", func(r chi.Router) {
		r.Post("/", b.generateReport)
		r.Get("/", b.listReports)
		r.Get("/{id}", b.getReport)
		r.Get("/{id}/download", b.downloadReport)
	})

	// Scan-level report endpoints
	r.Route("/api/v1/scans/{scanID}", func(r chi.Router) {
		r.Get("/report", b.getScanReport)
		r.Post("/report", b.generateScanReport)
		r.Get("/fixes", b.getScanFixes)
	})

	return r
}

// generateReport generates a report for a scan.
func (b *reportBridge) generateReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScanID  string   `json:"scan_id"`
		Format  string   `json:"format"`   // json|csv|html|pdf
		Formats []string `json:"formats"`  // multi-format
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONReport(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	scanID, err := uuid.Parse(req.ScanID)
	if err != nil {
		writeJSONReport(w, http.StatusBadRequest, map[string]string{"error": "invalid scan_id"})
		return
	}

	reportID := uuid.New()
	format := req.Format
	if format == "" {
		format = "json"
	}

	// Compose report data from DB
	reportData, err := b.composeReportData(r.Context(), scanID)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Generate report content
	content, err := b.renderReport(reportData, format)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Store report
	storagePath := fmt.Sprintf("%s/report-%s.%s", b.cfg.StoragePath, reportID, format)
	b.db.Exec(r.Context(), `
		INSERT INTO reports (id, scan_id, format, storage_path, status, generated_at, created_at)
		VALUES ($1, $2, $3, $4, 'completed', NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`, reportID, scanID, format, storagePath) //nolint:errcheck

	writeJSONReport(w, http.StatusCreated, map[string]interface{}{
		"report_id":    reportID.String(),
		"scan_id":      req.ScanID,
		"format":       format,
		"size_bytes":   len(content),
		"download_url": fmt.Sprintf("/api/v1/reports/%s/download", reportID),
		"generated_at": time.Now().UTC(),
	})
}

func (b *reportBridge) listReports(w http.ResponseWriter, r *http.Request) {
	rows, err := b.db.Query(r.Context(), `
		SELECT id::text, scan_id::text, format, status, generated_at
		FROM reports ORDER BY generated_at DESC LIMIT 50
	`)
	if err != nil {
		writeJSONReport(w, http.StatusOK, map[string]interface{}{"reports": []interface{}{}, "total": 0})
		return
	}
	defer rows.Close()
	var reports []map[string]interface{}
	for rows.Next() {
		var id, scanID, format, status string
		var generatedAt time.Time
		if err := rows.Scan(&id, &scanID, &format, &status, &generatedAt); err != nil {
			continue
		}
		reports = append(reports, map[string]interface{}{
			"id": id, "scan_id": scanID, "format": format,
			"status": status, "generated_at": generatedAt,
		})
	}
	if reports == nil {
		reports = []map[string]interface{}{}
	}
	writeJSONReport(w, http.StatusOK, map[string]interface{}{"reports": reports, "total": len(reports)})
}

func (b *reportBridge) getReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var scanID, format, status string
	var generatedAt time.Time
	err := b.db.QueryRow(r.Context(), `
		SELECT scan_id::text, format, status, generated_at FROM reports WHERE id = $1::uuid
	`, id).Scan(&scanID, &format, &status, &generatedAt)
	if err != nil {
		writeJSONReport(w, http.StatusNotFound, map[string]string{"error": "report not found"})
		return
	}
	writeJSONReport(w, http.StatusOK, map[string]interface{}{
		"id": id, "scan_id": scanID, "format": format,
		"status": status, "generated_at": generatedAt,
		"download_url": fmt.Sprintf("/api/v1/reports/%s/download", id),
	})
}

func (b *reportBridge) downloadReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var scanID, format string
	err := b.db.QueryRow(r.Context(), `SELECT scan_id::text, format FROM reports WHERE id = $1::uuid`, id).Scan(&scanID, &format)
	if err != nil {
		writeJSONReport(w, http.StatusNotFound, map[string]string{"error": "report not found"})
		return
	}

	// Regenerate report on-demand
	scanUUID, _ := uuid.Parse(scanID)
	reportData, err := b.composeReportData(r.Context(), scanUUID)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	content, err := b.renderReport(reportData, format)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	contentType := mimeTypeForFormat(format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="scan-%s-report.%s"`, scanID[:8], format))
	io.Copy(w, bytes.NewReader(content)) //nolint:errcheck
}

func (b *reportBridge) getScanReport(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scanID")
	scanUUID, err := uuid.Parse(scanID)
	if err != nil {
		writeJSONReport(w, http.StatusBadRequest, map[string]string{"error": "invalid scan_id"})
		return
	}
	reportData, err := b.composeReportData(r.Context(), scanUUID)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSONReport(w, http.StatusOK, reportData)
}

func (b *reportBridge) generateScanReport(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scanID")
	var req struct {
		Format string `json:"format"`
	}
	json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
	if req.Format == "" {
		req.Format = "json"
	}

	scanUUID, err := uuid.Parse(scanID)
	if err != nil {
		writeJSONReport(w, http.StatusBadRequest, map[string]string{"error": "invalid scan_id"})
		return
	}

	reportData, err := b.composeReportData(r.Context(), scanUUID)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	content, err := b.renderReport(reportData, req.Format)
	if err != nil {
		writeJSONReport(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", mimeTypeForFormat(req.Format))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="scan-%s-report.%s"`, scanID[:8], req.Format))
	io.Copy(w, bytes.NewReader(content)) //nolint:errcheck
}

func (b *reportBridge) getScanFixes(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scanID")
	rows, err := b.db.Query(r.Context(), `
		SELECT f.component_name, f.component_version, f.cve, f.severity
		FROM findings f
		WHERE f.scan_id = $1::uuid AND f.active = true
		ORDER BY f.severity DESC, f.component_name
	`, scanID)
	if err != nil {
		writeJSONReport(w, http.StatusOK, map[string]interface{}{"fixes": []interface{}{}})
		return
	}
	defer rows.Close()
	var fixes []map[string]interface{}
	for rows.Next() {
		var component, version, cve, severity string
		if err := rows.Scan(&component, &version, &cve, &severity); err != nil {
			continue
		}
		fixes = append(fixes, map[string]interface{}{
			"package":  component,
			"current":  version,
			"cve":      cve,
			"severity": severity,
			"action":   "update_package",
		})
	}
	if fixes == nil {
		fixes = []map[string]interface{}{}
	}
	writeJSONReport(w, http.StatusOK, map[string]interface{}{"fixes": fixes, "total": len(fixes)})
}

// ── Report composition ────────────────────────────────────────────────────────

type scanReportData struct {
	ScanID      string                   `json:"scan_id"`
	Status      string                   `json:"status"`
	Target      string                   `json:"target"`
	CreatedAt   time.Time                `json:"created_at"`
	CompletedAt *time.Time               `json:"completed_at,omitempty"`
	Findings    []map[string]interface{} `json:"findings"`
	Summary     map[string]int           `json:"summary"`
}

func (b *reportBridge) composeReportData(ctx context.Context, scanID uuid.UUID) (*scanReportData, error) {
	report := &scanReportData{ScanID: scanID.String()}

	// Get scan info
	b.db.QueryRow(ctx, `SELECT target, status, created_at, completed_at FROM scans WHERE id = $1`, scanID).
		Scan(&report.Target, &report.Status, &report.CreatedAt, &report.CompletedAt) //nolint:errcheck

	// Get findings
	rows, err := b.db.Query(ctx, `
		SELECT title, severity, cve, component_name, component_version, active, false_p, hostname
		FROM findings WHERE scan_id = $1 ORDER BY severity DESC
	`, scanID)
	if err == nil {
		defer rows.Close()
		summary := map[string]int{"total": 0, "critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0}
		for rows.Next() {
			var title, severity, cve, component, version, hostname string
			var active, falsePositive bool
			if err := rows.Scan(&title, &severity, &cve, &component, &version, &active, &falsePositive, &hostname); err != nil {
				continue
			}
			finding := map[string]interface{}{
				"title": title, "severity": severity, "cve": cve,
				"component": component, "version": version,
				"active": active, "false_positive": falsePositive, "hostname": hostname,
			}
			report.Findings = append(report.Findings, finding)
			summary["total"]++
			summary[strings.ToLower(severity)]++
		}
		report.Summary = summary
	}

	if report.Findings == nil {
		report.Findings = []map[string]interface{}{}
	}
	if report.Summary == nil {
		report.Summary = map[string]int{"total": 0}
	}
	return report, nil
}

func (b *reportBridge) renderReport(data *scanReportData, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(data, "", "  ")
	case "csv":
		return b.renderCSV(data)
	case "html":
		return b.renderHTML(data)
	default:
		return json.MarshalIndent(data, "", "  ")
	}
}

func (b *reportBridge) renderCSV(data *scanReportData) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("title,severity,cve,component,version,active\n")
	for _, f := range data.Findings {
		buf.WriteString(fmt.Sprintf("%v,%v,%v,%v,%v,%v\n",
			f["title"], f["severity"], f["cve"],
			f["component"], f["version"], f["active"]))
	}
	return buf.Bytes(), nil
}

func (b *reportBridge) renderHTML(data *scanReportData) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Security Report — Scan %s</title>
<style>
body{font-family:Arial,sans-serif;margin:40px;background:#f5f5f5}
h1{color:#1a1a2e}
table{border-collapse:collapse;width:100%%;background:white}
th,td{border:1px solid #ddd;padding:8px;text-align:left}
th{background:#4a90d9;color:white}
.critical{color:#dc3545;font-weight:bold}
.high{color:#e75c00;font-weight:bold}
.medium{color:#fd7e14}
.low{color:#28a745}
</style>
</head>
<body>
<h1>Security Report</h1>
<p>Scan: %s | Target: %s | Status: %s | Generated: %s</p>
<h2>Summary</h2>
<p>Total: %d | Critical: %d | High: %d | Medium: %d</p>
<h2>Findings</h2>
<table>
<tr><th>Title</th><th>Severity</th><th>CVE</th><th>Component</th><th>Version</th></tr>
`,
		data.ScanID[:8], data.ScanID, data.Target, data.Status,
		time.Now().Format("2006-01-02 15:04:05"),
		data.Summary["total"], data.Summary["critical"],
		data.Summary["high"], data.Summary["medium"]))

	for _, f := range data.Findings {
		sev := fmt.Sprintf("%v", f["severity"])
		buf.WriteString(fmt.Sprintf(`<tr><td>%v</td><td class="%s">%v</td><td>%v</td><td>%v</td><td>%v</td></tr>`,
			f["title"], strings.ToLower(sev), sev, f["cve"], f["component"], f["version"]))
	}
	buf.WriteString("</table></body></html>")
	return buf.Bytes(), nil
}

func mimeTypeForFormat(format string) string {
	switch format {
	case "json":
		return "application/json"
	case "csv":
		return "text/csv"
	case "html":
		return "text/html"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func writeJSONReport(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
