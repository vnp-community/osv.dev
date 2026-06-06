// compare-responses/main.go — Sample and compare API responses between legacy and new service
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	LegacyURL    string
	NewURL       string
	SampleRate   float64
	Duration     time.Duration
	OutputFile   string
	Concurrency  int
	VulnListFile string
}

type ComparisonResult struct {
	VulnID      string        `json:"vuln_id"`
	Match       bool          `json:"match"`
	LegacyMS    int64         `json:"legacy_ms"`
	NewMS       int64         `json:"new_ms"`
	Diffs       []string      `json:"diffs,omitempty"`
}

type Report struct {
	RunAt        time.Time          `json:"run_at"`
	Duration     string             `json:"duration"`
	TotalSampled int64              `json:"total_sampled"`
	TotalDiffs   int64              `json:"total_diffs"`
	DiffRate     float64            `json:"diff_rate"`
	AvgLegacyMS  float64            `json:"avg_legacy_ms"`
	AvgNewMS     float64            `json:"avg_new_ms"`
	Samples      []ComparisonResult `json:"samples,omitempty"`
	Passed       bool               `json:"passed"`
}

func fetchVuln(baseURL, vulnID string) (map[string]interface{}, int64, error) {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/vulns/%s", strings.TrimRight(baseURL, "/"), vulnID)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	elapsed := time.Since(start).Milliseconds()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, elapsed, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, elapsed, err
	}
	return data, elapsed, nil
}

func compareResponses(legacy, newResp map[string]interface{}) []string {
	diffs := []string{}

	// Critical fields that must EXACTLY match
	exactFields := []string{"id", "modified", "published"}
	for _, field := range exactFields {
		lv := fmt.Sprintf("%v", legacy[field])
		nv := fmt.Sprintf("%v", newResp[field])
		if lv != nv {
			diffs = append(diffs, fmt.Sprintf("field %q: legacy=%q new=%q", field, lv, nv))
		}
	}

	// Check affected[] count
	lAffected, _ := legacy["affected"].([]interface{})
	nAffected, _ := newResp["affected"].([]interface{})
	if len(lAffected) != len(nAffected) {
		diffs = append(diffs, fmt.Sprintf("affected[] count: legacy=%d new=%d",
			len(lAffected), len(nAffected)))
	}

	return diffs
}

func main() {
	cfg := Config{}
	var durationStr string
	flag.StringVar(&cfg.LegacyURL, "legacy-url", "http://api.osv.dev", "Legacy API base URL")
	flag.StringVar(&cfg.NewURL, "new-url", "http://new-api.osv.dev", "New API base URL")
	flag.Float64Var(&cfg.SampleRate, "sample-rate", 0.01, "Fraction of requests to sample")
	flag.StringVar(&durationStr, "duration", "1h", "How long to run comparisons")
	flag.StringVar(&cfg.OutputFile, "output", "diff-report.json", "Output report file")
	flag.IntVar(&cfg.Concurrency, "concurrency", 10, "Concurrent comparison workers")
	flag.StringVar(&cfg.VulnListFile, "vuln-list", "vuln-ids.txt", "File with vuln IDs to sample")
	flag.Parse()

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Fatalf("Invalid duration: %v", err)
	}
	cfg.Duration = duration

	// Load vuln IDs
	data, err := os.ReadFile(cfg.VulnListFile)
	if err != nil {
		log.Fatalf("Failed to read vuln list %s: %v", cfg.VulnListFile, err)
	}
	allIDs := strings.Split(strings.TrimSpace(string(data)), "\n")
	fmt.Printf("Loaded %d vuln IDs for comparison\n", len(allIDs))

	var (
		totalSampled int64
		totalDiffs   int64
		totalLegacyMS int64
		totalNewMS   int64
		mu           sync.Mutex
		results      []ComparisonResult
	)

	sem  := make(chan struct{}, cfg.Concurrency)
	stop := time.After(cfg.Duration)
	var wg sync.WaitGroup

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	fmt.Printf("Running comparisons for %s at %.0f%% sample rate...\n",
		cfg.Duration, cfg.SampleRate*100)

loop:
	for {
		select {
		case <-stop:
			break loop
		case <-ticker.C:
			if rand.Float64() > cfg.SampleRate {
				continue
			}
			vulnID := allIDs[rand.Intn(len(allIDs))]

			wg.Add(1)
			sem <- struct{}{}
			go func(id string) {
				defer wg.Done()
				defer func() { <-sem }()

				legacyData, legacyMS, err1 := fetchVuln(cfg.LegacyURL, id)
				newData, newMS, err2 := fetchVuln(cfg.NewURL, id)

				result := ComparisonResult{VulnID: id, LegacyMS: legacyMS, NewMS: newMS}
				if err1 != nil || err2 != nil {
					result.Diffs = []string{fmt.Sprintf("fetch error: legacy=%v new=%v", err1, err2)}
				} else {
					result.Diffs = compareResponses(legacyData, newData)
				}
				result.Match = len(result.Diffs) == 0

				atomic.AddInt64(&totalSampled, 1)
				atomic.AddInt64(&totalLegacyMS, legacyMS)
				atomic.AddInt64(&totalNewMS, newMS)
				if !result.Match {
					atomic.AddInt64(&totalDiffs, 1)
				}

				mu.Lock()
				if len(results) < 1000 { // Keep max 1000 samples in memory
					results = append(results, result)
				}
				mu.Unlock()
			}(vulnID)
		}
	}

	wg.Wait()

	// Build report
	sampled := atomic.LoadInt64(&totalSampled)
	diffs := atomic.LoadInt64(&totalDiffs)
	diffRate := float64(0)
	if sampled > 0 {
		diffRate = float64(diffs) / float64(sampled)
	}

	report := Report{
		RunAt:        time.Now(),
		Duration:     duration.String(),
		TotalSampled: sampled,
		TotalDiffs:   diffs,
		DiffRate:     diffRate,
		Samples:      results,
		Passed:       diffRate < 0.0001, // < 0.01% threshold
	}
	if sampled > 0 {
		report.AvgLegacyMS = float64(totalLegacyMS) / float64(sampled)
		report.AvgNewMS = float64(totalNewMS) / float64(sampled)
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(cfg.OutputFile, out, 0644); err != nil {
		log.Printf("Failed to write report: %v", err)
	}

	fmt.Printf("\n=== Comparison Report ===\n")
	fmt.Printf("Sampled: %d requests\n", sampled)
	fmt.Printf("Diffs:   %d (%.4f%%)\n", diffs, diffRate*100)
	fmt.Printf("Avg latency — legacy: %.0fms | new: %.0fms\n", report.AvgLegacyMS, report.AvgNewMS)
	fmt.Printf("Report saved: %s\n", cfg.OutputFile)

	if report.Passed {
		fmt.Printf("\nResult: PASS ✓\n")
		os.Exit(0)
	} else {
		fmt.Printf("\nResult: FAIL ✗ (diff rate %.4f%% > 0.01%% threshold)\n", diffRate*100)
		os.Exit(1)
	}
}
