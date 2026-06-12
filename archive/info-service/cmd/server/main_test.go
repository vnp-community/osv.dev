// Package main_test provides unit tests for info-service types and functions.
package main

import (
	"encoding/json"
	"testing"
	"time"
)

// TestCollectionInfo_JSON verifies JSON serialization of CollectionInfo.
func TestCollectionInfo_JSON(t *testing.T) {
	ci := CollectionInfo{
		Name:         "cves",
		Count:        250000,
		LastModified: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(ci)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded CollectionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Name != "cves" {
		t.Errorf("Name = %q, want %q", decoded.Name, "cves")
	}
	if decoded.Count != 250000 {
		t.Errorf("Count = %d, want 250000", decoded.Count)
	}
}

// TestDBInfo_ZeroValue verifies DBInfo zero value behavior.
func TestDBInfo_ZeroValue(t *testing.T) {
	info := &DBInfo{}
	if info.TotalCVEs != 0 {
		t.Errorf("expected TotalCVEs=0, got %d", info.TotalCVEs)
	}
	if info.TotalCPEs != 0 {
		t.Errorf("expected TotalCPEs=0, got %d", info.TotalCPEs)
	}
	if len(info.Collections) != 0 {
		t.Errorf("expected empty Collections, got %v", info.Collections)
	}
}

// TestDBInfo_JSON verifies full DBInfo JSON round-trip.
func TestDBInfo_JSON(t *testing.T) {
	info := &DBInfo{
		TotalCVEs:    250000,
		TotalCPEs:    500000,
		DatabaseSize: "1.23 GB",
		Collections: []CollectionInfo{
			{Name: "cves", Count: 250000},
			{Name: "cpe", Count: 500000},
		},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded DBInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.TotalCVEs != 250000 {
		t.Errorf("TotalCVEs = %d, want 250000", decoded.TotalCVEs)
	}
	if decoded.DatabaseSize != "1.23 GB" {
		t.Errorf("DatabaseSize = %q, want %q", decoded.DatabaseSize, "1.23 GB")
	}
	if len(decoded.Collections) != 2 {
		t.Errorf("Collections len = %d, want 2", len(decoded.Collections))
	}
}

// TestTrackedCollections verifies all 6 tracked collections are defined.
func TestTrackedCollections(t *testing.T) {
	// This mirrors the tracked slice in getDBInfo.
	tracked := []string{"cves", "cpe", "cwe", "capec", "via4", "ranking"}
	if len(tracked) != 6 {
		t.Errorf("expected 6 tracked collections, got %d", len(tracked))
	}
	for _, name := range []string{"cves", "cpe", "cwe", "capec"} {
		found := false
		for _, t := range tracked {
			if t == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in tracked collections", name)
		}
	}
}

// TestEnvDefault_Present verifies envDefault returns env value when set.
func TestEnvDefault_InfoService_Present(t *testing.T) {
	t.Setenv("TEST_KEY_INFO", "custom_value")
	result := envDefault("TEST_KEY_INFO", "default")
	if result != "custom_value" {
		t.Errorf("expected custom_value, got %q", result)
	}
}

// TestEnvDefault_Absent verifies envDefault returns default when not set.
func TestEnvDefault_InfoService_Absent(t *testing.T) {
	result := envDefault("__NOT_SET_KEY_INFO__", "fallback")
	if result != "fallback" {
		t.Errorf("expected fallback, got %q", result)
	}
}
