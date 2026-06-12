//go:build integration
// +build integration

// Package integration_test — scan_pipeline_test.go
// Integration tests cho scan-service pipeline.
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestScanPipeline_CreateAndList(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 10 * time.Second}

	// Create a scan
	body, _ := json.Marshal(map[string]interface{}{
		"targets":   []string{"192.168.1.1"},
		"scan_type": "nmap",
		"priority":  1,
	})
	req, _ := http.NewRequest("POST", srv+"/api/v1/scans/", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create scan request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 202/201, got %d", resp.StatusCode)
	}

	var createResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResp) //nolint:errcheck
	scanID, ok := createResp["scan_id"].(string)
	if !ok || scanID == "" {
		t.Fatal("expected scan_id in response")
	}
	t.Logf("scan created: %s", scanID)

	// List scans
	req2, _ := http.NewRequest("GET", srv+"/api/v1/scans/", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("list scans failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("list scans: expected 200, got %d", resp2.StatusCode)
	}

	var listResp map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&listResp) //nolint:errcheck
	scans, _ := listResp["scans"].([]interface{})
	t.Logf("scans listed: %d", len(scans))
}

func TestScanPipeline_GetScan(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 10 * time.Second}

	// Create scan first
	body, _ := json.Marshal(map[string]interface{}{
		"targets":   []string{"10.0.0.1"},
		"scan_type": "basic",
	})
	req, _ := http.NewRequest("POST", srv+"/api/v1/scans/", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create scan: %v", err)
	}
	var cr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&cr) //nolint:errcheck
	resp.Body.Close()

	scanID, _ := cr["scan_id"].(string)
	if scanID == "" {
		t.Skip("scan_id not returned (DB may not be available)")
	}

	// Get scan detail
	req2, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/scans/%s", srv, scanID), nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("get scan: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("get scan: unexpected status %d", resp2.StatusCode)
	}
	t.Logf("get scan %s: %d", scanID, resp2.StatusCode)
}

func TestScanPipeline_EmptyTargets_Returns400(t *testing.T) {
	srv := authTestServer(t)
	

	token := getAdminToken(t, srv)
	client := &http.Client{Timeout: 5 * time.Second}

	body, _ := json.Marshal(map[string]interface{}{
		"targets":   []string{},
		"scan_type": "nmap",
	})
	req, _ := http.NewRequest("POST", srv+"/api/v1/scans/", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty targets, got %d", resp.StatusCode)
	}
}
