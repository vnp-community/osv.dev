// reconcile/main.go — Compare record counts and spot-check data between Datastore and Firestore
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/firestore"
)

type Config struct {
	ProjectID          string
	OldSource          string  // "datastore"
	NewSource          string  // "firestore"
	ReportDiffThreshold float64 // e.g. 0.001 = 0.1%
	SampleSize         int
	OutputFile         string
}

type ReconcileReport struct {
	RunAt            time.Time `json:"run_at"`
	OldCount         int64     `json:"old_count"`
	NewCount         int64     `json:"new_count"`
	DiffCount        int64     `json:"diff_count"`
	DiffRate         float64   `json:"diff_rate"`
	SampleMismatches []string  `json:"sample_mismatches,omitempty"`
	Passed           bool      `json:"passed"`
	ThresholdPct     float64   `json:"threshold_pct"`
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.ProjectID, "project", os.Getenv("GCP_PROJECT"), "GCP project ID")
	flag.StringVar(&cfg.OldSource, "old-source", "datastore", "Old data source")
	flag.StringVar(&cfg.NewSource, "new-source", "firestore", "New data source")
	flag.Float64Var(&cfg.ReportDiffThreshold, "report-diff-threshold", 0.001, "Max acceptable diff rate (0.001 = 0.1%)")
	flag.IntVar(&cfg.SampleSize, "sample-size", 1000, "Number of random records to spot-check")
	flag.StringVar(&cfg.OutputFile, "output", "reconcile-report.json", "Output file for report")
	flag.Parse()

	ctx := context.Background()

	if cfg.ProjectID == "" {
		log.Fatal("--project or GCP_PROJECT env required")
	}

	fmt.Printf("=== OSV Data Reconciliation ===\n")
	fmt.Printf("Project: %s\n", cfg.ProjectID)
	fmt.Printf("Old: %s | New: %s\n\n", cfg.OldSource, cfg.NewSource)

	report := ReconcileReport{
		RunAt:        time.Now(),
		ThresholdPct: cfg.ReportDiffThreshold * 100,
	}

	// ── Count records in Datastore ──────────────────────────────────────────
	dsClient, err := datastore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Datastore client: %v", err)
	}
	defer dsClient.Close()

	dsQuery := datastore.NewQuery("Bug").KeysOnly()
	dsKeys, err := dsClient.GetAll(ctx, dsQuery, nil)
	if err != nil {
		log.Fatalf("Failed to query Datastore: %v", err)
	}
	report.OldCount = int64(len(dsKeys))
	fmt.Printf("Datastore Bug count: %d\n", report.OldCount)

	// ── Count records in Firestore ──────────────────────────────────────────
	fsClient, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer fsClient.Close()

	// Count via aggregation query
	fsCol := fsClient.Collection("vulnerabilities")
	aggQuery := fsCol.NewAggregationQuery().WithCount("count")
	results, err := aggQuery.Get(ctx)
	if err != nil {
		log.Fatalf("Failed to count Firestore records: %v", err)
	}
	countVal, ok := results["count"]
	if !ok {
		log.Fatal("No count in aggregation result")
	}
	report.NewCount = countVal.(*firestore.DocumentSnapshot).Data()["count"].(int64)
	fmt.Printf("Firestore vulnerabilities count: %d\n", report.NewCount)

	// ── Calculate diff ──────────────────────────────────────────────────────
	if report.OldCount > 0 {
		diff := report.OldCount - report.NewCount
		if diff < 0 {
			diff = -diff
		}
		report.DiffCount = diff
		report.DiffRate = float64(diff) / float64(report.OldCount)
	}

	fmt.Printf("\nDiff: %d records (%.4f%%)\n", report.DiffCount, report.DiffRate*100)

	// ── Random spot-check ───────────────────────────────────────────────────
	fmt.Printf("\nSpot-checking %d random records...\n", cfg.SampleSize)
	sampleSize := cfg.SampleSize
	if int(report.OldCount) < sampleSize {
		sampleSize = int(report.OldCount)
	}

	// Pick random keys
	indices := rand.Perm(len(dsKeys))
	if len(indices) > sampleSize {
		indices = indices[:sampleSize]
	}

	mismatches := []string{}
	for _, idx := range indices {
		key := dsKeys[idx]
		vulnID := key.Name

		// Check if exists in Firestore
		doc, err := fsClient.Collection("vulnerabilities").Doc(vulnID).Get(ctx)
		if err != nil || !doc.Exists() {
			mismatches = append(mismatches, vulnID)
		}
	}

	report.SampleMismatches = mismatches
	if len(mismatches) > 0 {
		fmt.Printf("⚠ %d sample records missing in Firestore\n", len(mismatches))
	} else {
		fmt.Printf("✓ All %d sample records found in Firestore\n", sampleSize)
	}

	// ── Pass/Fail ───────────────────────────────────────────────────────────
	report.Passed = report.DiffRate <= cfg.ReportDiffThreshold && len(mismatches) == 0

	// ── Write report ────────────────────────────────────────────────────────
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(cfg.OutputFile, data, 0644); err != nil {
		log.Printf("Failed to write report: %v", err)
	}

	fmt.Printf("\n=== Result: ")
	if report.Passed {
		fmt.Printf("PASS ✓ ===\n")
		fmt.Printf("Diff rate %.4f%% is within threshold %.4f%%\n",
			report.DiffRate*100, cfg.ReportDiffThreshold*100)
		os.Exit(0)
	} else {
		fmt.Printf("FAIL ✗ ===\n")
		fmt.Printf("Diff rate %.4f%% exceeds threshold %.4f%%\n",
			report.DiffRate*100, cfg.ReportDiffThreshold*100)
		os.Exit(1)
	}
}
