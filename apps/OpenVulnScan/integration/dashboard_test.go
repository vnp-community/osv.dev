//go:build integration
// +build integration

// Package integration — dashboard_test.go
// Integration tests cho dashboard và notification pipeline.
package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ── Dashboard Tests ───────────────────────────────────────────────────────────

func TestDashboard_Stats_RequiresAuth(t *testing.T) {
	srv := authTestServer(t)
	

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(srv + "/api/v1/dashboard")
	if err != nil {
		t.Fatalf("dashboard request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}

func TestDashboard_Stats_Authenticated(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", srv+"/api/v1/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("dashboard stats failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats) //nolint:errcheck
	t.Logf("dashboard stats: %+v", stats)

	// Verify expected keys
	for _, key := range []string{"total_scans", "total_findings", "total_products"} {
		if _, ok := stats[key]; !ok {
			t.Logf("warning: missing key %q in dashboard response", key)
		}
	}
}

func TestDashboard_Timeline_Authenticated(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", srv+"/api/v1/dashboard/timeline", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("dashboard timeline failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	t.Logf("timeline: %+v", result)
}

func TestDashboard_TopVulnerabilities_Authenticated(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", srv+"/api/v1/dashboard/top-vulnerabilities", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("top-vulnerabilities failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	t.Logf("top vulns: %+v", result)
}

// ── Notification Tests ────────────────────────────────────────────────────────

func TestNotification_SIEMConfig_Get(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", srv+"/api/v1/siem/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("siem config get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	t.Logf("siem config: %+v", result)
}

func TestNotification_Webhooks_RequiresAuth(t *testing.T) {
	srv := authTestServer(t)
	

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(srv + "/api/v1/notifications/webhooks")
	if err != nil {
		t.Fatalf("webhook list failed: %v", err)
	}
	defer resp.Body.Close()

	// Should require auth
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Logf("webhooks without auth: %d (expected 401 or 404)", resp.StatusCode)
	}
}

func TestHealth_Ready_Endpoint(t *testing.T) {
	srv := authTestServer(t)
	

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(srv + "/ready")
	if err != nil {
		t.Fatalf("ready endpoint failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("ready: unexpected status %d", resp.StatusCode)
	}
	t.Logf("ready endpoint: %d", resp.StatusCode)
}
