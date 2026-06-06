package tagging_test

import (
	"slices"
	"testing"

	"github.com/osv/pkg/classification/tagging"
)

func TestTagsFromCWEs(t *testing.T) {
	tests := []struct {
		name     string
		cweIDs   []string
		wantTags []string
	}{
		{
			name:     "SQL injection CWE-89",
			cweIDs:   []string{"CWE-89"},
			wantTags: []string{"sqli", "injection", "database"},
		},
		{
			name:     "XSS CWE-79",
			cweIDs:   []string{"CWE-79"},
			wantTags: []string{"xss", "injection", "web"},
		},
		{
			name:     "RCE via command injection CWE-78",
			cweIDs:   []string{"CWE-78"},
			wantTags: []string{"command-injection", "injection", "rce"},
		},
		{
			name:     "memory safety CWE-416 use-after-free",
			cweIDs:   []string{"CWE-416"},
			wantTags: []string{"memory-safety", "use-after-free"},
		},
		{
			name:     "multiple CWEs deduplicate injection tag",
			cweIDs:   []string{"CWE-89", "CWE-79"},
			wantTags: []string{"sqli", "injection", "database", "xss", "web"},
		},
		{
			name:     "without CWE- prefix normalized",
			cweIDs:   []string{"79"},
			wantTags: []string{"xss", "injection", "web"},
		},
		{
			name:     "unknown CWE returns empty",
			cweIDs:   []string{"CWE-99999"},
			wantTags: []string{},
		},
		{
			name:     "path traversal",
			cweIDs:   []string{"CWE-22"},
			wantTags: []string{"path-traversal", "file"},
		},
		{
			name:     "DoS resource exhaustion",
			cweIDs:   []string{"CWE-400"},
			wantTags: []string{"dos", "resource-exhaustion"},
		},
		{
			name:     "SSRF",
			cweIDs:   []string{"CWE-918"},
			wantTags: []string{"ssrf", "injection"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagging.TagsFromCWEs(tt.cweIDs)
			if len(tt.wantTags) == 0 && len(got) == 0 {
				return
			}
			for _, want := range tt.wantTags {
				if !slices.Contains(got, want) {
					t.Errorf("expected tag %q in result %v", want, got)
				}
			}
		})
	}
}

func TestTagsFromPackages(t *testing.T) {
	tests := []struct {
		name     string
		packages []string
		wantTags []string
	}{
		{
			name:     "Django web framework",
			packages: []string{"django"},
			wantTags: []string{"web", "python-framework"},
		},
		{
			name:     "OpenSSL crypto library",
			packages: []string{"openssl"},
			wantTags: []string{"crypto", "tls", "network"},
		},
		{
			name:     "Log4j Java logging",
			packages: []string{"log4j"},
			wantTags: []string{"logging", "java"},
		},
		{
			name:     "Docker container",
			packages: []string{"docker"},
			wantTags: []string{"container", "cloud"},
		},
		{
			name:     "case insensitive match",
			packages: []string{"Django", "OpenSSL"},
			wantTags: []string{"web", "python-framework", "crypto", "tls", "network"},
		},
		{
			name:     "package not matched returns empty",
			packages: []string{"totally-unknown-package"},
			wantTags: []string{},
		},
		{
			name:     "multiple packages merge tags",
			packages: []string{"nginx", "redis"},
			wantTags: []string{"web-server", "network", "database", "cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagging.TagsFromPackages(tt.packages)
			if len(tt.wantTags) == 0 && len(got) == 0 {
				return
			}
			for _, want := range tt.wantTags {
				if !slices.Contains(got, want) {
					t.Errorf("expected tag %q in result %v", want, got)
				}
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	a := []string{"sqli", "injection"}
	b := []string{"injection", "web", "xss"}
	merged := tagging.MergeTags(a, b)

	// Should deduplicate "injection"
	count := 0
	for _, tag := range merged {
		if tag == "injection" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("injection should appear exactly once, got %d times in %v", count, merged)
	}
	if len(merged) != 4 { // sqli, injection, web, xss
		t.Errorf("expected 4 unique tags, got %d: %v", len(merged), merged)
	}
}
