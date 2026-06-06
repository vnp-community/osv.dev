// smoke-test/main.go — Quick sanity check of all OSV service endpoints
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Endpoint struct {
	Name     string
	URL      string
	Expected int
}

type Result struct {
	Name    string
	URL     string
	Status  int
	Latency time.Duration
	Passed  bool
	Error   string
}

func main() {
	baseURL := flag.String("base-url", "https://api.osv.dev", "Base API URL")
	timeout := flag.Duration("timeout", 10*time.Second, "Per-request timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}

	endpoints := []Endpoint{
		{Name: "Health (Gateway)", URL: *baseURL + "/healthz", Expected: 200},
		{Name: "Get Vuln (CVE)", URL: *baseURL + "/v1/vulns/CVE-2021-44228", Expected: 200},
		{Name: "Get Vuln (GHSA)", URL: *baseURL + "/v1/vulns/GHSA-jfh8-c2jp-5v3q", Expected: 200},
		{Name: "Query by Package", URL: *baseURL + "/v1/query", Expected: 200},
		{Name: "Batch Query", URL: *baseURL + "/v1/querybatch", Expected: 200},
		{Name: "Not Found (404)", URL: *baseURL + "/v1/vulns/CVE-9999-99999", Expected: 404},
	}

	fmt.Printf("=== OSV Smoke Test ===\n")
	fmt.Printf("Base URL: %s\n\n", *baseURL)

	results := []Result{}
	failures := 0

	for _, ep := range endpoints {
		r := Result{Name: ep.Name, URL: ep.URL}
		start := time.Now()

		resp, err := client.Get(ep.URL) //nolint:gosec
		r.Latency = time.Since(start)

		if err != nil {
			r.Error = err.Error()
			r.Passed = false
		} else {
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()
			r.Status = resp.StatusCode
			r.Passed = resp.StatusCode == ep.Expected
			if !r.Passed {
				r.Error = fmt.Sprintf("expected %d, got %d", ep.Expected, resp.StatusCode)
			}
		}

		if r.Passed {
			fmt.Printf("  ✓ %-40s %3dms\n", r.Name, r.Latency.Milliseconds())
		} else {
			fmt.Printf("  ✗ %-40s FAIL: %s\n", r.Name, r.Error)
			failures++
		}

		results = append(results, r)
	}

	fmt.Printf("\n=== Summary: %d/%d passed ===\n", len(endpoints)-failures, len(endpoints))
	if failures > 0 {
		os.Exit(1)
	}
}
