// Package http — member_handler.go
// MemberHandler handles HTTP REST endpoints for product membership management.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/osv/finding-service/internal/domain/member"
	member_uc "github.com/osv/finding-service/internal/usecase/member"
)

// MemberHandler handles product member RBAC management.
type MemberHandler struct {
	addMember    *member_uc.AddProductMemberUseCase
	removeMember *member_uc.RemoveProductMemberUseCase
	listMembers  *member_uc.ListProductMembersUseCase
	checkPerm    *member_uc.CheckProductPermissionUseCase
	log          zerolog.Logger
}

// NewMemberHandler creates a new MemberHandler.
func NewMemberHandler(
	add *member_uc.AddProductMemberUseCase,
	remove *member_uc.RemoveProductMemberUseCase,
	list *member_uc.ListProductMembersUseCase,
	check *member_uc.CheckProductPermissionUseCase,
	log zerolog.Logger,
) *MemberHandler {
	return &MemberHandler{
		addMember:    add,
		removeMember: remove,
		listMembers:  list,
		checkPerm:    check,
		log:          log,
	}
}

// RegisterRoutes registers all member routes on the router.
func (h *MemberHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v2/products/{id}/members", h.List)
	r.Post("/api/v2/products/{id}/members", h.Add)
	r.Delete("/api/v2/products/{id}/members/{uid}", h.Remove)
}

// memberResponse is the JSON representation of a ProductMember.
type memberResponse struct {
	ID        string `json:"id"`
	ProductID string `json:"product"`
	UserID    string `json:"user"`
	Role      string `json:"role"`
	CreatedAt string `json:"date_added"`
}

func toMemberResponse(m *member.ProductMember) *memberResponse {
	return &memberResponse{
		ID:        m.ID.String(),
		ProductID: m.ProductID.String(),
		UserID:    m.UserID.String(),
		Role:      string(m.Role),
		CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// List handles GET /api/v2/products/{id}/members
func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	members, err := h.listMembers.Execute(r.Context(), productID)
	if err != nil {
		h.log.Error().Err(err).Msg("MemberHandler.List")
		respondError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	responses := make([]*memberResponse, 0, len(members))
	for _, m := range members {
		responses = append(responses, toMemberResponse(m))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(responses),
		"results": responses,
	})
}

// Add handles POST /api/v2/products/{id}/members
// Request: {"user_id": "uuid", "role": "Maintainer"}
// Response: 201 Created with member object
func (h *MemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	requesterID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "missing or invalid X-User-ID header")
		return
	}

	var req struct {
		UserID string `json:"user"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	m, err := h.addMember.Execute(r.Context(), member_uc.AddProductMemberInput{
		RequesterUserID: requesterID,
		ProductID:       productID,
		UserID:          userID,
		Role:            member.Role(req.Role),
	})
	if err != nil {
		switch err {
		case member_uc.ErrNotOwner:
			respondError(w, http.StatusForbidden, "only Owner or Maintainer can add members")
		case member_uc.ErrMemberExists:
			respondError(w, http.StatusConflict, "user is already a member of this product")
		case member_uc.ErrInvalidRole:
			respondError(w, http.StatusBadRequest, "invalid role: must be Owner, Maintainer, Writer, API Importer, or Reader")
		default:
			h.log.Error().Err(err).Msg("MemberHandler.Add")
			respondError(w, http.StatusInternalServerError, "failed to add member")
		}
		return
	}
	respondJSON(w, http.StatusCreated, toMemberResponse(m))
}

// Remove handles DELETE /api/v2/products/{id}/members/{uid}
// Only Owner can remove members.
func (h *MemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "uid"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	requesterID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, http.StatusUnauthorized, "missing or invalid X-User-ID header")
		return
	}

	if err := h.removeMember.Execute(r.Context(), member_uc.RemoveProductMemberInput{
		RequesterUserID: requesterID,
		ProductID:       productID,
		UserID:          userID,
	}); err != nil {
		if err == member_uc.ErrNotOwner {
			respondError(w, http.StatusForbidden, "only Owner can remove members")
			return
		}
		h.log.Error().Err(err).Msg("MemberHandler.Remove")
		respondError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
