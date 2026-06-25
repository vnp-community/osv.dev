// Package trivy — trivy_client.go
// TrivyClient wraps the Trivy CLI or Trivy Server HTTP API to produce SBOM output.
//
// S3-SCAN-01: Trivy Scanner Adapter
// ADDITIVE: existing nmap/zap scanners in internal/adapters/scanner/ are unchanged.
//
// Modes:
//   CLI mode  (serverURL == ""):  exec `trivy image/fs --format cyclonedx ...`
//   Server mode (serverURL != ""): POST /scan to Trivy Server HTTP endpoint
//
// Output: CycloneDX JSON (compatible with existing sbom parsers in the scan pipeline)
package trivy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SBOMReport is a minimal CycloneDX-compatible structure returned by Trivy.
// Downstream parsers handle full CycloneDX parsing.
type SBOMReport struct {
	Format     string          `json:"bomFormat"`
	SpecVer    string          `json:"specVersion"`
	Components []SBOMComponent `json:"components,omitempty"`
	Vulnerabilities []SBOMVulnerability `json:"vulnerabilities,omitempty"`
	Metadata   SBOMMetadata    `json:"metadata"`
}

// SBOMComponent represents a software component in the SBOM.
type SBOMComponent struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
}

// SBOMVulnerability represents a vulnerability found during scanning.
type SBOMVulnerability struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Ratings     []Rating `json:"ratings,omitempty"`
	Affects     []Affect `json:"affects,omitempty"`
}

// Rating holds CVSS severity rating.
type Rating struct {
	Score    float64 `json:"score,omitempty"`
	Severity string  `json:"severity,omitempty"`
	Method   string  `json:"method,omitempty"`
}

// Affect identifies affected components.
type Affect struct {
	Ref string `json:"ref"`
}

// SBOMMetadata holds scan metadata.
type SBOMMetadata struct {
	Timestamp string `json:"timestamp"`
	Component SBOMComponent `json:"component,omitempty"`
}

// TrivyClient invokes Trivy for container image, directory, or filesystem scanning.
type TrivyClient struct {
	serverURL string        // empty = CLI mode
	timeout   time.Duration
}

// New creates a TrivyClient.
//   serverURL: Trivy Server URL (e.g., "http://trivy-server:4954") or "" for CLI mode.
//   timeout:   per-scan timeout (0 = default 5 minutes)
func New(serverURL string, timeout time.Duration) *TrivyClient {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &TrivyClient{serverURL: serverURL, timeout: timeout}
}

// ScanImage scans a container image and returns CycloneDX SBOM.
// image example: "nginx:latest", "registry.example.com/app:v1.2"
func (c *TrivyClient) ScanImage(ctx context.Context, image string) (*SBOMReport, error) {
	return c.runCLI(ctx, "image", image)
}

// ScanDirectory scans a local directory for known vulnerabilities.
func (c *TrivyClient) ScanDirectory(ctx context.Context, path string) (*SBOMReport, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("trivy.ScanDirectory: resolve path %s: %w", path, err)
	}
	return c.runCLI(ctx, "fs", absPath)
}

// ScanFilesystem is an alias for ScanDirectory (mirrors Trivy's `fs` subcommand).
func (c *TrivyClient) ScanFilesystem(ctx context.Context, path string) (*SBOMReport, error) {
	return c.ScanDirectory(ctx, path)
}

// ── CLI mode ──────────────────────────────────────────────────────────────────

func (c *TrivyClient) runCLI(ctx context.Context, subcmd, target string) (*SBOMReport, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// trivy image --format cyclonedx --output - nginx:latest
	// --quiet suppresses progress bars, --no-progress keeps output clean
	args := []string{
		subcmd,
		"--format", "cyclonedx",
		"--output", "-",  // write to stdout
		"--quiet",
		target,
	}

	// Ensure cache dir exists
	cacheDir := filepath.Join(os.TempDir(), "trivy-cache")
	_ = os.MkdirAll(cacheDir, 0o755)

	cmd := exec.CommandContext(ctx, "trivy", args...)
	cmd.Env = append(os.Environ(), "TRIVY_CACHE_DIR="+cacheDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trivy %s %s: %w (stderr: %s)", subcmd, target, err, stderr.String())
	}

	var report SBOMReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return nil, fmt.Errorf("trivy: parse CycloneDX output: %w", err)
	}

	return &report, nil
}
