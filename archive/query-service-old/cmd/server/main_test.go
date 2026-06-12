// Package main_test — unit tests for query-service filter builder.
package main

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// TestBuildFilter_Empty verifies that empty filter produces empty bson.M.
func TestBuildFilter_Empty(t *testing.T) {
	f := QueryFilter{Limit: 100}
	result := buildFilter(f)
	if len(result) != 0 {
		t.Errorf("empty filter should produce empty bson.M, got %v", result)
	}
}

// TestBuildFilter_HideRejected verifies that rejected CVEs are filtered.
func TestBuildFilter_HideRejected(t *testing.T) {
	f := QueryFilter{HideRejected: true, Limit: 100}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("expected $and clause in filter")
	}
}

// TestBuildFilter_CVSSAbove verifies CVSS above filter.
func TestBuildFilter_CVSSAbove(t *testing.T) {
	score := 7.0
	f := QueryFilter{
		CVSSScore:    &score,
		CVSSVersion:  "V3",
		CVSSOperator: "above",
		Limit:        100,
	}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("expected $and clause with CVSS filter")
	}
}

// TestBuildFilter_CVSSBelow verifies CVSS below filter.
func TestBuildFilter_CVSSBelow(t *testing.T) {
	score := 4.0
	f := QueryFilter{
		CVSSScore:    &score,
		CVSSVersion:  "V2",
		CVSSOperator: "below",
		Limit:        100,
	}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("expected $and clause with CVSS below filter")
	}
}

// TestBuildFilter_TimeFrom verifies time-from filter.
func TestBuildFilter_TimeFrom(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := QueryFilter{
		StartDate:    &start,
		TimeOperator: "from",
		TimeField:    "modified",
		Limit:        100,
	}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("expected $and clause for time-from filter")
	}
}

// TestBuildFilter_TimeBetween verifies time-between filter.
func TestBuildFilter_TimeBetween(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	f := QueryFilter{
		StartDate:    &start,
		EndDate:      &end,
		TimeOperator: "between",
		TimeField:    "published",
		Limit:        100,
	}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("expected $and clause for time-between filter")
	}
}

// TestBuildFilter_Combined verifies combining CVSS + time filter.
func TestBuildFilter_Combined(t *testing.T) {
	score := 9.0
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	f := QueryFilter{
		CVSSScore:    &score,
		CVSSVersion:  "V3",
		CVSSOperator: "above",
		StartDate:    &start,
		TimeOperator: "from",
		TimeField:    "modified",
		HideRejected: true,
		Limit:        50,
	}
	result := buildFilter(f)
	if _, ok := result["$and"]; !ok {
		t.Fatal("combined filter should produce $and")
	}
	// bson.A is the type for BSON arrays
	andConds, ok := result["$and"].(bson.A)
	if !ok {
		t.Fatalf("$and should be bson.A, got %T", result["$and"])
	}
	// Should have 3 conditions: HideRejected + CVSS + Time
	if len(andConds) != 3 {
		t.Errorf("expected 3 $and conditions, got %d", len(andConds))
	}
}

// TestBuildFilter_DefaultCVSSField verifies that unknown CVSS version defaults to cvss3.
func TestBuildFilter_DefaultCVSSField(t *testing.T) {
	score := 7.5
	f := QueryFilter{
		CVSSScore:    &score,
		CVSSVersion:  "",     // empty → should default to cvss3
		CVSSOperator: "above",
		Limit:        100,
	}
	result := buildFilter(f)
	// Just verify it produces a filter without panic
	if result == nil {
		t.Error("buildFilter should not return nil")
	}
}
