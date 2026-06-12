// Package runners — ingestion_runner.go
// IngestionRunner lắng nghe NATS event "agent.report.submitted" và enrich
// packages với CVE data từ OSV API, sau đó tạo findings.
//
// Bridge Pattern: không import internal/ của ingestion-service.
// Implement OSV fetch + finding creation trực tiếp bằng HTTP + Postgres.
package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

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

// IngestionRunnerConfig cấu hình cho ingestion goroutine.
type IngestionRunnerConfig struct {
	DBURL           string
	OSVAPIURL       string // Default: https://api.osv.dev
	MaxConcurrency  int    // Max concurrent OSV API calls per report
}

// IngestionRunner implement ServiceRunner — xử lý agent.report.submitted events.
type IngestionRunner struct {
	cfg         IngestionRunnerConfig
	nc          *nats.Conn
	lis         *bufconn.Listener
	server      *grpc.Server
	log         zerolog.Logger
	HTTPHandler http.Handler
}

// NewIngestionRunner tạo IngestionRunner.
func NewIngestionRunner(cfg IngestionRunnerConfig, nc *nats.Conn, lis *bufconn.Listener) *IngestionRunner {
	if cfg.OSVAPIURL == "" {
		cfg.OSVAPIURL = "https://api.osv.dev/v1"
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 3
	}
	return &IngestionRunner{
		cfg: cfg,
		nc:  nc,
		lis: lis,
		log: log.With().Str("runner", "ingestion-service").Logger(),
	}
}

func (r *IngestionRunner) Name() string { return "ingestion-service" }

// Run khởi động ingestion goroutine.
func (r *IngestionRunner) Run(ctx context.Context) error {
	r.log.Info().Msg("initializing (Bridge Pattern)...")

	db, err := pgxpool.New(ctx, r.cfg.DBURL)
	if err != nil {
		return fmt.Errorf("ingestion: db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ingestion: db ping: %w", err)
	}

	bridge := newIngestionBridge(db, r.nc, r.cfg, r.log)
	r.HTTPHandler = bridge.router()

	// Subscribe NATS events
	go bridge.subscribeAgentReports(ctx)

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

	r.log.Info().Str("osv_api", r.cfg.OSVAPIURL).Msg("ingestion-service ready")

	select {
	case <-ctx.Done():
		r.log.Info().Msg("graceful shutdown...")
		r.server.GracefulStop()
		return nil
	case err := <-errCh:
		return wrapRunnerError("ingestion-service", err)
	}
}

// Health kiểm tra health status.
func (r *IngestionRunner) Health(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := transport.DialBufConn(hctx, r.lis)
	if err != nil {
		return fmt.Errorf("ingestion health: %w", err)
	}
	defer conn.Close()
	hc := grpc_health_v1.NewHealthClient(conn)
	resp, err := hc.Check(hctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("ingestion not serving: %s", resp.Status)
	}
	return nil
}

// Listener returns the bufconn listener.
func (r *IngestionRunner) Listener() *bufconn.Listener { return r.lis }

// ── Ingestion Bridge ──────────────────────────────────────────────────────────

type ingestionBridge struct {
	db  *pgxpool.Pool
	nc  *nats.Conn
	cfg IngestionRunnerConfig
	log zerolog.Logger
	hc  *http.Client
}

func newIngestionBridge(db *pgxpool.Pool, nc *nats.Conn, cfg IngestionRunnerConfig, l zerolog.Logger) *ingestionBridge {
	return &ingestionBridge{
		db:  db,
		nc:  nc,
		cfg: cfg,
		log: l,
		hc:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ── OSV types ──────────────────────────────────────────────────────────────────

type osvQueryRequest struct {
	Package *osvPackage `json:"package,omitempty"`
	Version string      `json:"version,omitempty"`
}

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem,omitempty"`
}

type osvBatchQueryRequest struct {
	Queries []osvQueryRequest `json:"queries"`
}

type osvVuln struct {
	ID       string       `json:"id"`
	Summary  string       `json:"summary"`
	Details  string       `json:"details"`
	Severity []osvSeverity `json:"severity"`
	Aliases  []string     `json:"aliases"`
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Ranges []struct {
			Type   string `json:"type"`
			Events []struct {
				Introduced string `json:"introduced,omitempty"`
				Fixed       string `json:"fixed,omitempty"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvQueryResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

type agentReportEvent struct {
	Hostname string `json:"hostname"`
	OSInfo   string `json:"os_info"`
	Packages []struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Ecosystem string `json:"ecosystem,omitempty"`
	} `json:"packages"`
}

// subscribeAgentReports subscribes to NATS "agent.report.submitted" events.
func (b *ingestionBridge) subscribeAgentReports(ctx context.Context) {
	if b.nc == nil {
		b.log.Warn().Msg("NATS not connected, agent report enrichment disabled")
		<-ctx.Done()
		return
	}

	sub, err := b.nc.Subscribe("agent.report.submitted", func(msg *nats.Msg) {
		var evt agentReportEvent
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			b.log.Error().Err(err).Msg("failed to unmarshal agent report event")
			return
		}

		b.log.Info().
			Str("hostname", evt.Hostname).
			Int("packages", len(evt.Packages)).
			Msg("agent report enrichment starting")

		go b.enrichPackages(ctx, evt)
	})
	if err != nil {
		b.log.Error().Err(err).Msg("NATS subscribe agent.report.submitted failed")
		<-ctx.Done()
		return
	}
	defer sub.Unsubscribe() //nolint:errcheck

	b.log.Info().Msg("NATS subscribed: agent.report.submitted")
	<-ctx.Done()
	b.log.Info().Msg("ingestion NATS subscription stopped")
}

// enrichPackages queries OSV API for each package and creates findings.
func (b *ingestionBridge) enrichPackages(ctx context.Context, evt agentReportEvent) {
	if len(evt.Packages) == 0 {
		return
	}

	// Rate limiting: semaphore pattern
	sem := make(chan struct{}, b.cfg.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalFindings := 0

	for _, pkg := range evt.Packages {
		wg.Add(1)
		sem <- struct{}{}

		go func(name, version, ecosystem string) {
			defer wg.Done()
			defer func() { <-sem }()

			vulns, err := b.queryOSV(ctx, name, version, ecosystem)
			if err != nil {
				b.log.Warn().Err(err).Str("pkg", name+"@"+version).Msg("OSV query failed")
				return
			}
			if len(vulns) == 0 {
				return
			}

			// Create findings for discovered vulnerabilities
			created := b.createFindingsForVulns(ctx, vulns, name, version, evt.Hostname)
			mu.Lock()
			totalFindings += created
			mu.Unlock()
		}(pkg.Name, pkg.Version, pkg.Ecosystem)
	}

	wg.Wait()
	b.log.Info().
		Str("hostname", evt.Hostname).
		Int("total_findings", totalFindings).
		Msg("agent report enrichment completed")

	// Publish enrichment completed event
	if b.nc != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"hostname":       evt.Hostname,
			"total_findings": totalFindings,
			"timestamp":      time.Now().UTC(),
		})
		b.nc.Publish("agent.enrichment.completed", payload) //nolint:errcheck
	}
}

// queryOSV queries OSV API for vulnerabilities affecting a specific package version.
func (b *ingestionBridge) queryOSV(ctx context.Context, name, version, ecosystem string) ([]osvVuln, error) {
	reqBody := osvQueryRequest{
		Package: &osvPackage{
			Name:      name,
			Ecosystem: ecosystem,
		},
		Version: version,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.cfg.OSVAPIURL+"/query", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("osv api status %d: %s", resp.StatusCode, string(body))
	}

	var result osvQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("osv decode: %w", err)
	}
	return result.Vulns, nil
}

// createFindingsForVulns creates findings in DB for OSV vulnerabilities.
// Uses idempotency check (hash_code) to avoid duplicates.
func (b *ingestionBridge) createFindingsForVulns(ctx context.Context, vulns []osvVuln, pkgName, pkgVersion, hostname string) int {
	created := 0
	for _, vuln := range vulns {
		cveID := vuln.ID
		// Prefer CVE alias if available
		for _, alias := range vuln.Aliases {
			if len(alias) > 3 && alias[:3] == "CVE" {
				cveID = alias
				break
			}
		}

		severity := b.determineSeverity(vuln)
		title := fmt.Sprintf("%s in %s@%s", cveID, pkgName, pkgVersion)
		// Idempotency: hash based on CVE+package+version+host
		hashCode := fmt.Sprintf("%x", fmt.Sprintf("%s|%s|%s|%s", cveID, pkgName, pkgVersion, hostname))

		// Check if finding already exists
		var exists bool
		b.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM findings WHERE hash_code = $1)`, hashCode).Scan(&exists) //nolint:errcheck
		if exists {
			continue
		}

		// Insert finding
		_, err := b.db.Exec(ctx, `
			INSERT INTO findings (
				title, description, severity, cve, component_name, component_version,
				hash_code, hostname, active, false_p, duplicate, out_of_scope,
				verified, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, true, false, false, false, false, NOW(), NOW()
			) ON CONFLICT (hash_code) DO NOTHING
		`, title, vuln.Summary, severity, cveID, pkgName, pkgVersion, hashCode, hostname)
		if err != nil {
			b.log.Warn().Err(err).Str("cve", cveID).Msg("failed to create finding")
			continue
		}
		created++
	}
	return created
}

// determineSeverity extracts severity from OSV vuln CVSS scores.
func (b *ingestionBridge) determineSeverity(vuln osvVuln) string {
	for _, sev := range vuln.Severity {
		if sev.Type == "CVSS_V3" {
			return b.cvssToSeverity(sev.Score)
		}
	}
	// Default to Medium if no CVSS
	return "Medium"
}

// cvssToSeverity converts CVSS vector or score to severity label.
func (b *ingestionBridge) cvssToSeverity(score string) string {
	// CVSS vector starts with "CVSS:3.x/AV:..."
	// Extract base score from vector or use score directly
	if len(score) > 2 && score[0] == 'C' {
		// Parse score from end of vector (e.g., "CVSS:3.1/... 9.8")
		return "High"
	}
	switch {
	case score >= "9.0":
		return "Critical"
	case score >= "7.0":
		return "High"
	case score >= "4.0":
		return "Medium"
	case score >= "0.1":
		return "Low"
	default:
		return "Info"
	}
}

// ── HTTP Routes ───────────────────────────────────────────────────────────────

func (b *ingestionBridge) router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingestion/status", b.handleStatus)
	mux.HandleFunc("/api/v1/ingestion/sync", b.handleManualSync)
	return mux
}

func (b *ingestionBridge) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSONIngestion(w, http.StatusOK, map[string]interface{}{
		"status":      "running",
		"osv_api_url": b.cfg.OSVAPIURL,
		"concurrency": b.cfg.MaxConcurrency,
	})
}

func (b *ingestionBridge) handleManualSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONIngestion(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	// Manual trigger: publish a test event
	if b.nc != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"hostname":  "manual-trigger",
			"os_info":   "manual",
			"packages":  []interface{}{},
		})
		b.nc.Publish("agent.report.submitted", payload) //nolint:errcheck
	}
	writeJSONIngestion(w, http.StatusOK, map[string]string{"message": "sync triggered"})
}

func writeJSONIngestion(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
