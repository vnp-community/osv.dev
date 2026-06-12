// Package cwe — unit tests for the CWE database.
package cwe_test

import (
	"testing"

	"github.com/osv/shared/pkg/cwe"
)

func TestGet(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"CWE-79", "CWE-79"},
		{"79", "CWE-79"},
		{"cwe-400", "CWE-400"},
		{"400", "CWE-400"},
	}
	for _, tc := range cases {
		e := cwe.Get(tc.input)
		if e == nil {
			t.Errorf("Get(%q) = nil, want entry %s", tc.input, tc.want)
			continue
		}
		if e.ID != tc.want {
			t.Errorf("Get(%q).ID = %q, want %q", tc.input, e.ID, tc.want)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	e := cwe.Get("CWE-99999")
	if e != nil {
		t.Errorf("Get(unknown) = %v, want nil", e)
	}
}

func TestTags(t *testing.T) {
	// CWE-400 should include resource-exhaustion tag
	tags := cwe.Tags("CWE-400")
	if len(tags) == 0 {
		t.Error("Tags(CWE-400) returned empty, expected tags")
	}
	found := false
	for _, tag := range tags {
		if tag == "type:resource-exhaustion" || tag == "impact:dos" {
			found = true
		}
	}
	if !found {
		t.Errorf("Tags(CWE-400) = %v, expected type:resource-exhaustion or impact:dos", tags)
	}
}

func TestTagsNotFound(t *testing.T) {
	tags := cwe.Tags("CWE-99999")
	if len(tags) != 0 {
		t.Errorf("Tags(unknown) = %v, want nil", tags)
	}
}

func TestTagsForAll(t *testing.T) {
	ids := []string{"CWE-79", "CWE-400", "CWE-416"}
	tags := cwe.TagsForAll(ids)
	if len(tags) == 0 {
		t.Error("TagsForAll returned empty")
	}
	// Check for duplicates
	seen := make(map[string]bool)
	for _, tag := range tags {
		if seen[tag] {
			t.Errorf("TagsForAll returned duplicate tag: %q", tag)
		}
		seen[tag] = true
	}
}

func TestGetCategory(t *testing.T) {
	cases := []struct {
		id       string
		category cwe.Category
	}{
		{"CWE-400", cwe.CategoryResourceMgmt},
		{"CWE-79", cwe.CategoryInjection},
		{"CWE-416", cwe.CategoryMemory},
		{"CWE-287", cwe.CategoryAuthentication},
		{"CWE-326", cwe.CategoryCryptography},
	}
	for _, tc := range cases {
		got := cwe.GetCategory(tc.id)
		if got != tc.category {
			t.Errorf("GetCategory(%q) = %q, want %q", tc.id, got, tc.category)
		}
	}
}

func TestGetCategoryUnknown(t *testing.T) {
	got := cwe.GetCategory("CWE-99999")
	if got != cwe.CategoryOther {
		t.Errorf("GetCategory(unknown) = %q, want %q", got, cwe.CategoryOther)
	}
}

func TestIsKnown(t *testing.T) {
	if !cwe.IsKnown("CWE-79") {
		t.Error("IsKnown(CWE-79) = false, want true")
	}
	if cwe.IsKnown("CWE-99999") {
		t.Error("IsKnown(CWE-99999) = true, want false")
	}
}

func TestList(t *testing.T) {
	entries := cwe.List()
	if len(entries) == 0 {
		t.Error("List() returned empty")
	}
	// Verify all entries have IDs
	for _, e := range entries {
		if e.ID == "" {
			t.Error("Entry has empty ID")
		}
		if e.Name == "" {
			t.Errorf("Entry %s has empty Name", e.ID)
		}
	}
}

func TestByCategory(t *testing.T) {
	entries := cwe.ByCategory(cwe.CategoryMemory)
	if len(entries) == 0 {
		t.Error("ByCategory(Memory) returned empty")
	}
	for _, e := range entries {
		if e.Category != cwe.CategoryMemory {
			t.Errorf("ByCategory(Memory) returned entry with wrong category: %s", e.ID)
		}
	}
}

func TestXSSEntry(t *testing.T) {
	e := cwe.Get("CWE-79")
	if e == nil {
		t.Fatal("CWE-79 not found")
	}
	if !contains(e.Tags, "type:xss") {
		t.Errorf("CWE-79 tags = %v, expected type:xss", e.Tags)
	}
	if e.Category != cwe.CategoryInjection {
		t.Errorf("CWE-79 category = %q, expected Injection", e.Category)
	}
}

func TestSQLInjectionEntry(t *testing.T) {
	e := cwe.Get("CWE-89")
	if e == nil {
		t.Fatal("CWE-89 not found")
	}
	if !contains(e.Tags, "type:sql-injection") {
		t.Errorf("CWE-89 tags = %v, expected type:sql-injection", e.Tags)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
