// Package handler implements the Admin API HTTP handlers for the OSV admin control plane.
// This handler manages CVE sources, import findings, vulnerabilities, API keys, and system health.
package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/osv/identity-service/internal/domain/entity"
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
	ID         string     `json:"id"`
	SourceName string     `json:"source_name"`
	VulnID     string     `json:"vuln_id,omitempty"`
	Category   string     `json:"category"` // "invalid_schema", "duplicate", "parse_error"
	Message    string     `json:"message"`
	OccurredAt time.Time  `json:"occurred_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	IsResolved bool       `json:"is_resolved"`
}

// VulnStats holds aggregate statistics.
type VulnStats struct {
	TotalVulns    int            `json:"total_vulns"`
	ByEcosystem   map[string]int `json:"by_ecosystem"`
	BySource      map[string]int `json:"by_source"`
	Withdrawn     int            `json:"withdrawn"`
	WithCVSS      int            `json:"with_cvss"`
	WithKEV       int            `json:"with_kev"`
	LastUpdatedAt time.Time      `json:"last_updated_at"`
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

// APIKey is the API response model for API keys.
type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"` // first chars visible
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	IsActive   bool       `json:"is_active"`
}

// APIKeyRepository defines the persistence operations for API keys.
type APIKeyRepository interface {
	Create(ctx context.Context, k *entity.APIKey) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Handler holds dependencies for admin API handlers.
type Handler struct {
	db         *pgxpool.Pool
	nats       *nats.Conn
	apiKeyRepo APIKeyRepository
}

// New creates a new admin Handler with real dependencies.
func New(db *pgxpool.Pool, nc *nats.Conn, apiKeyRepo APIKeyRepository) *Handler {
	return &Handler{
		db:         db,
		nats:       nc,
		apiKeyRepo: apiKeyRepo,
	}
}

// ── Sources (TASK-06-01) ──────────────────────────────────────────────────────

// ListSources returns all configured CVE sources with their sync status from DB.
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT name, url, state, last_sync_at, last_sync_error, total_vulns, sync_interval
		FROM cve_sources
		ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query sources")
		return
	}
	defer rows.Close()

	var sources []SourceStatus
	for rows.Next() {
		var s SourceStatus
		var syncInterval int64
		if err := rows.Scan(&s.Name, &s.URL, &s.State, &s.LastSyncAt, &s.LastSyncErr, &s.TotalVulns, &syncInterval); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan source")
			return
		}
		s.SyncInterval = fmt.Sprintf("%dh", syncInterval/3600)
		sources = append(sources, s)
	}
	if sources == nil {
		sources = []SourceStatus{}
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
	var s SourceStatus
	var syncInterval int64
	err := h.db.QueryRow(r.Context(), `
		SELECT name, url, state, last_sync_at, last_sync_error, total_vulns, sync_interval
		FROM cve_sources WHERE name = $1`, name).Scan(
		&s.Name, &s.URL, &s.State, &s.LastSyncAt, &s.LastSyncErr, &s.TotalVulns, &syncInterval)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	s.SyncInterval = fmt.Sprintf("%dh", syncInterval/3600)
	writeJSON(w, http.StatusOK, s)
}

// TriggerSync publishes a sync request for a source via NATS.
func (h *Handler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "source name required")
		return
	}

	payload := fmt.Sprintf(`{"source":"%s","requested_at":"%s"}`, name, time.Now().UTC().Format(time.RFC3339))
	if h.nats != nil {
		if err := h.nats.Publish("source.sync.requested", []byte(payload)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish sync request")
			return
		}
	}

	// Update state to "syncing" in DB
	_, _ = h.db.Exec(r.Context(), `UPDATE cve_sources SET state='syncing' WHERE name=$1`, name)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"source":  name,
		"message": "sync request published",
	})
}

// PauseSource pauses sync for a source.
func (h *Handler) PauseSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.db.Exec(r.Context(), `UPDATE cve_sources SET state='paused' WHERE name=$1`, name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pause source")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused", "source": name})
}

// ResumeSource resumes sync for a source.
func (h *Handler) ResumeSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.db.Exec(r.Context(), `UPDATE cve_sources SET state='active' WHERE name=$1`, name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resume source")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active", "source": name})
}

// ── Import Findings (TASK-06-02) ──────────────────────────────────────────────

// ListImportFindings returns recent import errors and warnings from DB.
func (h *Handler) ListImportFindings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source := q.Get("source")
	resolved := q.Get("resolved") == "true"
	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	query := `SELECT id, source_name, vuln_id, category, message, occurred_at, resolved_at, is_resolved
		FROM import_findings WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if source != "" {
		query += fmt.Sprintf(" AND source_name=$%d", argIdx)
		args = append(args, source)
		argIdx++
	}
	if !resolved {
		query += fmt.Sprintf(" AND is_resolved=$%d", argIdx)
		args = append(args, false)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query import findings")
		return
	}
	defer rows.Close()

	var findings []ImportFinding
	for rows.Next() {
		var f ImportFinding
		if err := rows.Scan(&f.ID, &f.SourceName, &f.VulnID, &f.Category, &f.Message,
			&f.OccurredAt, &f.ResolvedAt, &f.IsResolved); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan import finding")
			return
		}
		findings = append(findings, f)
	}
	if findings == nil {
		findings = []ImportFinding{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": findings, "count": len(findings)})
}

// ResolveImportFinding marks an import finding as resolved in DB.
func (h *Handler) ResolveImportFinding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "finding id required")
		return
	}
	now := time.Now().UTC()
	tag, err := h.db.Exec(r.Context(),
		`UPDATE import_findings SET is_resolved=true, resolved_at=$1 WHERE id=$2`,
		now, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "import finding not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          id,
		"is_resolved": true,
		"resolved_at": now,
	})
}

// ── Vulnerability Admin (TASK-06-03) ─────────────────────────────────────────

// WithdrawVuln withdraws a vulnerability and publishes a NATS event.
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

	now := time.Now().UTC()

	// Update DB
	_, err := h.db.Exec(r.Context(),
		`UPDATE vulnerabilities SET withdrawn=true, withdrawn_at=$1, withdrawn_reason=$2 WHERE id=$3`,
		now, body.Reason, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to withdraw vulnerability")
		return
	}

	// Publish event
	if h.nats != nil {
		payload := fmt.Sprintf(`{"vuln_id":"%s","withdrawn":true,"reason":"%s","withdrawn_at":"%s"}`,
			id, body.Reason, now.Format(time.RFC3339))
		_ = h.nats.Publish("vuln.withdrawn", []byte(payload))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"vuln_id":      id,
		"withdrawn":    true,
		"withdrawn_at": now,
		"reason":       body.Reason,
	})
}

// ReprocessVuln triggers reprocessing of a vulnerability via NATS.
func (h *Handler) ReprocessVuln(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "vuln id required")
		return
	}

	if h.nats != nil {
		payload := fmt.Sprintf(`{"vuln_id":"%s","requested_at":"%s"}`, id, time.Now().UTC().Format(time.RFC3339))
		if err := h.nats.Publish("vuln.reprocess.requested", []byte(payload)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish reprocess request")
			return
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"vuln_id": id,
		"status":  "reprocess_queued",
		"message": "vulnerability queued for reprocessing",
	})
}

// VulnStats returns aggregate vulnerability statistics from DB.
func (h *Handler) VulnStats(w http.ResponseWriter, r *http.Request) {
	var stats VulnStats
	stats.ByEcosystem = map[string]int{}
	stats.BySource = map[string]int{}

	// Total count
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM vulnerabilities`).Scan(&stats.TotalVulns)
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM vulnerabilities WHERE withdrawn=true`).Scan(&stats.Withdrawn)
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM vulnerabilities WHERE cvss_score IS NOT NULL`).Scan(&stats.WithCVSS)
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM vulnerabilities WHERE is_kev=true`).Scan(&stats.WithKEV)
	_ = h.db.QueryRow(r.Context(), `SELECT MAX(updated_at) FROM vulnerabilities`).Scan(&stats.LastUpdatedAt)

	// By ecosystem
	rows, _ := h.db.Query(r.Context(), `SELECT ecosystem, COUNT(*) FROM vulnerabilities GROUP BY ecosystem`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var eco string
			var cnt int
			if err := rows.Scan(&eco, &cnt); err == nil {
				stats.ByEcosystem[eco] = cnt
			}
		}
	}

	// By source
	srows, _ := h.db.Query(r.Context(), `SELECT source_name, COUNT(*) FROM vulnerabilities GROUP BY source_name`)
	if srows != nil {
		defer srows.Close()
		for srows.Next() {
			var src string
			var cnt int
			if err := srows.Scan(&src, &cnt); err == nil {
				stats.BySource[src] = cnt
			}
		}
	}

	writeJSON(w, http.StatusOK, stats)
}

// ── System Health (TASK-06-07) ────────────────────────────────────────────────

// SystemHealth returns health of all subsystems by probing each.
func (h *Handler) SystemHealth(w http.ResponseWriter, r *http.Request) {
	health := SystemHealth{
		Status:     "healthy",
		Timestamp:  time.Now().UTC(),
		Components: make(map[string]ComponentHealth),
	}

	// Probe PostgreSQL
	pgStart := time.Now()
	if err := h.db.Ping(r.Context()); err != nil {
		health.Components["postgres"] = ComponentHealth{Status: "down", Message: err.Error()}
		health.Status = "degraded"
	} else {
		health.Components["postgres"] = ComponentHealth{
			Status:  "ok",
			Latency: fmt.Sprintf("%dms", time.Since(pgStart).Milliseconds()),
		}
	}

	// Probe NATS
	if h.nats != nil {
		if h.nats.IsConnected() {
			health.Components["nats"] = ComponentHealth{Status: "ok"}
		} else {
			health.Components["nats"] = ComponentHealth{Status: "down", Message: "not connected"}
			health.Status = "degraded"
		}
	} else {
		health.Components["nats"] = ComponentHealth{Status: "down", Message: "not configured"}
	}

	writeJSON(w, http.StatusOK, health)
}

// ── API Key Management (TASK-06-06) ──────────────────────────────────────────

// ListAPIKeys returns all API keys for the requesting user.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by auth middleware)
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		writeError(w, http.StatusUnauthorized, "user ID required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	keys, err := h.apiKeyRepo.ListByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list API keys")
		return
	}

	apiKeys := make([]APIKey, 0, len(keys))
	for _, k := range keys {
		ak := APIKey{
			ID:        k.ID.String(),
			Name:      k.Name,
			Prefix:    k.Prefix,
			Scopes:    k.Permissions,
			CreatedAt: k.CreatedAt,
			IsActive:  k.RevokedAt == nil,
		}
		if k.ExpiresAt != nil {
			ak.ExpiresAt = k.ExpiresAt
		}
		if k.LastUsedAt != nil {
			ak.LastUsedAt = k.LastUsedAt
		}
		apiKeys = append(apiKeys, ak)
	}

	writeJSON(w, http.StatusOK, map[string]any{"keys": apiKeys, "count": len(apiKeys)})
}

// CreateAPIKey creates a new API key for the user.
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

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		writeError(w, http.StatusUnauthorized, "user ID required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Generate secure random key
	rawBytes := make([]byte, 24)
	if _, err := rand.Read(rawBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}
	rawKey := "osv_" + hex.EncodeToString(rawBytes)
	prefix := rawKey[:12]

	// Hash the key before storing
	import_hash := hashAPIKey(rawKey)

	k := &entity.APIKey{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        body.Name,
		KeyHash:     import_hash,
		Prefix:      prefix,
		Permissions: body.Scopes,
		ExpiresAt:   body.ExpiresAt,
	}

	if err := h.apiKeyRepo.Create(r.Context(), k); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create API key")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      k.ID.String(),
		"name":    k.Name,
		"prefix":  k.Prefix,
		"scopes":  k.Permissions,
		"key":     rawKey, // Shown ONCE — not stored in plain text
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
	keyID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	if err := h.apiKeyRepo.Revoke(r.Context(), keyID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke key")
		return
	}

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

// hashAPIKey returns SHA-256 hex of the raw key for secure storage.
func hashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}
