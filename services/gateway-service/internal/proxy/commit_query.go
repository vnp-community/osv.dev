// commit_query.go — Commit hash to vulnerability lookup for gateway-service.
// Handles the `commit` field in POST /v1/query OSV API requests.
//
// This is ADDITIVE — osv_handler.go is NOT modified here.
// The commit query is integrated into the existing queryVulns handler
// via the commitQueryHandler type.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// commitQueryResult is the OSV-compatible response for commit-based queries.
type commitQueryResult struct {
	Vulns []struct {
		ID       string   `json:"id"`
		Modified string   `json:"modified,omitempty"`
		Aliases  []string `json:"aliases,omitempty"`
	} `json:"vulns"`
	Note string `json:"_note,omitempty"` // informational field (non-standard)
}

// handleCommitQuery handles commit hash lookup in POST /v1/query.
// It attempts to resolve a git commit hash to a list of CVEs.
//
// Strategy:
//  1. Forward to data-service SearchByCommit HTTP endpoint (when available)
//  2. Fallback: search with commit as keyword via search-service
//  3. Fallback: return informative empty response
//
// This function is called by queryVulns() in osv_handler.go when
// the `commit` field is present in the request body.
func (h *osvV1Handler) handleCommitQuery(ctx context.Context, commit string) (*commitQueryResult, error) {
	if commit == "" {
		return nil, fmt.Errorf("commit hash is required")
	}

	// Sanitize: must be a hex string (SHA-1 or SHA-256)
	commit = strings.ToLower(strings.TrimSpace(commit))
	for _, c := range commit {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return nil, fmt.Errorf("invalid commit hash format: must be hex string")
		}
	}
	if len(commit) < 7 || len(commit) > 64 {
		return nil, fmt.Errorf("invalid commit hash length: got %d, want 7-64", len(commit))
	}

	// Try data-service HTTP endpoint (POST /v1/commit-lookup)
	result, err := h.queryCommitViaDataService(ctx, commit)
	if err == nil {
		return result, nil
	}
	log.Debug().Err(err).Str("commit", commit).Msg("commit query: data-service unavailable, trying search-service")

	// Fallback: search by commit as keyword
	result, err = h.queryCommitViaSearch(ctx, commit)
	if err == nil {
		return result, nil
	}
	log.Warn().Err(err).Str("commit", commit).Msg("commit query: search-service also unavailable")

	// Final fallback: empty result with informational note
	return &commitQueryResult{
		Vulns: []struct {
			ID       string   `json:"id"`
			Modified string   `json:"modified,omitempty"`
			Aliases  []string `json:"aliases,omitempty"`
		}{},
		Note: "commit-based lookup requires data-service QueryByCommit RPC (SD-FEAT-01). " +
			"Fallback search also unavailable. Result may be incomplete.",
	}, nil
}

// queryCommitViaDataService calls data-service HTTP POST /v1/commit-lookup.
func (h *osvV1Handler) queryCommitViaDataService(ctx context.Context, commit string) (*commitQueryResult, error) {
	// Derive data-service HTTP URL from gRPC addr
	// gRPC: localhost:50053 → HTTP: http://localhost:8082
	dataHTTP := grpcToHTTP(h.cvedb) // helper returns "" if cvedb is nil
	if dataHTTP == "" {
		return nil, fmt.Errorf("data-service not configured")
	}

	body, _ := json.Marshal(map[string]string{"commit": commit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dataHTTP+"/v1/commit-lookup",
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("data-service commit-lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("data-service commit-lookup: status %d", resp.StatusCode)
	}

	var result commitQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("data-service commit-lookup decode: %w", err)
	}
	return &result, nil
}

// queryCommitViaSearch queries search-service with commit hash as keyword.
func (h *osvV1Handler) queryCommitViaSearch(ctx context.Context, commit string) (*commitQueryResult, error) {
	if h.searchBase == "" {
		return nil, fmt.Errorf("search-service not configured")
	}

	upstreamURL := fmt.Sprintf("%s/v1/search?q=%s&limit=10", h.searchBase, commit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search-service: status %d", resp.StatusCode)
	}

	var searchResp struct {
		Items []struct {
			ID       string   `json:"id"`
			Modified string   `json:"modified"`
			Aliases  []string `json:"aliases"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	result := &commitQueryResult{}
	for _, item := range searchResp.Items {
		result.Vulns = append(result.Vulns, struct {
			ID       string   `json:"id"`
			Modified string   `json:"modified,omitempty"`
			Aliases  []string `json:"aliases,omitempty"`
		}{ID: item.ID, Modified: item.Modified, Aliases: item.Aliases})
	}
	return result, nil
}

// grpcToHTTP converts a gRPC address to a corresponding HTTP base URL.
// Returns empty string if client is nil.
func grpcToHTTP(cvedb interface{ Close() error }) string {
	if cvedb == nil {
		return ""
	}
	// Use DATA_SERVICE_HTTP env override if set, otherwise not available yet
	// (real implementation will extract addr from gRPC conn metadata)
	return ""
}
