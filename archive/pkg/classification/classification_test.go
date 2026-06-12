// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package classification

import (
	"testing"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCVSSTier(t *testing.T) {
	tests := []struct {
		score float64
		want  SeverityTier
	}{
		{10.0, TierCritical},
		{9.0, TierCritical},
		{8.9, TierHigh},
		{7.0, TierHigh},
		{6.9, TierMedium},
		{4.0, TierMedium},
		{3.9, TierLow},
		{0.1, TierLow},
		{0.0, TierUnknown},
	}
	for _, tt := range tests {
		if got := CVSSTier(tt.score); got != tt.want {
			t.Errorf("CVSSTier(%.1f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestTagsFromCVSSVector(t *testing.T) {
	tests := []struct {
		name   string
		vector string
		want   []Tag
	}{
		{
			name:   "network_rce",
			vector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want:   []Tag{TagNetworkBased, TagRCE},
		},
		{
			name:   "network_dos",
			vector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H",
			want:   []Tag{TagNetworkBased, TagDOS},
		},
		{
			name:   "local_info_disc",
			vector: "CVSS:3.1/AV:L/AC:L/PR:L/UI:N/S:U/C:H/I:N/A:N",
			want:   []Tag{TagLocalAccess, TagInfoDisc},
		},
		{
			name:   "empty_vector",
			vector: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TagsFromCVSSVector(tt.vector)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i, tag := range got {
				if tag != tt.want[i] {
					t.Errorf("tag[%d]: got %s, want %s", i, tag, tt.want[i])
				}
			}
		})
	}
}

func TestTagsFromDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantTag     Tag
	}{
		{"sql_injection", "This allows SQL injection via the username parameter.", TagSQLi},
		{"xss", "A Cross-site scripting vulnerability exists in the search box.", TagXSS},
		{"ssrf", "The application is vulnerable to Server-Side Request Forgery.", TagSSRF},
		{"buffer_overflow", "A buffer overflow in the parsing function.", TagMemorySafety},
		{"rce", "Allows remote code execution via crafted packets.", TagRCE},
		{"dos", "Triggers denial of service through resource exhaustion.", TagDOS},
		{"empty", "", Tag("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := TagsFromDescription(tt.description)
			if tt.wantTag == "" {
				if len(tags) != 0 {
					t.Errorf("expected no tags, got %v", tags)
				}
				return
			}
			found := false
			for _, tag := range tags {
				if tag == tt.wantTag {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected tag %s in %v", tt.wantTag, tags)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	t.Run("nil_vuln", func(t *testing.T) {
		r := Classify(nil)
		if r.SeverityTier != TierUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.SeverityTier)
		}
	})

	t.Run("with_fix", func(t *testing.T) {
		v := &osvschema.Vulnerability{
			Details: "SQL injection vulnerability.",
			Affected: []*osvschema.Affected{
				{
					Ranges: []*osvschema.Range{
						{
							Type: osvschema.Range_SEMVER,
							Events: []*osvschema.Event{
								{Introduced: "0"},
								{Fixed: "1.2.3"},
							},
						},
					},
				},
			},
		}
		r := Classify(v)
		if !r.HasFix {
			t.Error("expected HasFix=true")
		}
		foundFix := false
		for _, tag := range r.Tags {
			if tag == TagHasFix {
				foundFix = true
			}
		}
		if !foundFix {
			t.Errorf("expected %s tag in %v", TagHasFix, r.Tags)
		}
	})

	t.Run("withdrawn", func(t *testing.T) {
		v := &osvschema.Vulnerability{
			Withdrawn: timestamppb.Now(),
		}
		r := Classify(v)
		if !r.IsWithdrawn {
			t.Error("expected IsWithdrawn=true")
		}
	})
}
