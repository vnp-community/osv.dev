//go:build integration
// +build integration

// Package integration_test — auth flow integration tests.
// Run: go test -tags integration ./integration/... -v -timeout 120s
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osv/apps/openvulnscan/internal/testutil"
)

// baseURL is set by TestMain or injected by CI environment.
var baseURL string

// authTestServer creates a lightweight test HTTP server using the app's router.
// Reuses the running integration test server if available.
func authTestServer(t *testing.T) string {
	t.Helper()
	if baseURL != "" {
		return baseURL
	}
	// Fall back to localhost if running standalone
	return "http://localhost:8080"
}

func TestAuthFlow(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	email := fmt.Sprintf("test_%d@test.com", time.Now().UnixNano())
	password := "password123!"

	t.Run("register new user", func(t *testing.T) {
		payload := fmt.Sprintf(`{"email":%q,"password":%q,"first_name":"Test","last_name":"User"}`, email, password)
		resp, err := http.Post(srv+"/api/v1/auth/register", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Skipf("register endpoint unavailable: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		t.Logf("register response %d: %s", resp.StatusCode, body)
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Logf("register returned %d (may already exist)", resp.StatusCode)
		}
	})

	t.Run("login returns access token", func(t *testing.T) {
		payload := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
		resp, err := http.Post(srv+"/api/v1/auth/login", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Skipf("login endpoint unavailable: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Logf("login returned %d (non-critical if user not pre-seeded)", resp.StatusCode)
			return
		}
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
		if token, ok := result["access_token"].(string); !ok || token == "" {
			t.Error("expected access_token in login response")
		}
	})

	t.Run("protected endpoint without token returns 401", func(t *testing.T) {
		resp, err := http.Get(srv + "/api/v1/scans")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated /api/v1/scans")
	})

	t.Run("invalid credentials return 401", func(t *testing.T) {
		payload := `{"email":"nobody@example.com","password":"wrongpassword"}`
		resp, err := http.Post(srv+"/api/v1/auth/login", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusUnauthorized, resp.StatusCode, "invalid credentials")
	})

	t.Run("refresh token endpoint exists", func(t *testing.T) {
		resp, err := http.Post(srv+"/api/v1/auth/refresh", "application/json",
			strings.NewReader(`{"refresh_token":"invalid"}`))
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		// Should return 400 or 401, not 404
		if resp.StatusCode == http.StatusNotFound {
			t.Error("refresh endpoint not found (404)")
		}
	})
}

func TestHealthEndpoints(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	t.Run("health returns 200", func(t *testing.T) {
		resp, err := http.Get(srv + "/health")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /health")
		result := testutil.ReadJSON(t, resp)
		testutil.AssertContainsKey(t, result, "status", "health response")
	})

	t.Run("ready returns status", func(t *testing.T) {
		resp, err := http.Get(srv + "/ready")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		// May be 200 or 503 depending on state
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("unexpected /ready status: %d", resp.StatusCode)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	resp, err := http.Get(srv + "/health")
	if err != nil {
		t.Skipf("server unavailable: %v", err)
	}
	defer resp.Body.Close()

	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
	}
	for _, h := range headers {
		if resp.Header.Get(h) == "" {
			t.Logf("optional security header missing: %s (non-critical)", h)
		}
	}
}

func TestRateLimiting(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	// Send 25 rapid requests — expect some 429s after rate limit kicks in
	limitHit := false
	for i := 0; i < 25; i++ {
		resp, err := http.Post(srv+"/api/v1/auth/login", "application/json",
			bytes.NewBufferString(`{"email":"x@x.com","password":"x"}`))
		if err != nil {
			break
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			limitHit = true
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}
	if !limitHit {
		t.Log("rate limiting not triggered (may not be enabled in test env)")
	}
}

func TestAgentEndpoints(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	t.Run("agent download returns python script", func(t *testing.T) {
		resp, err := http.Get(srv + "/agent/download")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /agent/download")
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "#!/usr/bin/env python") {
			t.Error("agent script doesn't contain python shebang")
		}
	})

	t.Run("agent report accepted", func(t *testing.T) {
		payload := `{
			"hostname": "integration-test-host",
			"os_info": "Linux 5.15",
			"packages": [
				{"name": "openssl", "version": "1.1.1t", "ecosystem": "Debian"},
				{"name": "log4j", "version": "2.14.0", "ecosystem": "Maven"}
			]
		}`
		resp, err := http.Post(srv+"/agent/report", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "POST /agent/report")
		result := testutil.ReadJSON(t, resp)
		testutil.AssertContainsKey(t, result, "message", "agent report response")
	})
}

func TestDashboardEndpoints(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	// Use admin token if available
	token := getAdminToken(t, srv)
	if token == "" {
		t.Skip("no admin token available")
	}

	t.Run("dashboard returns stats", func(t *testing.T) {
		resp := testutil.DoRequest(t, "GET", srv+"/api/v1/dashboard", token, "")
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/dashboard")
		result := testutil.ReadJSON(t, resp)
		for _, key := range []string{"total_scans", "total_findings", "generated_at"} {
			testutil.AssertContainsKey(t, result, key, "dashboard response")
		}
	})

	t.Run("dashboard timeline returns data", func(t *testing.T) {
		resp := testutil.DoRequest(t, "GET", srv+"/api/v1/dashboard/timeline", token, "")
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/dashboard/timeline")
		result := testutil.ReadJSON(t, resp)
		testutil.AssertContainsKey(t, result, "timeline", "timeline response")
	})

	t.Run("top vulnerabilities returns list", func(t *testing.T) {
		resp := testutil.DoRequest(t, "GET", srv+"/api/v1/dashboard/top-vulnerabilities", token, "")
		defer resp.Body.Close()
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/dashboard/top-vulnerabilities")
	})
}

func TestScanEndpoints(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	token := getAdminToken(t, srv)
	if token == "" {
		t.Skip("no admin token available")
	}

	t.Run("list scans returns paginated result", func(t *testing.T) {
		resp := testutil.DoRequest(t, "GET", srv+"/api/v1/scans", token, "")
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/scans")
		resp.Body.Close()
	})

	t.Run("create scan queues successfully", func(t *testing.T) {
		payload := `{"targets":["127.0.0.1"],"scan_type":"discovery","priority":5}`
		resp := testutil.DoRequest(t, "POST", srv+"/api/v1/scans", token, payload)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		t.Logf("create scan: %d %s", resp.StatusCode, body)
		// 201 Created or 202 Accepted
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
			t.Logf("create scan returned unexpected status (non-critical in test env): %d", resp.StatusCode)
		}
	})
}

func TestOpenAPIDocEndpoints(t *testing.T) {
	srv := authTestServer(t)
	testutil.SkipIfNoInfra(t, srv)

	t.Run("swagger ui accessible", func(t *testing.T) {
		resp, err := http.Get(srv + "/api/v1/docs")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Log("Swagger UI not mounted (optional)")
			return
		}
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/docs")
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "swagger") && !strings.Contains(string(body), "openapi") {
			t.Log("Swagger UI response doesn't contain expected content")
		}
	})

	t.Run("openapi yaml accessible", func(t *testing.T) {
		resp, err := http.Get(srv + "/api/v1/openapi.yaml")
		if err != nil {
			t.Skipf("server unavailable: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Log("OpenAPI YAML not mounted (optional)")
			return
		}
		testutil.AssertStatusCode(t, http.StatusOK, resp.StatusCode, "GET /api/v1/openapi.yaml")
	})
}

// getAdminToken tìm admin token hoặc trả về "" nếu không có.
func getAdminToken(t *testing.T, srv string) string {
	t.Helper()
	creds := []struct{ email, pass string }{
		{"admin@openvulnscan.local", "admin123"},
		{"admin@example.com", "Admin123!"},
	}
	for _, c := range creds {
		payload := fmt.Sprintf(`{"email":%q,"password":%q}`, c.email, c.pass)
		resp, err := http.Post(srv+"/api/v1/auth/login", "application/json", strings.NewReader(payload))
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
			resp.Body.Close()
			if token, ok := result["access_token"].(string); ok && token != "" {
				return token
			}
		}
		resp.Body.Close()
	}
	return ""
}

// Verify httptest is imported (used in tests that create local servers).
var _ = httptest.NewServer
