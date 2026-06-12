// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package handler contains HTTP handlers for the Web BFF.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────
// Data types
// ─────────────────────────────────────────────

// EcosystemStats holds the count of vulnerabilities per ecosystem.
type EcosystemStats struct {
	Ecosystem string `json:"ecosystem"`
	VulnCount int    `json:"vuln_count"`
}

// HomepageStatsResponse is the response payload for GET /api/v1/stats.
type HomepageStatsResponse struct {
	TotalVulns  int              `json:"total_vulns"`
	Ecosystems  []EcosystemStats `json:"ecosystems"`
	LastUpdated time.Time        `json:"last_updated"`
}

// VulnerabilityDetail is the aggregated vuln detail response.
type VulnerabilityDetail struct {
	ID           string          `json:"id"`
	Summary      string          `json:"summary"`
	Details      string          `json:"details"`
	Modified     time.Time       `json:"modified"`
	Published    time.Time       `json:"published,omitempty"`
	Severity     string          `json:"severity,omitempty"`
	CVSSScore    float64         `json:"cvss_score,omitempty"`
	Ecosystems   []string        `json:"ecosystems,omitempty"`
	Packages     []string        `json:"packages,omitempty"`
	Aliases      []string        `json:"aliases,omitempty"`
	Affected     json.RawMessage `json:"affected,omitempty"`
	References   json.RawMessage `json:"references,omitempty"`
	AIMetadata   *AIMetadata     `json:"ai_metadata,omitempty"`
	RelatedVulns []RelatedVuln   `json:"related_vulns,omitempty"`
}

// AIMetadata holds AI-enriched vulnerability metadata.
type AIMetadata struct {
	TechnicalSummary    string   `json:"technical_summary,omitempty"`
	RemediationAdvice   string   `json:"remediation_advice,omitempty"`
	AttackVectorTags    []string `json:"attack_vector_tags,omitempty"`
	ExploitabilityScore float64  `json:"exploitability_score,omitempty"`
}

// RelatedVuln is a related vulnerability summary.
type RelatedVuln struct {
	ID      string  `json:"id"`
	Summary string  `json:"summary"`
	Score   float64 `json:"score"`
}

// LintResult is the OSV linter response.
type LintResult struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ─────────────────────────────────────────────
// Ports (upstream gRPC clients abstracted)
// ─────────────────────────────────────────────

// QueryServiceClient provides access to the Vulnerability Query Service.
type QueryServiceClient interface {
	GetByID(ctx context.Context, id string) (*VulnerabilityDetail, error)
	GetEcosystemStats(ctx context.Context) (*HomepageStatsResponse, error)
}

// SearchServiceClient provides access to the Search Service.
type SearchServiceClient interface {
	Search(ctx context.Context, query string, pageSize int, pageToken string) (json.RawMessage, error)
	Autocomplete(ctx context.Context, prefix string, maxResults int) ([]string, error)
}

// AIEnrichmentClient provides access to the AI Enrichment Service.
type AIEnrichmentClient interface {
	GetEnrichment(ctx context.Context, vulnID string) (*AIMetadata, error)
}

// StatsCache provides stale-while-revalidate caching for homepage stats.
type StatsCache interface {
	Get(ctx context.Context, key string) (*HomepageStatsResponse, bool)
	GetStale(ctx context.Context, key string) (*HomepageStatsResponse, bool)
	Set(ctx context.Context, key string, v *HomepageStatsResponse, ttl time.Duration)
}

// ─────────────────────────────────────────────
// HomepageHandler
// ─────────────────────────────────────────────

// HomepageHandler serves GET /api/v1/stats.
type HomepageHandler struct {
	queryClient QueryServiceClient
	statsCache  StatsCache
}

// NewHomepageHandler creates a new HomepageHandler.
func NewHomepageHandler(qc QueryServiceClient, cache StatsCache) *HomepageHandler {
	return &HomepageHandler{queryClient: qc, statsCache: cache}
}

// GetStats serves ecosystem stats with stale-while-revalidate (hard TTL 24h, soft 30min).
func (h *HomepageHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	const cacheKey = "homepage_stats"

	if cached, ok := h.statsCache.Get(r.Context(), cacheKey); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	stats, err := h.queryClient.GetEcosystemStats(r.Context())
	if err != nil {
		if stale, ok := h.statsCache.GetStale(r.Context(), cacheKey); ok {
			writeJSON(w, http.StatusOK, stale)
			return
		}
		writeError(w, http.StatusBadGateway, "upstream unavailable")
		return
	}

	h.statsCache.Set(r.Context(), cacheKey, stats, 24*time.Hour)
	writeJSON(w, http.StatusOK, stats)
}

// ─────────────────────────────────────────────
// VulnerabilityHandler
// ─────────────────────────────────────────────

// VulnerabilityHandler serves /api/v1/vulns/{id}.
type VulnerabilityHandler struct {
	queryClient QueryServiceClient
	aiClient    AIEnrichmentClient
}

// NewVulnerabilityHandler creates a new VulnerabilityHandler.
func NewVulnerabilityHandler(qc QueryServiceClient, ai AIEnrichmentClient) *VulnerabilityHandler {
	return &VulnerabilityHandler{queryClient: qc, aiClient: ai}
}

// GetDetail fetches vulnerability details and AI enrichment in parallel.
// AI metadata is optional — degrades gracefully on AI service failure.
func (h *VulnerabilityHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	// Extract {id} from URL (chi router).
	id := extractPathParam(r.URL.Path, "/api/v1/vulns/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "vulnerability ID is required")
		return
	}

	var (
		vuln   *VulnerabilityDetail
		aiMeta *AIMetadata
		vulnErr error
		mu     sync.Mutex
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		v, err := h.queryClient.GetByID(r.Context(), id)
		mu.Lock()
		vuln, vulnErr = v, err
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		if h.aiClient == nil {
			return
		}
		ai, _ := h.aiClient.GetEnrichment(r.Context(), id) // best-effort
		mu.Lock()
		aiMeta = ai
		mu.Unlock()
	}()

	wg.Wait()

	if vulnErr != nil {
		writeError(w, http.StatusNotFound, "vulnerability not found")
		return
	}

	if aiMeta != nil {
		vuln.AIMetadata = aiMeta
	}
	writeJSON(w, http.StatusOK, vuln)
}

// ─────────────────────────────────────────────
// SearchHandler
// ─────────────────────────────────────────────

// SearchHandler serves /api/v1/search.
type SearchHandler struct {
	searchClient SearchServiceClient
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(sc SearchServiceClient) *SearchHandler {
	return &SearchHandler{searchClient: sc}
}

// Search proxies queries to the Search Service.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pageToken := r.URL.Query().Get("page_token")
	result, err := h.searchClient.Search(r.Context(), q, 20, pageToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "search unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Autocomplete serves /api/v1/search/autocomplete.
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("q")
	suggestions, err := h.searchClient.Autocomplete(r.Context(), prefix, 10)
	if err != nil {
		suggestions = []string{} // degrade gracefully
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"suggestions": suggestions})
}

// ─────────────────────────────────────────────
// LinterHandler
// ─────────────────────────────────────────────

// LinterHandler serves POST /api/v1/lint.
type LinterHandler struct{}

// Lint validates OSV JSON in-process (no external calls).
func (h *LinterHandler) Lint(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result := validateOSVJSON(raw)
	code := http.StatusOK
	if !result.IsValid {
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, result)
}

// validateOSVJSON performs basic OSV schema validation in-process.
func validateOSVJSON(raw json.RawMessage) *LintResult {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return &LintResult{IsValid: false, Errors: []string{"invalid JSON: " + err.Error()}}
	}

	var errors, warnings []string
	requiredFields := []string{"id", "modified"}
	for _, f := range requiredFields {
		if _, ok := obj[f]; !ok {
			errors = append(errors, "missing required field: "+f)
		}
	}
	if _, ok := obj["summary"]; !ok {
		warnings = append(warnings, "missing recommended field: summary")
	}
	return &LintResult{
		IsValid:  len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}
}

// ─────────────────────────────────────────────
// HealthHandler
// ─────────────────────────────────────────────

// HealthHandler serves /health/live and /health/ready.
type HealthHandler struct {
	queryClient QueryServiceClient
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(qc QueryServiceClient) *HealthHandler {
	return &HealthHandler{queryClient: qc}
}

// Live always returns 200.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready checks upstream reachability.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	_, err := h.queryClient.GetEcosystemStats(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "upstream query service unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ─────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func extractPathParam(path, prefix string) string {
	after := strings.TrimPrefix(path, prefix)
	return strings.TrimSuffix(after, "/")
}
