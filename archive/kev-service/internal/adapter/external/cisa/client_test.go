package cisa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/globalcve/kev-service/internal/adapter/external/cisa"
)

func TestFetchKEVCatalog(t *testing.T) {
	payload := map[string]interface{}{
		"catalogVersion": "2024.01.01",
		"count":          2,
		"vulnerabilities": []map[string]string{
			{
				"cveID":             "CVE-2021-44228",
				"vendorProject":     "Apache",
				"product":           "Log4j2",
				"vulnerabilityName": "Apache Log4j2 Remote Code Execution Vulnerability",
				"dateAdded":         "2021-12-10",
				"dueDate":           "2021-12-24",
				"notes":             "Apply patches.",
			},
			{
				"cveID":             "CVE-2024-12345",
				"vendorProject":     "Example Corp",
				"product":           "Widget",
				"vulnerabilityName": "Example Vulnerability",
				"dateAdded":         "2024-01-15",
				"dueDate":           "2024-01-29",
				"notes":             "",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	}))
	defer srv.Close()

	client := cisa.NewClient(srv.URL)
	entries, err := client.FetchKEVCatalog(context.Background())
	if err != nil {
		t.Fatalf("FetchKEVCatalog error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.CVEID != "CVE-2021-44228" {
		t.Errorf("CVEID = %q", e.CVEID)
	}
	if e.VendorProject != "Apache" {
		t.Errorf("VendorProject = %q", e.VendorProject)
	}
	want := time.Date(2021, 12, 10, 0, 0, 0, 0, time.UTC)
	if !e.DateAdded.Equal(want) {
		t.Errorf("DateAdded = %v, want %v", e.DateAdded, want)
	}
}

func TestFetchKEVCatalog_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := cisa.NewClient(srv.URL)
	_, err := client.FetchKEVCatalog(context.Background())
	if err == nil {
		t.Error("expected error for non-OK status")
	}
}
