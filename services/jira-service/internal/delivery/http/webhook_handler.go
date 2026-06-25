// Package http provides the JIRA webhook handler for jira-service.
package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	jiraconf "github.com/osv/jira-service/internal/domain/jiraconfig"
	jirasync "github.com/osv/jira-service/internal/usecase/sync"
)

// ConfigRepository provides JIRA configs by ID for webhook verification.
type ConfigRepository interface {
	FindByID(ctx context.Context, id string) (*jiraconf.JIRAConfig, error)
}

// WebhookHandler handles POST /webhooks/jira/{config_id}.
type WebhookHandler struct {
	configRepo ConfigRepository
	pullStatus *jirasync.PullStatusUseCase
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(cr ConfigRepository, ps *jirasync.PullStatusUseCase) *WebhookHandler {
	return &WebhookHandler{configRepo: cr, pullStatus: ps}
}

// ─── JIRA Webhook payload types ────────────────────────────────────────────────

// JIRAWebhookPayload is the incoming JIRA webhook event.
type JIRAWebhookPayload struct {
	WebhookEvent string    `json:"webhookEvent"` // "jira:issue_updated", "comment_created"
	Issue        JIRAIssue `json:"issue"`
	ChangeLog    ChangeLog `json:"changelog"`
	Comment      *Comment  `json:"comment"`
}

// JIRAIssue holds the minimal issue data from JIRA webhook.
type JIRAIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"` // e.g. "PROJ-123"
	Fields struct {
		Status struct {
			Name string `json:"name"` // "Done", "In Progress", etc.
		} `json:"status"`
		Summary string `json:"summary"`
	} `json:"fields"`
}

// ChangeLog holds the list of field changes in the webhook event.
type ChangeLog struct {
	Items []ChangeLogItem `json:"items"`
}

// ChangeLogItem describes a single field change.
type ChangeLogItem struct {
	Field   string `json:"field"`
	FromStr string `json:"fromString"`
	ToStr   string `json:"toString"`
}

// Comment is a JIRA comment (for comment_created/comment_updated events).
type Comment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

// Handle processes POST /webhooks/jira/{config_id}.
// No JWT auth — uses HMAC signature verification instead.
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	configID := r.PathValue("config_id")

	// 1. Look up JIRA config for webhook secret
	cfg, err := h.configRepo.FindByID(r.Context(), configID)
	if err != nil || cfg == nil {
		http.Error(w, `{"detail":"Configuration not found"}`, http.StatusNotFound)
		return
	}

	// 2. Read body (limit to 10MB)
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, `{"detail":"Failed to read request body"}`, http.StatusBadRequest)
		return
	}

	// 3. Verify HMAC signature (X-Hub-Signature: sha256=<hex>)
	if cfg.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature")
		if !verifyHMAC(body, cfg.WebhookSecret, signature) {
			http.Error(w, `{"detail":"Invalid signature"}`, http.StatusUnauthorized)
			return
		}
	}

	// 4. Respond 202 Accepted immediately (async processing)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`)) //nolint:errcheck

	// 5. Process async in background goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		h.process(ctx, body)
	}()
}

// process deserializes and handles the JIRA webhook payload.
func (h *WebhookHandler) process(ctx context.Context, body []byte) {
	var payload JIRAWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}

	switch payload.WebhookEvent {
	case "jira:issue_updated":
		// Check for status field change in changelog
		for _, item := range payload.ChangeLog.Items {
			if item.Field == "status" && item.ToStr != "" {
				_ = h.pullStatus.Execute(ctx, &jirasync.PullStatusInput{
					JIRAKey:       payload.Issue.Key,
					JIRAID:        payload.Issue.ID,
					NewJIRAStatus: item.ToStr,
				})
			}
		}

	case "comment_created", "comment_updated":
		if payload.Comment != nil {
			h.pullStatus.SyncComment(ctx, payload.Issue.Key, payload.Comment.Body)
		}
	}
}

// verifyHMAC verifies a "sha256=<hex>" HMAC signature against the body.
func verifyHMAC(body []byte, secret, signature string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	expectedHex := strings.TrimPrefix(signature, prefix)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actual := hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison
	return hmac.Equal([]byte(actual), []byte(expectedHex))
}
