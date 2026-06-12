package http

import "net/http"

// IntegrationHandler handles Jira integration REST endpoints
// GET    /integrations/jira
// POST   /integrations/jira
// PUT    /integrations/jira/{id}
// DELETE /integrations/jira/{id}
// POST   /integrations/jira/{id}/sync
// POST   /integrations/jira/webhook  (receive Jira webhooks)

func ListJiraIntegrationsHandler(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(http.StatusOK) }
func CreateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) }
func UpdateJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func DeleteJiraIntegrationHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }
func SyncJiraHandler(w http.ResponseWriter, r *http.Request)              { w.WriteHeader(http.StatusAccepted) }
func JiraWebhookHandler(w http.ResponseWriter, r *http.Request)           { w.WriteHeader(http.StatusOK) }
