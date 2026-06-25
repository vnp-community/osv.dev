// Package http provides the JIRA config HTTP handler for jira-service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	jiraconf "github.com/osv/jira-service/internal/domain/jiraconfig"
	pginfra "github.com/osv/jira-service/internal/infra/postgres"
)

// JiraConfigRepository is the persistence interface for JIRA configurations.
type JiraConfigRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*jiraconf.JIRAConfig, error)
	FindByProduct(ctx context.Context, productID uuid.UUID) (*jiraconf.JIRAConfig, error)
	List(ctx context.Context, limit, offset int) ([]*jiraconf.JIRAConfig, int, error)
	Save(ctx context.Context, cfg *jiraconf.JIRAConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// JiraAPIClient tests connectivity to a JIRA instance.
type JiraAPIClient interface {
	TestConnection(ctx context.Context, jiraURL, username, apiToken string) error
	GetVersionAndProject(ctx context.Context, jiraURL, username, apiToken, projectKey string) (version string, projectFound bool, err error)
}

// CryptoService encrypts/decrypts API tokens.
type CryptoService interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(ciphertext string) ([]byte, error)
}



// ConfigHandler handles JIRA configuration HTTP endpoints.
type ConfigHandler struct {
	configRepo  JiraConfigRepository
	jiraClient  JiraAPIClient
	crypto      CryptoService
	platformURL string // for webhook URL construction
	// [FIX TASK-HC-013] issueRepo provides real CRUD for jira_issue_mappings
	issueRepo   *pginfra.IssueMappingRepo
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(repo JiraConfigRepository, client JiraAPIClient, crypto CryptoService, platformURL string) *ConfigHandler {
	return &ConfigHandler{
		configRepo:  repo,
		jiraClient:  client,
		crypto:      crypto,
		platformURL: platformURL,
	}
}

// WithIssueMappingRepo sets the issue mapping repository (TASK-HC-013).
func (h *ConfigHandler) WithIssueMappingRepo(repo *pginfra.IssueMappingRepo) *ConfigHandler {
	h.issueRepo = repo
	return h
}

// GET /jira/config → /jira-configs
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configRepo.FindByProduct(r.Context(), uuid.Nil) // fallback to nil if no product context
	if err != nil || cfg == nil {
		respondJSONConfig(w, 404, map[string]string{"error": "NOT_FOUND", "message": "No JIRA configuration found"})
		return
	}

	respondJSONConfig(w, 200, map[string]interface{}{
		"id":                cfg.ID,
		"jira_url":          cfg.URL,
		"project_key":       cfg.ProjectKey,
		"username":          cfg.Username,
		"api_token_preview": maskToken(cfg.PasswordEnc),
		"is_active":         cfg.IsActive,
		"webhook_url":       h.platformURL + "/api/v1/jira/webhook",
		"created_at":        cfg.CreatedAt.Format(time.RFC3339),
		"last_sync_at":      nil,
	})
}

// POST /jira/config
func (h *ConfigHandler) CreateOrUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JiraURL    string `json:"jira_url"`
		ProjectKey string `json:"project_key"`
		Username   string `json:"username"`
		APIToken   string `json:"api_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "Invalid request body"})
		return
	}
	if req.JiraURL == "" || req.Username == "" || req.APIToken == "" {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "jira_url, username, and api_token are required"})
		return
	}

	// Test connection first — fail fast before storing
	if err := h.jiraClient.TestConnection(r.Context(), req.JiraURL, req.Username, req.APIToken); err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "JIRA_CONNECTION_FAILED", "message": "Cannot connect to JIRA: " + err.Error()})
		return
	}

	// Encrypt token at rest
	encToken, err := h.crypto.Encrypt([]byte(req.APIToken))
	if err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": "Encryption failed"})
		return
	}

	cfg := &jiraconf.JIRAConfig{
		ID:          uuid.New(),
		ProductID:   uuid.Nil, // global config
		URL:         req.JiraURL,
		ProjectKey:  req.ProjectKey,
		Username:    req.Username,
		PasswordEnc: encToken,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := h.configRepo.Save(r.Context(), cfg); err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": "Failed to save configuration"})
		return
	}

	respondJSONConfig(w, 201, map[string]interface{}{
		"id":          cfg.ID,
		"jira_url":    cfg.URL,
		"project_key": cfg.ProjectKey,
		"webhook_url": h.platformURL + "/api/v1/jira/webhook",
		"is_active":   cfg.IsActive,
		"created_at":  cfg.CreatedAt.Format(time.RFC3339),
	})
}

// POST /jira/config/test
func (h *ConfigHandler) TestConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configRepo.FindByProduct(r.Context(), uuid.Nil)
	if err != nil || cfg == nil {
		respondJSONConfig(w, 404, map[string]string{"error": "NOT_FOUND", "message": "No JIRA configuration found. Create one first."})
		return
	}

	// Decrypt token for test
	plainToken, err := h.crypto.Decrypt(cfg.PasswordEnc)
	if err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": "Failed to decrypt token"})
		return
	}

	start := time.Now()
	jiraVersion, projectFound, err := h.jiraClient.GetVersionAndProject(
		r.Context(), cfg.URL, cfg.Username, string(plainToken), cfg.ProjectKey,
	)
	elapsed := time.Since(start)

	if err != nil {
		respondJSONConfig(w, 200, map[string]interface{}{
			"success":          false,
			"error":            err.Error(),
			"response_time_ms": elapsed.Milliseconds(),
		})
		return
	}

	respondJSONConfig(w, 200, map[string]interface{}{
		"success":          true,
		"jira_version":     jiraVersion,
		"project_found":    projectFound,
		"response_time_ms": elapsed.Milliseconds(),
	})
}

// POST /jira/config/bulk
func (h *ConfigHandler) BulkCreateJiraConfigs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Configurations []struct {
			JiraURL    string `json:"jira_url"`
			ProjectKey string `json:"project_key"`
			Username   string `json:"username"`
			APIToken   string `json:"api_token"`
		} `json:"configurations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "Invalid request body"})
		return
	}

	results := make([]map[string]any, 0, len(req.Configurations))
	created := 0
	for _, cfg := range req.Configurations {
		// Encrypt token at rest
		encToken, err := h.crypto.Encrypt([]byte(cfg.APIToken))
		if err != nil {
			results = append(results, map[string]any{
				"project_key": cfg.ProjectKey, "status": "error", "message": "encryption failed",
			})
			continue
		}

		record := &jiraconf.JIRAConfig{
			ID:          uuid.New(),
			ProductID:   uuid.Nil,
			URL:         cfg.JiraURL,
			ProjectKey:  cfg.ProjectKey,
			Username:    cfg.Username,
			PasswordEnc: encToken,
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}

		if err := h.configRepo.Save(r.Context(), record); err != nil {
			results = append(results, map[string]any{"project_key": cfg.ProjectKey, "status": "error", "message": err.Error()})
		} else {
			results = append(results, map[string]any{"project_key": cfg.ProjectKey, "status": "created", "id": record.ID})
			created++
		}
	}

	respondJSONConfig(w, http.StatusMultiStatus, map[string]any{
		"created_count": created,
		"failed_count":  len(results) - created,
		"results":       results,
	})
}

// ── v2 CRUD handlers for /api/v2/jira-configurations ─────────────────────────
// TASK-007 FIX: gateway routes v2 requests here but handlers were missing.

// ListConfigs handles GET /api/v2/jira-configurations
func (h *ConfigHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	cfgs, total, err := h.configRepo.List(r.Context(), 100, 0)
	if err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	if cfgs == nil {
		cfgs = []*jiraconf.JIRAConfig{}
	}
	respondJSONConfig(w, 200, map[string]interface{}{
		"data":  cfgs,
		"total": total,
	})
}

// GetConfigByID handles GET /api/v2/jira-configurations/{id}
func (h *ConfigHandler) GetConfigByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "id is required"})
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "invalid UUID"})
		return
	}
	cfg, err := h.configRepo.FindByID(r.Context(), uid)
	if err != nil || cfg == nil {
		respondJSONConfig(w, 404, map[string]string{"error": "NOT_FOUND", "message": "configuration not found"})
		return
	}
	respondJSONConfig(w, 200, cfg)
}

// UpdateConfigByID handles PUT /api/v2/jira-configurations/{id}
func (h *ConfigHandler) UpdateConfigByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "id is required"})
		return
	}
	var req struct {
		JiraURL    string `json:"jira_url"`
		ProjectKey string `json:"project_key"`
		Username   string `json:"username"`
		APIToken   string `json:"api_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid request body"})
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "invalid UUID"})
		return
	}
	encToken, err := h.crypto.Encrypt([]byte(req.APIToken))
	if err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": "encryption failed"})
		return
	}
	cfg := &jiraconf.JIRAConfig{
		ID:          uid,
		ProductID:   uuid.Nil,
		URL:         req.JiraURL,
		ProjectKey:  req.ProjectKey,
		Username:    req.Username,
		PasswordEnc: encToken,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := h.configRepo.Save(r.Context(), cfg); err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	respondJSONConfig(w, 200, cfg)
}

// DeleteConfigByID handles DELETE /api/v2/jira-configurations/{id}
func (h *ConfigHandler) DeleteConfigByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "id is required"})
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "invalid UUID"})
		return
	}
	if err := h.configRepo.Delete(r.Context(), uid); err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── JIRA Issues handlers (TASK-HC-013: real DB-backed CRUD) ──────────────────

// ListIssues handles GET /api/v2/jira-issues
// [FIX TASK-HC-013] Reads from jira_issue_mappings table — no longer a stub.
func (h *ConfigHandler) ListIssues(w http.ResponseWriter, r *http.Request) {
	if h.issueRepo == nil {
		respondJSONConfig(w, 200, map[string]interface{}{"data": []interface{}{}, "total": 0})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	issues, total, err := h.issueRepo.List(r.Context(), page, pageSize)
	if err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	if issues == nil {
		issues = []*pginfra.IssueMappingRecord{}
	}
	respondJSONConfig(w, 200, map[string]interface{}{"data": issues, "total": total})
}

// CreateIssue handles POST /api/v2/jira-issues
// [FIX TASK-HC-013] Persists finding→JIRA link in jira_issue_mappings.
func (h *ConfigHandler) CreateIssue(w http.ResponseWriter, r *http.Request) {
	if h.issueRepo == nil {
		respondJSONConfig(w, http.StatusServiceUnavailable, map[string]string{"error": "NOT_CONFIGURED", "message": "issue repository not available"})
		return
	}
	var req struct {
		FindingID   string `json:"finding_id"`
		JiraID      string `json:"jira_id"`
		JiraKey     string `json:"jira_key"`
		JiraURL     string `json:"jira_url"`
		JiraStatus  string `json:"jira_status"`
		JiraPriority string `json:"jira_priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid request body"})
		return
	}
	findingID, err := uuid.Parse(req.FindingID)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid finding_id UUID"})
		return
	}
	m := &pginfra.IssueMappingRecord{
		FindingID:    findingID,
		JiraID:       req.JiraID,
		JiraKey:      req.JiraKey,
		JiraURL:      req.JiraURL,
		JiraStatus:   req.JiraStatus,
		JiraPriority: req.JiraPriority,
		Synced:       true,
	}
	if err := h.issueRepo.Save(r.Context(), m); err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	respondJSONConfig(w, http.StatusCreated, m)
}

// GetIssueByFinding handles GET /api/v2/jira-issues/{finding_id}
// [FIX TASK-HC-013] Queries jira_issue_mappings by finding UUID.
func (h *ConfigHandler) GetIssueByFinding(w http.ResponseWriter, r *http.Request) {
	if h.issueRepo == nil {
		respondJSONConfig(w, 404, map[string]string{"error": "NOT_FOUND", "message": "no JIRA issue linked to this finding"})
		return
	}
	findingIDStr := r.PathValue("finding_id")
	findingID, err := uuid.Parse(findingIDStr)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "invalid finding_id"})
		return
	}
	mapping, err := h.issueRepo.FindByFindingID(r.Context(), findingID)
	if err != nil {
		respondJSONConfig(w, 404, map[string]string{"error": "NOT_FOUND", "message": "no JIRA issue linked to this finding"})
		return
	}
	respondJSONConfig(w, 200, mapping)
}

// DeleteIssue handles DELETE /api/v2/jira-issues/{id}
// [FIX TASK-HC-013] Deletes from jira_issue_mappings by UUID.
func (h *ConfigHandler) DeleteIssue(w http.ResponseWriter, r *http.Request) {
	if h.issueRepo == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondJSONConfig(w, 400, map[string]string{"error": "BAD_REQUEST", "message": "invalid id"})
		return
	}
	if err := h.issueRepo.Delete(r.Context(), id); err != nil {
		respondJSONConfig(w, 500, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func maskToken(encryptedToken string) string {
	if len(encryptedToken) < 16 {
		return "***"
	}
	// Show "ATxx...xxxx" pattern — only first 4 and last 4 chars
	return encryptedToken[:4] + "..." + encryptedToken[len(encryptedToken)-4:]
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func respondJSONConfig(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func (h *ConfigHandler) SetIssueRepo(repo *pginfra.IssueMappingRepo) {
	h.issueRepo = repo
}
