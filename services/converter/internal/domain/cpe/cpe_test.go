package cpe_test

import (
	"testing"

	"github.com/osv/converter/internal/domain/cpe"
)

func TestParseCPE(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantVendor string
		wantProd   string
		wantVer    string
		wantPart   string
	}{
		{
			name:       "valid application CPE",
			input:      "cpe:2.3:a:apache:log4j:2.14.0:*:*:*:*:*:*:*",
			wantPart:   "a",
			wantVendor: "apache",
			wantProd:   "log4j",
			wantVer:    "2.14.0",
		},
		{
			name:       "valid OS CPE",
			input:      "cpe:2.3:o:linux:linux_kernel:5.10.0:*:*:*:*:*:*:*",
			wantPart:   "o",
			wantVendor: "linux",
			wantProd:   "linux_kernel",
			wantVer:    "5.10.0",
		},
		{
			name:       "CPE with quoted vendor",
			input:      "cpe:2.3:a:open-ssl:openssl:1.0.2:*:*:*:*:*:*:*",
			wantPart:   "a",
			wantVendor: "open-ssl",
			wantProd:   "openssl",
			wantVer:    "1.0.2",
		},
		{
			name:    "missing cpe prefix",
			input:   "2.3:a:apache:log4j:2.14.0:*:*:*:*:*:*:*",
			wantErr: true,
		},
		{
			name:    "too few components",
			input:   "cpe:2.3:a:apache:log4j",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cpe.ParseCPE(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCPE(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCPE(%q) unexpected error: %v", tt.input, err)
			}
			if got.Part != tt.wantPart {
				t.Errorf("Part: got %q, want %q", got.Part, tt.wantPart)
			}
			if got.Vendor != tt.wantVendor {
				t.Errorf("Vendor: got %q, want %q", got.Vendor, tt.wantVendor)
			}
			if got.Product != tt.wantProd {
				t.Errorf("Product: got %q, want %q", got.Product, tt.wantProd)
			}
			if got.Version != tt.wantVer {
				t.Errorf("Version: got %q, want %q", got.Version, tt.wantVer)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestExtractVersionRangesFromCPEs(t *testing.T) {
	tests := []struct {
		name          string
		configs       []cpe.Configuration
		validVersions []string
		wantCount     int
		wantIntro     string
		wantFixed     string
		wantSource    cpe.VersionSource
	}{
		{
			name: "versionStartIncluding + versionEndExcluding",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:              "cpe:2.3:a:apache:log4j:*:*:*:*:*:*:*:*",
									Vulnerable:            true,
									VersionStartIncluding: ptr("2.0.0"),
									VersionEndExcluding:   ptr("2.17.1"),
								},
							},
						},
					},
				},
			},
			wantCount:  1,
			wantIntro:  "2.0.0",
			wantFixed:  "2.17.1",
			wantSource: cpe.VersionSourceCPERange,
		},
		{
			name: "versionEndIncluding → infer lastAffected when no valid versions",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:            "cpe:2.3:a:vendor:product:*:*:*:*:*:*:*:*",
									Vulnerable:          true,
									VersionEndIncluding: ptr("1.2.3"),
								},
							},
						},
					},
				},
			},
			wantCount:  1,
			wantIntro:  "0",
			wantFixed:  "",
			wantSource: cpe.VersionSourceCPERange,
		},
		{
			name: "CPE version field as lastAffected",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:   "cpe:2.3:a:vendor:product:3.0.1:*:*:*:*:*:*:*",
									Vulnerable: true,
								},
							},
						},
					},
				},
			},
			wantCount:  1,
			wantIntro:  "0",
			wantFixed:  "",
			wantSource: cpe.VersionSourceCPEString,
		},
		{
			name: "non-vulnerable CPE skipped",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:              "cpe:2.3:a:vendor:product:*:*:*:*:*:*:*:*",
									Vulnerable:            false,
									VersionStartIncluding: ptr("1.0.0"),
									VersionEndExcluding:   ptr("1.5.0"),
								},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "AND operator node skipped",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "AND",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:   "cpe:2.3:a:vendor:product:1.0.0:*:*:*:*:*:*:*",
									Vulnerable: true,
								},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "CPE with ANY version skipped",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:   "cpe:2.3:a:vendor:product:*:*:*:*:*:*:*:*",
									Vulnerable: true,
								},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "versionStartExcluding derives introduced via nextVersion",
			configs: []cpe.Configuration{
				{
					Nodes: []cpe.ConfigNode{
						{
							Operator: "OR",
							CPEMatch: []cpe.CPEMatch{
								{
									Criteria:              "cpe:2.3:a:vendor:product:*:*:*:*:*:*:*:*",
									Vulnerable:            true,
									VersionStartExcluding: ptr("1.0.0"),
									VersionEndExcluding:   ptr("2.0.0"),
								},
							},
						},
					},
				},
			},
			validVersions: []string{"1.0.0", "1.1.0", "2.0.0", "2.1.0"},
			wantCount:     1,
			wantIntro:     "1.1.0",
			wantFixed:     "2.0.0",
			wantSource:    cpe.VersionSourceCPERange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, _ := cpe.ExtractVersionRangesFromCPEs(tt.configs, tt.validVersions)
			if len(ranges) != tt.wantCount {
				t.Fatalf("got %d ranges, want %d; ranges=%+v", len(ranges), tt.wantCount, ranges)
			}
			if tt.wantCount == 0 {
				return
			}
			vr := ranges[0]
			if vr.Introduced != tt.wantIntro {
				t.Errorf("Introduced: got %q, want %q", vr.Introduced, tt.wantIntro)
			}
			if vr.Fixed != tt.wantFixed {
				t.Errorf("Fixed: got %q, want %q", vr.Fixed, tt.wantFixed)
			}
			if vr.Source != tt.wantSource {
				t.Errorf("Source: got %v, want %v", vr.Source, tt.wantSource)
			}
		})
	}
}

func TestIsDenied(t *testing.T) {
	if !cpe.IsDenied("netapp", "anything") {
		t.Error("netapp should be denied for any product")
	}
	if !cpe.IsDenied("linux", "linux_kernel") {
		t.Error("linux:linux_kernel should be denied")
	}
	if cpe.IsDenied("linux", "other_product") {
		t.Error("linux:other_product should NOT be denied")
	}
	if cpe.IsDenied("apache", "log4j") {
		t.Error("apache:log4j should NOT be denied")
	}
}

func TestDeduplicateVersionRanges(t *testing.T) {
	ranges := []cpe.VersionRange{
		{Introduced: "1.0", Fixed: "2.0", CPECriteria: "cpe:2.3:a:v:p:*:*:*:*:*:*:*:*"},
		{Introduced: "1.0", Fixed: "2.0", CPECriteria: "cpe:2.3:a:v:p:*:*:*:*:*:*:*:*"},
		{Introduced: "2.0", Fixed: "3.0", CPECriteria: "cpe:2.3:a:v:p:*:*:*:*:*:*:*:*"},
	}
	got := cpe.DeduplicateVersionRanges(ranges)
	if len(got) != 2 {
		t.Errorf("DeduplicateVersionRanges: got %d, want 2", len(got))
	}
}
