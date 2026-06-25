// osv_query_test.go — Integration tests for OSV v1 query API via gateway.
// Tests: GET /v1/vulns/{id}, POST /v1/query, POST /v1/querybatch, GET /v1/search
//
// Requires running platform stack. Set GATEWAY_URL env var.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// gatewayURL returns the gateway service URL from env or default.
func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// TestOSVQueryHealth verifies gateway /health endpoint.
func TestOSVQueryHealth(t *testing.T) {
	resp, err := http.Get(gatewayURL() + "/health")
	if err != nil {
		t.Skipf("gateway not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health: got %d, want 200", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "ok" {
		t.Errorf("health status: got %v, want 'ok'", body["status"])
	}
}

// TestOSVQueryByID tests GET /v1/vulns/{id}.
func TestOSVQueryByID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This test uses CVE-2021-44228 (Log4Shell) — widely known CVE that should be in the DB
	// In test environment, this may return 404 or the actual CVE
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		gatewayURL()+"/v1/vulns/CVE-2021-44228", nil)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("gateway not running: %v", err)
	}
	defer resp.Body.Close()

	// Accept 200 (found) or 404 (not found — empty DB is OK for integration test)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /v1/vulns/CVE-2021-44228: got %d, want 200 or 404. Body: %s",
			resp.StatusCode, body)
	}
}

// TestOSVQueryByPackage tests POST /v1/query with package lookup.
func TestOSVQueryByPackage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"package": map[string]string{
			"ecosystem": "npm",
			"name":      "lodash",
		},
		"version": "4.17.20",
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		gatewayURL()+"/v1/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("gateway not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /v1/query: got %d, want 200. Body: %s", resp.StatusCode, respBody)
		return
	}

	// Validate response structure
	var result struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("POST /v1/query: failed to decode response: %v", err)
	}
	// result.Vulns may be empty in a test environment — that's OK
	t.Logf("POST /v1/query (npm/lodash@4.17.20): found %d vulns", len(result.Vulns))
}

// TestOSVQueryBatch tests POST /v1/querybatch.
func TestOSVQueryBatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"queries": []map[string]interface{}{
			{
				"package": map[string]string{"ecosystem": "npm", "name": "lodash"},
				"version": "4.17.20",
			},
			{
				"package": map[string]string{"ecosystem": "PyPI", "name": "requests"},
				"version": "2.27.0",
			},
		},
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		gatewayURL()+"/v1/querybatch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("gateway not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /v1/querybatch: got %d, want 200. Body: %s", resp.StatusCode, respBody)
		return
	}

	var result struct {
		Results []struct {
			Vulns []struct{ ID string `json:"id"` } `json:"vulns"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("POST /v1/querybatch: decode: %v", err)
		return
	}
	if len(result.Results) != 2 {
		t.Errorf("POST /v1/querybatch: got %d results, want 2", len(result.Results))
	}
}

// TestSitemapXML tests GET /sitemap.xml.
func TestSitemapXML(t *testing.T) {
	resp, err := http.Get(gatewayURL() + "/sitemap.xml")
	if err != nil {
		t.Skipf("gateway not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /sitemap.xml: got %d, want 200", resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || (!contains(contentType, "xml")) {
		t.Errorf("GET /sitemap.xml: Content-Type=%q, want XML", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	if !contains(string(body), "<urlset") {
		t.Errorf("GET /sitemap.xml: response missing <urlset> element. Body: %.200s", body)
	}
}

// TestSchemaValidation tests POST /admin/validate.
func TestSchemaValidation(t *testing.T) {
	dataURL := os.Getenv("DATA_SERVICE_URL")
	if dataURL == "" {
		dataURL = "http://localhost:8082"
	}

	validOSV := fmt.Sprintf(`{
		"id": "CVE-2021-44228",
		"published": "2021-12-10T00:00:00Z",
		"modified": "2021-12-11T00:00:00Z",
		"summary": "Log4Shell RCE",
		"details": "A critical RCE vulnerability in Log4j2.",
		"affected": [{"package": {"ecosystem": "Maven", "name": "org.apache.logging.log4j:log4j-core"}}]
	}`)

	resp, err := http.Post(dataURL+"/admin/validate",
		"application/json", bytes.NewBufferString(validOSV))
	if err != nil {
		t.Skipf("data-service not running: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if !result.Valid {
		t.Errorf("validateOSV: valid record rejected: %v", result.Errors)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
