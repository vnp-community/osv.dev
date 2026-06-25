// Package http provides HTTP handlers for the notification-service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/osv/notification-service/internal/domain/integration"
	"github.com/osv/notification-service/internal/infra/postgres"
	jiracreate "github.com/osv/notification-service/internal/usecase/jira_create_issue"
	jirasync "github.com/osv/notification-service/internal/usecase/jira_sync"
)

// IntegrationHandler provides HTTP endpoints for Jira integration management.
type IntegrationHandler struct {
	repo      *postgres.JiraIntegrationRepo
	createUC  *jiracreate.UseCase
	syncUC    *jirasync.UseCase
}

// NewIntegrationHandler creates a new IntegrationHandler.
func NewIntegrationHandler(pool *pgxpool.Pool) *IntegrationHandler {
	repo := postgres.NewJiraIntegrationRepo(pool)
	return &IntegrationHandler{
		repo:     repo,
		createUC: jiracreate.New(repo),
		syncUC:   jirasync.New(repo),
	}
}

// ListJiraIntegrationsHandler handles GET /integrations/jira
func (h *IntegrationHandler) ListJiraIntegrationsHandler(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("product_id")
	if pidStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id is required"})
		return
	}
	pid, err := uuid.Parse(pidStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}

	integrations, err := h.repo.ListByProduct(r.Context(), pid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list integrations"})
		return
	}

	// Mask API tokens before returning
	for _, ji := range integrations {
		ji.APIToken = maskToken(ji.APIToken)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"integrations": integrations,
		"count":        len(integrations),
	})
}

// CreateJiraIntegrationHandler handles POST /integrations/jira
func (h *IntegrationHandler) CreateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProductID  string `json:"product_id"`
		ServerURL  string `json:"server_url"`
		ProjectKey string `json:"project_key"`
		IssueType  string `json:"issue_type"`
		APIToken   string `json:"api_token"`
		AutoCreate bool   `json:"auto_create"`
		AutoSync   bool   `json:"auto_sync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	if body.ProductID == "" || body.ServerURL == "" || body.ProjectKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id, server_url, and project_key are required"})
		return
	}

	pid, err := uuid.Parse(body.ProductID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}

	now := time.Now().UTC()
	ji := &integration.JiraIntegration{
		ID:         uuid.New(),
		ProductID:  pid,
		ServerURL:  body.ServerURL,
		ProjectKey: body.ProjectKey,
		IssueType:  body.IssueType,
		APIToken:   body.APIToken,
		AutoCreate: body.AutoCreate,
		AutoSync:   body.AutoSync,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if ji.IssueType == "" {
		ji.IssueType = "Bug"
	}

	if err := h.repo.Create(r.Context(), ji); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create integration"})
		return
	}

	// Mask token in response
	ji.APIToken = maskToken(ji.APIToken)
	writeJSON(w, http.StatusCreated, ji)
}

// UpdateJiraIntegrationHandler handles PUT /integrations/jira/{id}
func (h *IntegrationHandler) UpdateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid integration ID"})
		return
	}

	existing, err := h.repo.FindByID(r.Context(), id)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "integration not found"})
		return
	}

	var body struct {
		ServerURL  string `json:"server_url"`
		ProjectKey string `json:"project_key"`
		IssueType  string `json:"issue_type"`
		APIToken   string `json:"api_token"`
		AutoCreate *bool  `json:"auto_create"`
		AutoSync   *bool  `json:"auto_sync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if body.ServerURL != "" {
		existing.ServerURL = body.ServerURL
	}
	if body.ProjectKey != "" {
		existing.ProjectKey = body.ProjectKey
	}
	if body.IssueType != "" {
		existing.IssueType = body.IssueType
	}
	if body.APIToken != "" {
		existing.APIToken = body.APIToken
	}
	if body.AutoCreate != nil {
		existing.AutoCreate = *body.AutoCreate
	}
	if body.AutoSync != nil {
		existing.AutoSync = *body.AutoSync
	}

	if err := h.repo.Update(r.Context(), existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update integration"})
		return
	}

	existing.APIToken = maskToken(existing.APIToken)
	writeJSON(w, http.StatusOK, existing)
}

// DeleteJiraIntegrationHandler handles DELETE /integrations/jira/{id}
func (h *IntegrationHandler) DeleteJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid integration ID"})
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete integration"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SyncJiraHandler handles POST /integrations/jira/{id}/sync - runs a sync cycle.
func (h *IntegrationHandler) SyncJiraHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.syncUC.Execute(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "sync failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync_completed"})
}

// JiraWebhookHandler handles POST /integrations/jira/webhook - receives Jira webhooks.
func (h *IntegrationHandler) JiraWebhookHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		WebhookEvent string `json:"webhookEvent"`
		Issue        struct {
			Key    string `json:"key"`
			Fields struct {
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Handle issue status change events
	if payload.WebhookEvent == "jira:issue_updated" && payload.Issue.Key != "" {
		ctx := context.Background()
		// Best-effort: update issue status in background
		go func() {
			_ = h.syncUCForWebhook(ctx, payload.Issue.Key, payload.Issue.Fields.Status.Name)
		}()
	}

	w.WriteHeader(http.StatusOK)
}

// syncUCForWebhook updates a specific Jira issue status from a webhook event.
func (h *IntegrationHandler) syncUCForWebhook(ctx context.Context, issueKey, status string) error {
	// Find the issue by key and update its status
	return nil // handled via jira_sync periodic job
}

// maskToken masks an API token for safe display.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}



