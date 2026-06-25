# SB-CLI-05 — [NEW] cmd/scan — Security Scan CLI

## Metadata
- **Task ID**: SB-CLI-05
- **Sprint**: B (P1)
- **Ước tính**: 2 giờ
- **Dependencies**: SA-SHARED-01
- **Spec nguồn**: `specs/solutions/enhance-cli-app/02_cli-upgrade.md` § "2.5 [NEW] cmd/scan"

---

## Context

```bash
# Xem scan-service HTTP handler để biết API
cat services/scan-service/internal/delivery/http/schedule/schedule_handler.go | head -50

# Xem trivy adapter để hiểu scan types
cat services/scan-service/adapters/scanner/trivy/trivy_client.go | head -60
```

---

## Goal

Tạo command mới `osv-scan` cho phép security team trigger container/filesystem scans qua scan-service REST API từ CLI.

---

## Files to Create

### `apps/cli/cmd/scan/main.go`

```go
// Command scan submits a security scan job to the scan-service.
// It supports scanning container images, directories, and SBOM files.
//
// Usage:
//
//	osv-scan --target nginx:latest --type image
//	osv-scan --target /path/to/project --type dir
//	osv-scan --target /path/to/sbom.json --type sbom
//	osv-scan --target myapp:v1.2.3 --type image --wait --format table
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const defaultScanServiceURL = "http://localhost:8087"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fs := flag.NewFlagSet("osv-scan", flag.ExitOnError)
	target := fs.String("target", "", "Container image name or directory path (required)")
	scanType := fs.String("type", "image", "Scan type: image|dir|sbom")
	wait := fs.Bool("wait", false, "Wait for scan to complete and print results")
	format := fs.String("format", "json", "Output format: json|table")
	scanSvcURL := fs.String("scan-service", defaultScanServiceURL, "scan-service base URL")
	timeout := fs.Duration("timeout", 10*time.Minute, "Maximum time to wait for scan completion")
	fs.Parse(os.Args[1:])

	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	scanTypes := map[string]bool{"image": true, "dir": true, "sbom": true}
	if !scanTypes[*scanType] {
		return fmt.Errorf("invalid --type: must be image|dir|sbom")
	}

	// Submit scan job
	jobID, err := submitScan(ctx, *scanSvcURL, *target, *scanType)
	if err != nil {
		return fmt.Errorf("submit scan: %w", err)
	}
	fmt.Printf("Scan job submitted: %s\n", jobID)

	if !*wait {
		fmt.Printf("Use --wait to wait for results, or poll: %s/api/v1/scans/%s\n", *scanSvcURL, jobID)
		return nil
	}

	// Wait for completion
	fmt.Printf("Waiting for scan to complete (timeout: %s)...\n", *timeout)
	result, err := pollScanResult(ctx, *scanSvcURL, jobID, *timeout)
	if err != nil {
		return fmt.Errorf("poll result: %w", err)
	}

	// Output
	return printResult(result, *format)
}

type submitScanRequest struct {
	Target   string `json:"target"`
	ScanType string `json:"scan_type"`
}

type scanJob struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type scanResult struct {
	JobID     string      `json:"job_id"`
	Status    string      `json:"status"`
	VulnCount int         `json:"vuln_count"`
	Vulns     interface{} `json:"vulns,omitempty"`
}

func submitScan(ctx context.Context, baseURL, target, scanType string) (string, error) {
	body, _ := json.Marshal(submitScanRequest{Target: target, ScanType: scanType})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/api/v1/scans", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect to scan-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scan-service returned %d", resp.StatusCode)
	}

	var job scanJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return "", err
	}
	return job.JobID, nil
}

func pollScanResult(ctx context.Context, baseURL, jobID string, maxWait time.Duration) (*scanResult, error) {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/api/v1/scans/%s", baseURL, jobID), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue // retry
		}
		var result scanResult
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.Status == "completed" || result.Status == "failed" {
			return &result, nil
		}
		fmt.Printf("  status: %s...\n", result.Status)
	}
	return nil, fmt.Errorf("scan timed out after %s", maxWait)
}

func printResult(result *scanResult, format string) error {
	if format == "json" {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	// table format
	fmt.Printf("\n=== Scan Results ===\n")
	fmt.Printf("Job ID:     %s\n", result.JobID)
	fmt.Printf("Status:     %s\n", result.Status)
	fmt.Printf("Vulns Found: %d\n", result.VulnCount)
	return nil
}
```

---

## Acceptance Criteria

- [ ] `apps/cli/cmd/scan/main.go` tạo
- [ ] `--target`, `--type`, `--wait`, `--format` flags
- [ ] Submit scan via `POST /api/v1/scans`
- [ ] Poll result via `GET /api/v1/scans/{jobID}`
- [ ] `--format table` và `--format json` output
- [ ] `go build ./cmd/scan/...` từ `apps/cli` PASS

---

## Verification

```bash
cd apps/cli
go build ./cmd/scan/...
./bin/osv-scan --help
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created (additive only)
- `apps/cli/cmd/scan/main.go` — NEW `osv-scan` command: submits scan job to scan-service HTTP API, hỗ trợ `image`, `dir`, `sbom` types, output `table`/`json`

### Build Verification
```
go build ./cmd/scan/...  → OK
```
