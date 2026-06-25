// Package http provides the HTTP router for the jira-service.
package http

import (
	"net/http"
)

// Router holds all HTTP handlers for jira-service.
type Router struct {
	Webhook *WebhookHandler
	Config  *ConfigHandler
}

// NewRouter creates the chi-style HTTP mux for jira-service.
func NewRouter(r *Router) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"jira-service"}`)) //nolint:errcheck
	})

	// Webhook (HMAC-verified, no JWT auth)
	if r.Webhook != nil {
		mux.HandleFunc("POST /webhooks/jira/{config_id}", r.Webhook.Handle)
	}

	// Config management endpoints (require admin role — enforced by gateway)
	if r.Config != nil {
		mux.HandleFunc("POST /jira/config/bulk", r.Config.BulkCreateJiraConfigs)
		mux.HandleFunc("GET /jira/config", r.Config.GetConfig)
		mux.HandleFunc("POST /jira/config", r.Config.CreateOrUpdateConfig)
		mux.HandleFunc("PUT /jira/config", r.Config.CreateOrUpdateConfig) // TASK-007 FIX: gateway sends PUT for updates
		mux.HandleFunc("POST /jira/config/test", r.Config.TestConfig)

		// API v1 explicit routes
		mux.HandleFunc("GET /api/v1/jira/config", r.Config.GetConfig)
		mux.HandleFunc("POST /api/v1/jira/config", r.Config.CreateOrUpdateConfig)
		mux.HandleFunc("PUT /api/v1/jira/config", r.Config.CreateOrUpdateConfig)
		mux.HandleFunc("POST /api/v1/jira/config/test", r.Config.TestConfig)
		
		// Integrations UI aliases
		mux.HandleFunc("GET /api/v1/integrations/jira", r.Config.GetConfig)
		mux.HandleFunc("PUT /api/v1/integrations/jira", r.Config.CreateOrUpdateConfig)
		mux.HandleFunc("POST /api/v1/integrations/jira", r.Config.CreateOrUpdateConfig)

		// Legacy path compatibility
		mux.HandleFunc("GET /jira-configs", r.Config.GetConfig)

		// TASK-007 FIX: v2 CRUD — gateway proxies /api/v2/jira-configurations/* → here
		// Literal paths BEFORE /{id} wildcard
		mux.HandleFunc("GET /api/v2/jira-configurations", r.Config.ListConfigs)
		mux.HandleFunc("POST /api/v2/jira-configurations", r.Config.CreateOrUpdateConfig)
		mux.HandleFunc("POST /api/v2/jira-configurations/bulk", r.Config.BulkCreateJiraConfigs)
		mux.HandleFunc("GET /api/v2/jira-configurations/{id}", r.Config.GetConfigByID)
		mux.HandleFunc("PUT /api/v2/jira-configurations/{id}", r.Config.UpdateConfigByID)
		mux.HandleFunc("DELETE /api/v2/jira-configurations/{id}", r.Config.DeleteConfigByID)

		// TASK-007 FIX: jira-issues stubs
		mux.HandleFunc("GET /api/v2/jira-issues", r.Config.ListIssues)
		mux.HandleFunc("POST /api/v2/jira-issues", r.Config.CreateIssue)
		mux.HandleFunc("GET /api/v2/jira-issues/{finding_id}", r.Config.GetIssueByFinding)
		mux.HandleFunc("DELETE /api/v2/jira-issues/{id}", r.Config.DeleteIssue)
	}

	return mux
}
