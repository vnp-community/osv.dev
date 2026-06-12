// Package handler implements the Admin API HTTP handlers (TASK-06-01 to 06-07).
//
// Each handler is self-contained and writes JSON responses.
// Business logic is minimal — the admin service acts as an operational control plane
// that delegates to domain services via gRPC or NATS events.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SourceStatus is the admin view of a CVE source.
type SourceStatus struct {
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	State        string     `json:"state"` // "active", "paused", "error"
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
	LastSyncErr  string     `json:"last_sync_error,omitempty"`
	TotalVulns   int        `json:"total_vulns"`
	SyncInterval string     `json:"sync_interval"`
}

// ImportFinding represents a failed or suspicious import event.
type ImportFinding struct {
	ID          string    `json:"id"`
	SourceName  string    `json:"source_name"`
	VulnID      string    `json:"vuln_id,omitempty"`
	Category    string    `json:"category"` // "invalid_schema", "duplicate", "parse_error"
	Message     string    `json:"message"`
	OccurredAt  time.Time `json:"occurred_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	IsResolved  bool      `json:"is_resolved"`
}

// VulnStats holds aggregate statistics.
type VulnStats struct {
	TotalVulns      int                `json:"total_vulns"`
	ByEcosystem     map[string]int     `json:"by_ecosystem"`
	BySource        map[string]int     `json:"by_source"`
	Withdrawn       int                `json:"withdrawn"`
	WithCVSS        int                `json:"with_cvss"`
	WithKEV         int                `json:"with_kev"`
	LastUpdatedAt   time.Time          `json:"last_updated_at"`
}

// SystemHealth reports health of all subsystems.
type SystemHealth struct {
	Status     string                    `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp  time.Time                 `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth is the health status of a single component.
type ComponentHealth struct {
	Status  string `json:"status"` // "ok", "degraded", "down"
	Latency string `json:"latency_ms,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIKey holds an API key for programmatic access.
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Prefix      string    `json:"prefix"` // first 8 chars visible
	Scopes      []string  `json:"scopes"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	IsActive    bool      `json:"is_active"`
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Handler holds dependencies for admin API handlers.
type Handler struct {
	// Future: inject service clients (source-sync gRPC, Firestore, NATS)
}

// New creates a new admin Handler.
func New() *Handler {
	return &Handler{}
}

// ── Sources (TASK-06-01) ──────────────────────────────────────────────────────

// ListSources returns all configured CVE sources with their sync status.
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	// TODO: Load from source-sync service. Returning mock data.
	sources := []SourceStatus{
		{
			Name:         "nvd",
			URL:          "https://nvd.nist.gov/feeds/json/cve/2.0/",
			State:        "active",
			TotalVulns:   250000,
			SyncInterval: "6h",
		},
		{
			Name:         "github-advisory-database",
			URL:          "https://github.com/github/advisory-database",
			State:        "active",
			TotalVulns:   35000,
			SyncInterval: "1h",
		},
		{
			Name:         "osv-malicious-packages",
			URL:          "https://github.com/ossf/malicious-packages",
			State:        "active",
			TotalVulns:   5000,
			SyncInterval: "30m",
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources, "count": len(sources)})
}

// GetSource returns detail for a single source by name.
func (h *Handler) GetSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "source name required")
		return
	}
	// TODO: Query source-sync service
	src := SourceStatus{
		Name:         name,
		URL:          fmt.Sprintf("https://example.com/%s", name),
		State:        "active",
		TotalVulns:   0,
		SyncInterval: "6h",
	}
	writeJSON(w, http.StatusOK, src)
}

// TriggerSync publishes a sync request for a source.
func (h *Handler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "source name required")
		return
	}
	// TODO: Publish "source.sync.requested" to NATS
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"source":  name,
		"message": "sync request published",
	})
}

// PauseSource pauses sync for a source.
func (h *Handler) PauseSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	// TODO: Update source state via source-sync service
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "paused",
		"source": name,
	})
}

// ResumeSource resumes sync for a source.
func (h *Handler) ResumeSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	// TODO: Update source state via source-sync service
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "active",
		"source": name,
	})
}

// ── Import Findings (TASK-06-02) ──────────────────────────────────────────────

// ListImportFindings returns recent import errors and warnings.
func (h *Handler) ListImportFindings(w http.ResponseWriter, r *http.Request) {
	// Query params: ?source=nvd&resolved=false&limit=50
	limit := 50
	resolved := r.URL.Query().Get("resolved") == "true"
	source := r.URL.Query().Get("source")

	// TODO: Query Firestore import_findings collection
	findings := []ImportFinding{}
	_ = limit
	_ = resolved
	_ = source

	writeJSON(w, http.StatusOK, map[string]any{
		"findings": findings,
		"count":    len(findings),
	})
}

// ResolveImportFinding marks an import finding as resolved.
func (h *Handler) ResolveImportFinding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "finding id required")
		return
	}
	now := time.Now().UTC()
	// TODO: Update Firestore import_findings/{id}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          id,
		"is_resolved": true,
		"resolved_at": now,
	})
}

// ── Vulnerability Admin (TASK-06-03) ─────────────────────────────────────────

// WithdrawVuln withdraws a vulnerability from the public dataset.
func (h *Handler) WithdrawVuln(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "vuln id required")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// TODO: Set withdrawn=true in Firestore + publish vuln.withdrawn event
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, map[string]any{
		"vuln_id":      id,
		"withdrawn":    true,
		"withdrawn_at": now,
		"reason":       body.Reason,
	})
}

// ReprocessVuln triggers reprocessing of a vulnerability.
func (h *Handler) ReprocessVuln(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "vuln id required")
		return
	}
	// TODO: Publish "vuln.reprocess.requested" to NATS
	writeJSON(w, http.StatusAccepted, map[string]string{
		"vuln_id": id,
		"status":  "reprocess_queued",
		"message": "vulnerability queued for reprocessing",
	})
}

// VulnStats returns aggregate vulnerability statistics.
func (h *Handler) VulnStats(w http.ResponseWriter, r *http.Request) {
	// TODO: Aggregate from Firestore
	stats := VulnStats{
		TotalVulns: 0,
		ByEcosystem: map[string]int{
			"PyPI":       0,
			"npm":        0,
			"Maven":      0,
			"Go":         0,
			"crates.io":  0,
			"NuGet":      0,
		},
		BySource: map[string]int{
			"nvd":                       0,
			"github-advisory-database":  0,
			"osv-malicious-packages":    0,
		},
		Withdrawn:     0,
		WithCVSS:      0,
		WithKEV:       0,
		LastUpdatedAt: time.Now().UTC(),
	}
	writeJSON(w, http.StatusOK, stats)
}

// ── System Health (TASK-06-07) ────────────────────────────────────────────────

// SystemHealth returns health of all subsystems.
func (h *Handler) SystemHealth(w http.ResponseWriter, r *http.Request) {
	// TODO: Probe each service in parallel
	health := SystemHealth{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Components: map[string]ComponentHealth{
			"firestore": {Status: "ok", Latency: "12ms"},
			"nats":      {Status: "ok", Latency: "2ms"},
			"redis":     {Status: "ok", Latency: "1ms"},
			"opensearch": {Status: "ok", Latency: "25ms"},
			"source-sync": {Status: "ok"},
			"ai-enrichment": {Status: "ok"},
		},
	}
	writeJSON(w, http.StatusOK, health)
}

// ── API Key Management (TASK-06-06) ──────────────────────────────────────────

// ListAPIKeys returns all API keys.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: Query Firestore api_keys collection
	writeJSON(w, http.StatusOK, map[string]any{"keys": []APIKey{}, "count": 0})
}

// CreateAPIKey creates a new API key.
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// TODO: Generate key, hash it, store in Firestore
	// Return full key ONLY once (not stored in plain text)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     "key-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":   body.Name,
		"scopes": body.Scopes,
		"key":    "osvadm_PLACEHOLDER_CHANGE_ME", // Real key generated here
		"prefix": "osvadm_",
		"message": "Store this key securely — it will not be shown again",
	})
}

// RevokeAPIKey revokes an API key.
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "key id required")
		return
	}
	// TODO: Set is_active=false in Firestore
	writeJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": "revoked",
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
