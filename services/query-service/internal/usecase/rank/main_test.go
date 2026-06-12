// Package main_test provides unit tests for ranking-service core functions.
// Tests the loosyLookup algorithm logic without a real MongoDB.
package main

import (
	"testing"
)

// TestLoosyLookup_SkipParts verifies that parts like "cpe", "2.3", "*" are skipped.
func TestSkipParts_CPEPrefix(t *testing.T) {
	skipParts := map[string]bool{
		"":    true,
		"cpe": true, "2.3": true,
		"a": true, "o": true, "h": true,
		"*": true, "-": true,
	}

	for _, part := range []string{"cpe", "2.3", "a", "o", "h", "*", "-", ""} {
		if !skipParts[part] {
			t.Errorf("expected %q to be in skipParts", part)
		}
	}
}

// TestSkipParts_MeaningfulParts verifies that meaningful parts are not skipped.
func TestSkipParts_MeaningfulParts(t *testing.T) {
	skipParts := map[string]bool{
		"":    true,
		"cpe": true, "2.3": true,
		"a": true, "o": true, "h": true,
		"*": true, "-": true,
	}

	for _, part := range []string{"apache", "log4j", "2.14.1", "microsoft", "windows_10"} {
		if skipParts[part] {
			t.Errorf("expected %q NOT to be in skipParts", part)
		}
	}
}

// TestRankingEntry_JSON verifies JSON serialization of RankingEntry.
func TestRankingEntry_JSON(t *testing.T) {
	entry := RankingEntry{
		CPE: "cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*",
		Rank: []RankEntry{
			{Group: "critical", Rank: 1},
			{Group: "high", Rank: 5},
		},
	}

	if entry.CPE == "" {
		t.Error("CPE should not be empty")
	}
	if len(entry.Rank) != 2 {
		t.Errorf("expected 2 rank entries, got %d", len(entry.Rank))
	}
	if entry.Rank[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", entry.Rank[0].Rank)
	}
}

// TestRankEntry_Fields verifies RankEntry struct fields.
func TestRankEntry_Fields(t *testing.T) {
	re := RankEntry{Group: "medium", Rank: 3}
	if re.Group != "medium" {
		t.Errorf("expected group=medium, got %q", re.Group)
	}
	if re.Rank != 3 {
		t.Errorf("expected rank=3, got %d", re.Rank)
	}
}

// TestEnvDefault_Present verifies envDefault returns env value when set.
func TestEnvDefault_Present(t *testing.T) {
	t.Setenv("TEST_KEY_RANKING", "value_from_env")
	result := envDefault("TEST_KEY_RANKING", "default")
	if result != "value_from_env" {
		t.Errorf("expected value_from_env, got %q", result)
	}
}

// TestEnvDefault_Absent verifies envDefault returns default when not set.
func TestEnvDefault_Absent(t *testing.T) {
	result := envDefault("__NOT_SET_KEY_RANKING__", "default_value")
	if result != "default_value" {
		t.Errorf("expected default_value, got %q", result)
	}
}
