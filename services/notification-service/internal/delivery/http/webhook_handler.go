package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/notification-service/internal/domain/repository"
	entity "github.com/osv/notification-service/internal/domain/webhook"
	"github.com/osv/notification-service/internal/usecase"
	"github.com/osv/shared/pkg/middleware/auth"
)

// WebhookDTO is the JSON response for Webhook listing.
type WebhookDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    string   `json:"secret"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"created_at"`
}

type WebhookHandler struct {
	registerUC  *usecase.RegisterWebhookUseCase
	deliverer   *usecase.WebhookDeliverer
	webhookRepo repository.WebhookRepository
}

func NewWebhookHandler(registerUC *usecase.RegisterWebhookUseCase, deliverer *usecase.WebhookDeliverer, webhookRepo repository.WebhookRepository) *WebhookHandler {
	return &WebhookHandler{
		registerUC:  registerUC,
		deliverer:   deliverer,
		webhookRepo: webhookRepo,
	}
}

// POST /api/v2/webhooks
// Body: {"url":"https://example.com/webhook","events":["kev.new","cve.new.critical"],"secret":"optional"}
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	var req struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Secret string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, 400, "url is required")
		return
	}
	events := make([]entity.EventType, len(req.Events))
	for i, e := range req.Events {
		events[i] = entity.EventType(e)
	}

	wh, err := h.registerUC.Execute(r.Context(), usecase.RegisterWebhookInput{
		URL:     req.URL,
		Events:  events,
		Secret:  req.Secret,
		OwnerID: claims.UserID,
	})

	switch {
	case errors.Is(err, usecase.ErrInsecureURL):
		respondError(w, 400, "webhook URL must use HTTPS")
	case errors.Is(err, usecase.ErrSSRFBlocked):
		respondError(w, 400, "webhook URL points to private/internal network (SSRF protection)")
	case errors.Is(err, usecase.ErrPingFailed):
		respondError(w, 400, "webhook URL did not respond to ping test")
	case errors.Is(err, usecase.ErrUnresolvable):
		respondError(w, 400, "webhook URL hostname cannot be resolved")
	case err != nil:
		respondError(w, 500, "failed to register webhook")
	default:
		dto := WebhookDTO{
			ID:        wh.ID(),
			Name:      wh.URL(),
			URL:       wh.URL(),
			Active:    wh.IsActive(),
			CreatedAt: wh.CreatedAt().Format(time.RFC3339),
		}
		whEvents := wh.Events()
		dto.Events = make([]string, 0, len(whEvents))
		for _, e := range whEvents {
			dto.Events = append(dto.Events, string(e))
		}
		respondJSON(w, 201, dto)
	}
}

// GET /api/v2/webhooks — list owner's webhooks
// FIX: returns JSON array directly (spec), not {"webhooks": [...]} wrapper.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	webhooks, err := h.webhookRepo.FindByOwner(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, 500, "failed to list webhooks")
		return
	}
	if webhooks == nil {
		respondJSON(w, 200, []WebhookDTO{})
		return
	}

	dtos := make([]WebhookDTO, len(webhooks))
	for i, wh := range webhooks {
		whEvents := wh.Events()
		// FIX: never nil — JSON must be [] not null
		events := make([]string, 0, len(whEvents))
		for _, e := range whEvents {
			events = append(events, string(e))
		}
		secret := wh.Secret()
		if secret != "" {
			secret = "***"
		}
		dtos[i] = WebhookDTO{
			ID:        wh.ID(),
			Name:      wh.URL(), // Fallback to URL since domain doesn't have Name
			URL:       wh.URL(),
			Events:    events,
			Secret:    secret,
			Active:    wh.IsActive(),
			CreatedAt: wh.CreatedAt().Format(time.RFC3339),
		}
	}
	respondJSON(w, 200, dtos)
}


// DELETE /api/v2/webhooks/{id} — revoke webhook
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.webhookRepo.Delete(r.Context(), id, claims.UserID); err != nil {
		respondError(w, 404, "webhook not found")
		return
	}
	w.WriteHeader(204)
}

// POST /api/v2/webhooks/bulk
func (h *WebhookHandler) BulkCreateWebhooks(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	var req struct {
		Webhooks []struct {
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Secret string   `json:"secret"`
		} `json:"webhooks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}

	type WebhookResult struct {
		URL     string     `json:"url"`
		Status  string     `json:"status"`
		ID      *uuid.UUID `json:"id,omitempty"`
		Secret  string     `json:"secret,omitempty"` // HMAC secret — one time
		Message string     `json:"message,omitempty"`
	}

	results := make([]WebhookResult, 0, len(req.Webhooks))
	created := 0
	for _, whInput := range req.Webhooks {
		events := make([]entity.EventType, len(whInput.Events))
		for i, e := range whInput.Events {
			events[i] = entity.EventType(e)
		}

		wh, err := h.registerUC.Execute(r.Context(), usecase.RegisterWebhookInput{
			URL:     whInput.URL,
			Events:  events,
			Secret:  whInput.Secret,
			OwnerID: claims.UserID,
		})

		if err != nil {
			msg := err.Error()
			if errors.Is(err, usecase.ErrSSRFBlocked) {
				msg = "private IP not allowed"
			}
			results = append(results, WebhookResult{URL: whInput.URL, Status: "error", Message: msg})
		} else {
			uid, _ := uuid.Parse(wh.ID()) // Assume wh.ID() returns string, we try to parse it if needed.
			results = append(results, WebhookResult{URL: whInput.URL, Status: "created", ID: &uid, Secret: wh.Secret()})
			created++
		}
	}
	respondJSON(w, http.StatusMultiStatus, map[string]any{"created_count": created, "results": results})
}

// GET /api/v2/webhooks/{id}/deliveries — delivery history
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.OVSClaimsFromContext(r.Context())
	if !ok || claims == nil {
		respondError(w, 401, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	limit := parseInt(r.URL.Query().Get("limit"), 50)

	deliveries, err := h.webhookRepo.ListDeliveries(r.Context(), id, limit)
	if err != nil {
		respondError(w, 500, "failed to list deliveries")
		return
	}
	respondJSON(w, 200, map[string]interface{}{"deliveries": deliveries})
}

// POST /api/v2/webhooks/{id}/test — send test ping event
func (h *WebhookHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	// The repo in this module might take string or uuid, let's assume it takes string based on ListDeliveries
	wh, err := h.webhookRepo.FindByID(r.Context(), idStr, "")
	if err != nil {
		respondError(w, 404, "Webhook not found")
		return
	}

	// Build test payload
	testPayload := map[string]interface{}{
		"event_type": "test",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"message":    "This is a test notification from OSV Platform",
		"webhook_id": idStr,
		"platform":   "OSV Platform",
	}

	payloadBytes, _ := json.Marshal(testPayload)

	// Compute HMAC signature if secret configured
	var signature string
	if wh.Secret() != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret()))
		mac.Write(payloadBytes)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	// Send test HTTP request
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", wh.URL(), bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OSV-Platform/1.0")
	req.Header.Set("X-OSV-Event", "test")
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	deliveryID := "dlv_test_" + uuid.New().String()[:8]

	if err != nil {
		respondJSON(w, 200, map[string]interface{}{
			"delivery_id":      deliveryID,
			"status":           "failed",
			"error":            err.Error(),
			"response_time_ms": elapsed.Milliseconds(),
		})
		return
	}
	defer resp.Body.Close()

	respondJSON(w, 200, map[string]interface{}{
		"delivery_id":      deliveryID,
		"status":           "success",
		"response_code":    resp.StatusCode,
		"response_time_ms": elapsed.Milliseconds(),
	})
}

func respondJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func respondError(w http.ResponseWriter, code int, msg string) {
	respondJSON(w, code, map[string]string{"error": msg})
}

func parseInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
