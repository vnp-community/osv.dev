// Command osv-scan scans a container image, local directory, or SBOM file
// for vulnerabilities using scan-service and the OSV database.
//
// This is a NEW command — no existing CLI code is modified.
//
// Usage:
//
//	# Scan a container image:
//	osv-scan -target nginx:latest -type image
//
//	# Scan a local directory:
//	osv-scan -target /path/to/project -type dir
//
//	# Scan from SBOM file:
//	osv-scan -target /path/to/sbom.json -type sbom
//
// Environment variables:
//
//	SCAN_SERVICE_HTTP  — scan-service HTTP address (default: http://localhost:8087)
//	OSV_API_BASE       — OSV API base URL (default: http://localhost:8080 = local gateway)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ScanRequest is sent to POST /v1/scans on scan-service.
type ScanRequest struct {
	Target   string `json:"target"`
	ScanType string `json:"scan_type"` // "image", "dir", "sbom"
	OutputFormat string `json:"output_format"` // "table", "json"
}

// ScanResponse from scan-service.
type ScanResponse struct {
	JobID     string     `json:"job_id"`
	Status    string     `json:"status"`
	VulnCount int        `json:"vuln_count"`
	Findings  []Finding  `json:"findings,omitempty"`
}

// Finding represents a single vulnerability found during a scan.
type Finding struct {
	CVEID        string   `json:"cve_id"`
	PackageName  string   `json:"package_name"`
	InstalledVer string   `json:"installed_ver"`
	FixedVer     string   `json:"fixed_ver,omitempty"`
	Severity     string   `json:"severity"`
	Description  string   `json:"description,omitempty"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("scan failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	target  := flag.String("target", "", "Scan target: image name, directory path, or SBOM file (required)")
	scanType := flag.String("type", "image", "Scan type: image | dir | sbom")
	format  := flag.String("output", "table", "Output format: table | json")
	timeout := flag.Duration("timeout", 10*time.Minute, "Scan timeout")
	flag.Parse()

	if *target == "" {
		return fmt.Errorf("-target is required")
	}

	scanServiceURL := os.Getenv("SCAN_SERVICE_HTTP")
	if scanServiceURL == "" {
		scanServiceURL = "http://localhost:8087"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	slog.Info("starting scan",
		slog.String("target", *target),
		slog.String("type", *scanType),
	)

	resp, err := submitScan(ctx, scanServiceURL, *target, *scanType, *format)
	if err != nil {
		return fmt.Errorf("scan submit: %w", err)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	default:
		printTable(resp)
	}
	return nil
}

func submitScan(ctx context.Context, serviceURL, target, scanType, format string) (*ScanResponse, error) {
	reqBody, _ := json.Marshal(ScanRequest{
		Target:       target,
		ScanType:     scanType,
		OutputFormat: format,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		serviceURL+"/v1/scans", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 0} // context controls timeout
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /v1/scans: %w", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("scan-service returned %d: %s", httpResp.StatusCode, body)
	}

	var result ScanResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode scan response: %w", err)
	}
	return &result, nil
}

func printTable(resp *ScanResponse) {
	fmt.Printf("\nScan Job: %s | Status: %s | Total: %d vulnerabilities\n",
		resp.JobID, resp.Status, resp.VulnCount)
	if len(resp.Findings) == 0 {
		fmt.Println("No vulnerabilities found.")
		return
	}
	fmt.Printf("\n%-20s %-30s %-12s %-12s %-10s\n",
		"CVE ID", "Package", "Installed", "Fixed", "Severity")
	fmt.Println(repeatChar('-', 90))
	for _, f := range resp.Findings {
		fmt.Printf("%-20s %-30s %-12s %-12s %-10s\n",
			f.CVEID, f.PackageName, f.InstalledVer,
			orDash(f.FixedVer), f.Severity)
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
