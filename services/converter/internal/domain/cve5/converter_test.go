// Package cve5 — unit tests for CVE5 converter and ADP merging.
package cve5_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/osv/converter/internal/domain/cve5"
)

func TestConvertToOSV_Basic(t *testing.T) {
	record := &cve5.CVERecord{
		DataType:    "CVE_RECORD",
		DataVersion: "5.0",
		CVEMetadata: &cve5.CVEMetadata{
			CVEID:         "CVE-2023-44487",
			State:         "PUBLISHED",
			DatePublished: time.Date(2023, 10, 10, 0, 0, 0, 0, time.UTC),
			DateUpdated:   time.Date(2023, 10, 15, 0, 0, 0, 0, time.UTC),
		},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				Descriptions: []*cve5.Description{
					{Lang: "en", Value: "HTTP/2 Rapid Reset Attack vulnerability."},
				},
				Affected: []*cve5.AffectedEntry{
					{
						Vendor:  "golang",
						Product: "golang.org/x/net",
						Versions: []*cve5.Version{
							{Version: "0.0.0", Status: "affected", LessThan: "0.18.0", VersionType: "semver"},
						},
					},
				},
				References: []*cve5.Reference{
					{URL: "https://github.com/golang/go/issues/63417", Tags: []string{"issue-tracking"}},
					{URL: "https://github.com/golang/net/commit/abc123", Tags: []string{"patch"}},
				},
				Metrics: []*cve5.Metric{
					{
						Format: "CVSS",
						CVSSv31: &cve5.CVSSData{
							VectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H",
							BaseScore:    7.5,
							BaseSeverity: "HIGH",
						},
					},
				},
				ProblemTypes: []*cve5.ProblemType{
					{
						Descriptions: []*cve5.ProblemTypeDescription{
							{Lang: "en", CWEID: "CWE-400", Description: "Uncontrolled Resource Consumption"},
						},
					},
				},
			},
		},
	}

	vuln, err := cve5.ConvertToOSV(record)
	if err != nil {
		t.Fatalf("ConvertToOSV: %v", err)
	}

	if vuln.Id != "CVE-2023-44487" {
		t.Errorf("Id = %q, want %q", vuln.Id, "CVE-2023-44487")
	}
	if vuln.Summary == "" {
		t.Error("Summary is empty")
	}
	if vuln.Details == "" {
		t.Error("Details is empty")
	}
	if len(vuln.Affected) != 1 {
		t.Errorf("len(Affected) = %d, want 1", len(vuln.Affected))
	}
	if len(vuln.Severity) != 1 {
		t.Errorf("len(Severity) = %d, want 1", len(vuln.Severity))
	}
	if len(vuln.References) != 2 {
		t.Errorf("len(References) = %d, want 2", len(vuln.References))
	}
}

func TestConvertToOSV_NilRecord(t *testing.T) {
	_, err := cve5.ConvertToOSV(nil)
	if err == nil {
		t.Error("expected error for nil record")
	}
}

func TestConvertToOSV_MissingCNA(t *testing.T) {
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-0001"},
		Containers:  &cve5.Containers{},
	}
	_, err := cve5.ConvertToOSV(record)
	if err == nil {
		t.Error("expected error for missing CNA")
	}
}

func TestMergeADPContainers(t *testing.T) {
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-44487"},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				Descriptions: []*cve5.Description{{Lang: "en", Value: "Test CVE"}},
			},
			ADP: []*cve5.ADP{
				{
					ProviderMetadata: &cve5.ProviderMetadata{ShortName: "NVD-ADP"},
					Metrics: []*cve5.Metric{
						{
							CVSSv31: &cve5.CVSSData{
								VectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H",
								BaseScore:    7.5,
							},
						},
					},
					Affected: []*cve5.AffectedEntry{
						{
							Vendor:  "nginx",
							Product: "nginx",
							Versions: []*cve5.Version{
								{Version: "1.24.0", Status: "affected", LessThan: "1.25.3"},
							},
						},
					},
				},
			},
		},
	}

	vuln, err := cve5.ConvertToOSV(record)
	if err != nil {
		t.Fatalf("ConvertToOSV: %v", err)
	}

	// Merge ADP
	cve5.MergeADPContainers(vuln, record)

	// Should have CVSS severity from ADP
	if len(vuln.Severity) == 0 {
		t.Error("expected severity from ADP merge")
	}
	// Should have affected entry from ADP
	if len(vuln.Affected) != 1 {
		t.Errorf("len(Affected) = %d, want 1 (from ADP)", len(vuln.Affected))
	}
}

func TestMergeADPContainers_NoDuplicate(t *testing.T) {
	// Same product in CNA and ADP — should not duplicate
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-0001"},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				Descriptions: []*cve5.Description{{Lang: "en", Value: "Test"}},
				Affected: []*cve5.AffectedEntry{
					{Vendor: "test", Product: "myapp", Versions: []*cve5.Version{{Version: "1.0", Status: "affected"}}},
				},
			},
			ADP: []*cve5.ADP{
				{
					ProviderMetadata: &cve5.ProviderMetadata{ShortName: "NVD-ADP"},
					Affected: []*cve5.AffectedEntry{
						{Vendor: "test", Product: "myapp", Versions: []*cve5.Version{{Version: "1.0", Status: "affected"}}},
					},
				},
			},
		},
	}
	vuln, _ := cve5.ConvertToOSV(record)
	cve5.MergeADPContainers(vuln, record)

	if len(vuln.Affected) != 1 {
		t.Errorf("expected 1 affected (no duplicate), got %d", len(vuln.Affected))
	}
}

func TestExtractCWEsFromRecord(t *testing.T) {
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-44487"},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				ProblemTypes: []*cve5.ProblemType{
					{
						Descriptions: []*cve5.ProblemTypeDescription{
							{CWEID: "CWE-400"},
							{CWEID: "CWE-835"},
						},
					},
				},
			},
		},
	}
	ids := cve5.ExtractCWEsFromRecord(record)
	if len(ids) != 2 {
		t.Errorf("len(ids) = %d, want 2", len(ids))
	}
	if ids[0] != "CWE-400" {
		t.Errorf("ids[0] = %q, want CWE-400", ids[0])
	}
}

func TestExtractCWEsFromRecord_Dedup(t *testing.T) {
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-0001"},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				ProblemTypes: []*cve5.ProblemType{
					{Descriptions: []*cve5.ProblemTypeDescription{{CWEID: "CWE-400"}, {CWEID: "CWE-400"}}},
				},
			},
		},
	}
	ids := cve5.ExtractCWEsFromRecord(record)
	if len(ids) != 1 {
		t.Errorf("expected 1 deduplicated CWE, got %d", len(ids))
	}
}

func TestReferenceClassification(t *testing.T) {
	record := &cve5.CVERecord{
		CVEMetadata: &cve5.CVEMetadata{CVEID: "CVE-2023-0001"},
		Containers: &cve5.Containers{
			CNA: &cve5.CNA{
				Descriptions: []*cve5.Description{{Lang: "en", Value: "Test"}},
				References: []*cve5.Reference{
					{URL: "https://github.com/org/repo/commit/abc", Tags: []string{"patch"}},
					{URL: "https://github.com/org/repo/issues/1", Tags: []string{"issue-tracking"}},
					{URL: "https://vendor.com/advisory/123", Tags: []string{"vendor-advisory"}},
				},
			},
		},
	}
	vuln, err := cve5.ConvertToOSV(record)
	if err != nil {
		t.Fatal(err)
	}
	if len(vuln.References) != 3 {
		t.Errorf("expected 3 refs, got %d", len(vuln.References))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	// Verify ConvertToOSV can handle a record parsed from JSON
	jsonData := `{
		"dataType": "CVE_RECORD",
		"dataVersion": "5.0",
		"cveMetadata": {"cveId": "CVE-2023-44487", "cveState": "PUBLISHED"},
		"containers": {
			"cna": {
				"descriptions": [{"lang": "en", "value": "Test vulnerability"}],
				"affected": [],
				"references": [{"url": "https://example.com"}]
			}
		}
	}`
	var record cve5.CVERecord
	if err := json.Unmarshal([]byte(jsonData), &record); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	vuln, err := cve5.ConvertToOSV(&record)
	if err != nil {
		t.Fatalf("ConvertToOSV: %v", err)
	}
	if vuln.Id != "CVE-2023-44487" {
		t.Errorf("Id = %q, want CVE-2023-44487", vuln.Id)
	}
}
