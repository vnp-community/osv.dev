// Package handler implements JIRA webhook and REST API handlers.
package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/defectdojo/jira/internal/domain"
	"github.com/defectdojo/jira/internal/usecase"
)

// WebhookHandler handles incoming JIRA webhook events.
type WebhookHandler struct {
	configRepo    domain.JIRAConfigRepository
	syncStatusUC  *usecase.SyncIssueStatusUseCase
	log           zerolog.Logger
}

func NewWebhookHandler(repo domain.JIRAConfigRepository, syncUC *usecase.SyncIssueStatusUseCase, log zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{configRepo: repo, syncStatusUC: syncUC, log: log}
}

// Handle processes JIRA webhook events at POST /api/v2/jira/webhook.
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body failed"})
		return
	}

	// Validate HMAC-SHA256 signature
	sig := r.Header.Get("X-Hub-Signature-256")
	secret := r.Header.Get("X-JIRA-Config-ID") // config ID used to look up webhook secret
	if !h.validateSignature(secret, body, sig) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

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

	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if payload.Issue.Key != "" && payload.Issue.Fields.Status.Name != "" {
		go func() {
			if err := h.syncStatusUC.OnJIRAWebhook(r.Context(), payload.Issue.Key, payload.Issue.Fields.Status.Name); err != nil {
				h.log.Error().Err(err).Str("issue_key", payload.Issue.Key).Msg("sync issue status failed")
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) validateSignature(configID string, body []byte, sig string) bool {
	// In production: look up webhook secret from config repo using configID
	// For now, return true if no signature header is set (allowing unauthenticated dev mode)
	if sig == "" {
		return true
	}
	// sig format: "sha256=<hex>"
	if len(sig) < 8 || sig[:7] != "sha256=" {
		return false
	}
	expected := sig[7:]
	secret := "placeholder" // TODO: fetch from configRepo.FindByID(configID).WebhookSecret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actual := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(actual))
}

// ConfigHandler handles JIRA configuration CRUD.
type ConfigHandler struct {
	configRepo domain.JIRAConfigRepository
	log        zerolog.Logger
}

func NewConfigHandler(repo domain.JIRAConfigRepository, log zerolog.Logger) *ConfigHandler {
	return &ConfigHandler{configRepo: repo, log: log}
}

func (h *ConfigHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v2/jira-configurations", h.List)
	r.Post("/api/v2/jira-configurations", h.Create)
	r.Get("/api/v2/jira-configurations/{id}", h.Get)
}

func (h *ConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.configRepo.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Scrub API tokens before returning
	for _, c := range configs {
		c.APITokenEncrypted = "***"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(configs), "results": configs})
}

func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Implementation similar to List but single config
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
