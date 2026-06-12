package dataquality_test

import (
	"context"
	"testing"
	"time"

	"github.com/osv/admin/internal/application/dataquality"
)

func TestMonitor(t *testing.T) {
	ctx := context.Background()
	monitor := dataquality.NewMonitor()

	t.Run("missing CVSS flagged", func(t *testing.T) {
		rec := dataquality.CVERecord{
			ID: "CVE-2024-0001", SourceID: "nvd",
			HasCVSS: false, Description: "A long enough description here",
			AffectedCount: 1, LastModified: time.Now(),
			CWEIDs: []string{"CWE-79"},
		}
		findings := monitor.Check(ctx, rec)
		hasMissingCVSS := false
		for _, f := range findings {
			if f.CheckType == dataquality.CheckMissingCVSS {
				hasMissingCVSS = true
			}
		}
		if !hasMissingCVSS {
			t.Error("expected CheckMissingCVSS finding")
		}
	})

	t.Run("missing description flagged", func(t *testing.T) {
		rec := dataquality.CVERecord{
			ID: "CVE-2024-0002", SourceID: "nvd",
			HasCVSS: true, Description: "Short", AffectedCount: 1,
			LastModified: time.Now(), CWEIDs: []string{"CWE-89"},
		}
		findings := monitor.Check(ctx, rec)
		hasDesc := false
		for _, f := range findings {
			if f.CheckType == dataquality.CheckMissingDescription {
				hasDesc = true
			}
		}
		if !hasDesc {
			t.Error("expected CheckMissingDescription finding")
		}
	})

	t.Run("missing affected is critical", func(t *testing.T) {
		rec := dataquality.CVERecord{
			ID: "CVE-2024-0003", SourceID: "nvd",
			HasCVSS: true, Description: "A sufficiently long description about the vulnerability",
			AffectedCount: 0, LastModified: time.Now(), CWEIDs: []string{"CWE-22"},
		}
		findings := monitor.Check(ctx, rec)
		hasCritical := false
		for _, f := range findings {
			if f.CheckType == dataquality.CheckMissingAffected && f.Severity == dataquality.SeverityCritical {
				hasCritical = true
			}
		}
		if !hasCritical {
			t.Error("expected critical CheckMissingAffected finding")
		}
	})

	t.Run("stale CVE flagged", func(t *testing.T) {
		rec := dataquality.CVERecord{
			ID: "CVE-2020-0001", SourceID: "nvd",
			HasCVSS: true, Description: "A sufficiently long description about the vulnerability",
			AffectedCount: 2, LastModified: time.Now().Add(-400 * 24 * time.Hour),
			CWEIDs: []string{"CWE-79"},
		}
		findings := monitor.Check(ctx, rec)
		hasStale := false
		for _, f := range findings {
			if f.CheckType == dataquality.CheckStaleCVE {
				hasStale = true
			}
		}
		if !hasStale {
			t.Error("expected CheckStaleCVE finding")
		}
	})

	t.Run("good record has no critical findings", func(t *testing.T) {
		rec := dataquality.CVERecord{
			ID: "CVE-2024-9999", SourceID: "nvd",
			HasCVSS: true, Description: "A sufficiently long description about the vulnerability",
			AffectedCount: 2, LastModified: time.Now(), CWEIDs: []string{"CWE-79"},
		}
		findings := monitor.Check(ctx, rec)
		for _, f := range findings {
			if f.Severity == dataquality.SeverityCritical {
				t.Errorf("unexpected critical finding: %+v", f)
			}
		}
	})

	t.Run("resolve finding", func(t *testing.T) {
		monitor2 := dataquality.NewMonitor()
		rec := dataquality.CVERecord{
			ID: "CVE-2024-R001", SourceID: "nvd",
			HasCVSS: false, Description: "A long enough description",
			AffectedCount: 1, LastModified: time.Now(), CWEIDs: []string{"CWE-79"},
		}
		findings := monitor2.Check(ctx, rec)
		if len(findings) == 0 {
			t.Fatal("expected at least one finding")
		}

		err := monitor2.ResolveFinding(ctx, findings[0].ID)
		if err != nil {
			t.Fatalf("ResolveFinding: %v", err)
		}

		// Check it's resolved
		all := monitor2.ListFindings(ctx, "", true)
		found := false
		for _, f := range all {
			if f.ID == findings[0].ID && f.Resolved {
				found = true
			}
		}
		if !found {
			t.Error("finding should be marked resolved")
		}
	})

	t.Run("summary statistics", func(t *testing.T) {
		monitor3 := dataquality.NewMonitor()
		// Add a record with critical finding (missing affected)
		rec := dataquality.CVERecord{
			ID: "CVE-2024-S001", SourceID: "nvd",
			HasCVSS: false, Description: "A long enough description",
			AffectedCount: 0, LastModified: time.Now(),
		}
		monitor3.Check(ctx, rec)

		summary := monitor3.Summary(ctx)
		if summary.Open == 0 {
			t.Error("expected > 0 open findings")
		}
		if summary.Critical == 0 {
			t.Error("expected > 0 critical findings")
		}
	})
}
