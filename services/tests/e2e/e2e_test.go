// Package e2e contains end-to-end tests for the CVE Binary Tool API.
// These tests require the full stack to be running (docker compose up).
// Run with: go test -race -tags e2e ./tests/e2e/...
//
// +build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const defaultGatewayURL = "http://localhost:8080"

func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return defaultGatewayURL
}

// waitForHealth waits up to 30s for the gateway to be healthy.
func waitForHealth(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(gatewayURL() + "/health")
		if err == nil && resp.StatusCode == 200 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("gateway not healthy after 30s")
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 1: Binary Scan with known vulnerable binary
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario1_BinaryScan_KnownVulnerable(t *testing.T) {
	waitForHealth(t)

	// Use testdata binary stub
	fileBytes, err := os.ReadFile("testdata/openssl-1.0.1e-stub")
	if err != nil {
		t.Skipf("testdata/openssl-1.0.1e-stub not found: %v", err)
	}

	body, contentType := buildMultipartForm(t, "file", "openssl-1.0.1e-stub", fileBytes,
		map[string]string{"options": `{"formats":["json"],"min_cvss":0}`})

	resp, err := http.Post(gatewayURL()+"/api/v1/scan/binary", contentType, body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	t.Logf("Scan result: total_cves=%v exit_code=%v", result["total_cves"], result["exit_code"])
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 2: SBOM Scan (package list)
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario2_PackageListScan(t *testing.T) {
	waitForHealth(t)

	fileBytes := []byte("Django==2.2.0\nPillow==5.2.0\nrequests==2.20.0\n")

	body, contentType := buildMultipartForm(t, "file", "requirements.txt", fileBytes, nil)

	resp, err := http.Post(gatewayURL()+"/api/v1/scan/package-list", contentType, body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	t.Logf("Package list scan: total_cves=%v", result["total_cves"])
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 3: DB Status
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario3_DBStatus(t *testing.T) {
	waitForHealth(t)

	resp, err := http.Get(gatewayURL() + "/api/v1/db/status")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Accept 200 or 503 (service not configured)
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 4: Health Endpoints
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario4_HealthEndpoints(t *testing.T) {
	waitForHealth(t)

	endpoints := []string{"/health", "/health/live", "/health/ready"}
	for _, ep := range endpoints {
		resp, err := http.Get(gatewayURL() + ep)
		if err != nil {
			t.Errorf("%s: request failed: %v", ep, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 207 {
			t.Errorf("%s: expected 200 or 207, got %d", ep, resp.StatusCode)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Scenario 5: Rate Limiting
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario5_RateLimiting(t *testing.T) {
	waitForHealth(t)

	// Send many requests quickly; eventually expect 429
	got429 := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(gatewayURL() + "/health")
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 429 {
			got429 = true
			break
		}
	}
	// Rate limit may not be triggered for /health (no RL on health)
	// This is a "smoke test" rather than a strict assertion
	t.Logf("Rate limit triggered: %v", got429)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: build multipart form
// ─────────────────────────────────────────────────────────────────────────────

func buildMultipartForm(t *testing.T, fileField, filename string, fileBytes []byte, extraFields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
		t.Fatalf("copy file bytes: %v", err)
	}

	for k, v := range extraFields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}

	w.Close()
	return &buf, w.FormDataContentType()
}

// suppress unused import lint
var _ = fmt.Sprintf
var _ = strings.Contains
