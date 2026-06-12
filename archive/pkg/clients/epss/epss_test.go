// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package epss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeServer(t *testing.T, scores []*Score) *httptest.Server {
	t.Helper()
	resp := apiResponse{
		Status:     "OK",
		StatusCode: 200,
		Total:      len(scores),
		Limit:      100,
		Data:       scores,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
}

func TestGet(t *testing.T) {
	srv := makeServer(t, []*Score{
		{CVEID: "CVE-2023-44487", EPSS: 0.9734, Percentile: 0.9998, Date: "2026-06-03"},
	})
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))
	score, err := client.Get(context.Background(), "CVE-2023-44487")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if score == nil {
		t.Fatal("expected score, got nil")
	}
	if score.EPSS != 0.9734 {
		t.Errorf("expected EPSS=0.9734, got %f", score.EPSS)
	}
	if score.Percentile != 0.9998 {
		t.Errorf("expected Percentile=0.9998, got %f", score.Percentile)
	}
}

func TestGet_NotFound(t *testing.T) {
	srv := makeServer(t, []*Score{})
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))
	score, err := client.Get(context.Background(), "CVE-9999-99999")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if score != nil {
		t.Error("expected nil for unknown CVE")
	}
}

func TestGetBatch(t *testing.T) {
	srv := makeServer(t, []*Score{
		{CVEID: "CVE-2023-44487", EPSS: 0.97, Percentile: 0.999, Date: "2026-06-03"},
		{CVEID: "CVE-2021-44228", EPSS: 0.97, Percentile: 0.999, Date: "2026-06-03"},
	})
	defer srv.Close()

	client := NewClient(WithBaseURL(srv.URL))
	scores, err := client.GetBatch(context.Background(), []string{"CVE-2023-44487", "CVE-2021-44228"})
	if err != nil {
		t.Fatalf("GetBatch: %v", err)
	}
	if len(scores) != 2 {
		t.Errorf("expected 2 scores, got %d", len(scores))
	}
}

func TestScore_Tier(t *testing.T) {
	tests := []struct {
		percentile float64
		want       SeverityTier
	}{
		{0.999, TierCritical},
		{0.95, TierCritical},
		{0.90, TierHigh},
		{0.75, TierHigh},
		{0.60, TierMedium},
		{0.50, TierMedium},
		{0.30, TierLow},
		{0.01, TierLow},
	}
	for _, tt := range tests {
		s := &Score{Percentile: tt.percentile}
		if got := s.Tier(); got != tt.want {
			t.Errorf("Percentile=%.3f: got %s, want %s", tt.percentile, got, tt.want)
		}
	}
}
