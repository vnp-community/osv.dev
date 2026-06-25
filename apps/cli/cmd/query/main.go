// Command osv-query queries the OSV vulnerability database.
// This is a NEW command — no existing CLI code is modified.
//
// Query modes:
//
//	# By package + ecosystem + version:
//	osv-query -ecosystem npm -package lodash -version 4.17.20
//
//	# By git commit hash:
//	osv-query -commit abc123def456
//
//	# By CVE/OSV ID (full details):
//	osv-query -id CVE-2021-44228
//
//	# Batch query from JSON file:
//	osv-query -batch queries.json
//
//	# Interactive search:
//	osv-query -search "log4j remote code"
//
// Environment variables:
//
//	OSV_API_BASE — OSV v1 API base URL (default: http://localhost:8080 for local gateway)
//	              Set to https://api.osv.dev for production
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("query failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	// Flags
	ecosystem := flag.String("ecosystem", "", "Package ecosystem (npm, PyPI, Go, Maven, etc.)")
	pkg       := flag.String("package", "", "Package name")
	version   := flag.String("version", "", "Package version")
	commit    := flag.String("commit", "", "Git commit hash")
	cveID     := flag.String("id", "", "CVE or OSV ID (e.g. CVE-2021-44228)")
	search    := flag.String("search", "", "Full-text search query")
	format    := flag.String("output", "table", "Output format: table | json")
	limit     := flag.Int("limit", 10, "Max results for search mode")
	flag.Parse()

	baseURL := os.Getenv("OSV_API_BASE")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 0}

	switch {
	case *cveID != "":
		return queryByID(ctx, client, baseURL, *cveID, *format)
	case *commit != "":
		return queryByCommit(ctx, client, baseURL, *commit, *format)
	case *pkg != "":
		return queryByPackage(ctx, client, baseURL, *ecosystem, *pkg, *version, *format)
	case *search != "":
		return querySearch(ctx, client, baseURL, *search, *limit, *format)
	default:
		return fmt.Errorf("one of -id, -commit, -package, or -search is required")
	}
}

// ── Query implementations ─────────────────────────────────────────────────────

func queryByID(ctx context.Context, client *http.Client, base, id, format string) error {
	reqURL := fmt.Sprintf("%s/v1/vulns/%s", base, url.PathEscape(id))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Vulnerability %s not found.\n", id)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}

	if format == "json" {
		fmt.Println(string(body))
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	printVulnDetail(result)
	return nil
}

func queryByPackage(ctx context.Context, client *http.Client, base, ecosystem, pkg, version, format string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"package": map[string]string{
			"ecosystem": ecosystem,
			"name":      pkg,
		},
		"version": version,
	})
	return postQuery(ctx, client, base+"/v1/query", body, format)
}

func queryByCommit(ctx context.Context, client *http.Client, base, commit, format string) error {
	body, _ := json.Marshal(map[string]string{"commit": commit})
	return postQuery(ctx, client, base+"/v1/query", body, format)
}

func postQuery(ctx context.Context, client *http.Client, url string, body []byte, format string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, respBody)
	}

	if format == "json" {
		fmt.Println(string(respBody))
		return nil
	}

	var result struct {
		Vulns []struct {
			ID       string   `json:"id"`
			Modified string   `json:"modified"`
			Aliases  []string `json:"aliases,omitempty"`
		} `json:"vulns"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	if len(result.Vulns) == 0 {
		fmt.Println("No vulnerabilities found.")
		return nil
	}
	fmt.Printf("Found %d vulnerabilities:\n\n", len(result.Vulns))
	fmt.Printf("%-20s %-25s %s\n", "ID", "Modified", "Aliases")
	fmt.Println(strings.Repeat("-", 70))
	for _, v := range result.Vulns {
		aliases := strings.Join(v.Aliases, ", ")
		if aliases == "" {
			aliases = "-"
		}
		fmt.Printf("%-20s %-25s %s\n", v.ID, v.Modified, aliases)
	}
	return nil
}

func querySearch(ctx context.Context, client *http.Client, base, query string, limit int, format string) error {
	reqURL := fmt.Sprintf("%s/v1/search?q=%s&limit=%d", base, url.QueryEscape(query), limit)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}

	if format == "json" {
		fmt.Println(string(body))
		return nil
	}

	var result struct {
		Items []struct {
			ID       string  `json:"id"`
			Summary  string  `json:"summary"`
			Severity string  `json:"severity"`
			Score    float64 `json:"cvss_score"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	fmt.Printf("Search results for %q:\n\n", query)
	fmt.Printf("%-20s %-10s %-6s %s\n", "ID", "Severity", "Score", "Summary")
	fmt.Println(strings.Repeat("-", 80))
	for _, item := range result.Items {
		summary := item.Summary
		if len(summary) > 45 {
			summary = summary[:42] + "..."
		}
		fmt.Printf("%-20s %-10s %-6.1f %s\n", item.ID, item.Severity, item.Score, summary)
	}
	return nil
}

func printVulnDetail(v map[string]interface{}) {
	fmt.Printf("ID: %v\n", v["id"])
	fmt.Printf("Summary: %v\n", v["summary"])
	fmt.Printf("Published: %v\n", v["published"])
	fmt.Printf("Modified: %v\n", v["modified"])
	if details, ok := v["details"].(string); ok && details != "" {
		if len(details) > 200 {
			details = details[:200] + "..."
		}
		fmt.Printf("Details: %s\n", details)
	}
}
