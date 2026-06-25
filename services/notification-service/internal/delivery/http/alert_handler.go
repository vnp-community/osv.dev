package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID         uuid.UUID
	Type       string
	Title      string
	Message    string
	Severity   string
	IsRead     bool
	EntityType string
	EntityID   string
	CreatedAt  time.Time
	ReadAt     *time.Time
}

type AlertRepository interface {
	ListByUser(ctx context.Context, userID, isRead string, limit, offset int) ([]*Alert, int, int, error)
	CountUnread(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, alertID, userID string) error
	MarkAllRead(ctx context.Context, userID string) (int, error)
}

type AlertsHandler struct {
	alertRepo AlertRepository
}

func NewAlertsHandler(r AlertRepository) *AlertsHandler {
	return &AlertsHandler{alertRepo: r}
}

// GET /v2/notification-alerts → alias /notifications
func (h *AlertsHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	// Support both 'read' and 'is_read' query params (spec uses is_read, tests use read)
	isRead := r.URL.Query().Get("is_read")
	if isRead == "" {
		isRead = r.URL.Query().Get("read") // alias
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	alerts, total, unread, err := h.alertRepo.ListByUser(r.Context(), userID, isRead, limit, offset)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"notifications": mapAlerts(alerts),
		"total":         total,
		"unread_count":  unread,
		"page":          offset/limit + 1,
		"page_size":     limit,
	})
}

// PATCH /notifications/{id}/read
func (h *AlertsHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	alertID := r.PathValue("id")
	userID := r.Header.Get("X-User-ID")

	if err := h.alertRepo.MarkRead(r.Context(), alertID, userID); err != nil {
		respondError(w, 404, "Notification not found")
		return
	}
	respondJSON(w, 200, map[string]interface{}{"id": alertID, "is_read": true})
}

// POST /notifications/mark-all-read
func (h *AlertsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	count, err := h.alertRepo.MarkAllRead(r.Context(), userID)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, 200, map[string]interface{}{"marked_count": count})
}

// GET /notifications/unread-count
func (h *AlertsHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	count, err := h.alertRepo.CountUnread(r.Context(), userID)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	respondJSON(w, 200, map[string]interface{}{"unread_count": count})
}

// NotificationDTO — response shape
type NotificationDTO struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`        // "kev.new" | "finding.sla.breached" | ...
	Title      string  `json:"title"`
	Message    string  `json:"message"`
	Severity   string  `json:"severity"`    // "Critical" | "High" | "Info"
	Read       bool    `json:"read"`        // alias for is_read (test_admin_notifications expects this key)
	IsRead     bool    `json:"is_read"`
	EntityType string  `json:"entity_type"` // "cve" | "finding" | "scan"
	EntityID   string  `json:"entity_id"`
	CreatedAt  string  `json:"created_at"`
	ReadAt     *string `json:"read_at"`
}

func mapAlerts(alerts []*Alert) []NotificationDTO {
	dtos := make([]NotificationDTO, len(alerts))
	for i, a := range alerts {
		var readAt *string
		if a.ReadAt != nil {
			s := a.ReadAt.Format(time.RFC3339)
			readAt = &s
		}
		dtos[i] = NotificationDTO{
			ID:         a.ID.String(),
			Type:       a.Type,
			Title:      a.Title,
			Message:    a.Message,
			Severity:   a.Severity,
			Read:       a.IsRead,
			IsRead:     a.IsRead,
			EntityType: a.EntityType,
			EntityID:   a.EntityID,
			CreatedAt:  a.CreatedAt.Format(time.RFC3339),
			ReadAt:     readAt,
		}
	}
	return dtos
}
