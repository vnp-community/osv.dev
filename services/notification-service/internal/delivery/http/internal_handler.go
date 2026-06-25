package http

import (
	"encoding/json"
	"net/http"

	"github.com/osv/notification-service/internal/usecase"
)

type InternalHandler struct {
	dispatcher *usecase.AlertDispatcher
}

func NewInternalHandler(dispatcher *usecase.AlertDispatcher) *InternalHandler {
	return &InternalHandler{dispatcher: dispatcher}
}

func (h *InternalHandler) ReceiveCVEEvents(w http.ResponseWriter, r *http.Request) {
	// Verify X-Internal-Service header
	if r.Header.Get("X-Internal-Service") != "data-service" {
		respondError(w, 403, "internal only")
		return
	}

	var payload struct {
		Events []usecase.CVEEvent `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, 400, "invalid request body")
		return
	}

	for _, ev := range payload.Events {
		h.dispatcher.Dispatch(r.Context(), ev)
	}
	respondJSON(w, 200, map[string]string{"status": "dispatched"})
}
