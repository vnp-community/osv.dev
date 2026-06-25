// Package http — test_handler.go
// TestHandler exposes Test CRUD endpoints for /api/v2/tests.
//
// Routes:
//   GET    /api/v2/tests             → List (scoped by engagement_id)
//   POST   /api/v2/tests             → Create
//   GET    /api/v2/tests/{id}        → Get
//   PUT    /api/v2/tests/{id}        → Update (percent_complete, title, etc.)
//   DELETE /api/v2/tests/{id}        → Delete
package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/osv/finding-service/internal/domain/repository"
	"github.com/osv/finding-service/internal/domain/test"
)

// TestHandler handles HTTP requests for tests.
type TestHandler struct {
	repo repository.TestRepository
	log  zerolog.Logger
}

// NewTestHandler creates a new TestHandler.
func NewTestHandler(repo repository.TestRepository, log zerolog.Logger) *TestHandler {
	return &TestHandler{repo: repo, log: log}
}

// TestResponse is the JSON representation of a Test.
type TestResponse struct {
	ID              string   `json:"id"`
	EngagementID    string   `json:"engagement_id"`
	ScanType        string   `json:"scan_type"`
	Title           string   `json:"title,omitempty"`
	Version         string   `json:"version,omitempty"`
	BuildID         string   `json:"build_id,omitempty"`
	CommitHash      string   `json:"commit_hash,omitempty"`
	BranchTag       string   `json:"branch_tag,omitempty"`
	PercentComplete int      `json:"percent_complete"`
	Tags            []string `json:"tags"`
	CreatedAt       string   `json:"created_at"`
}

// CreateTestRequest is the body for POST /api/v2/tests.
type CreateTestRequest struct {
	EngagementID string   `json:"engagement_id"`
	ScanType     string   `json:"scan_type"`
	Title        string   `json:"title"`
	Version      string   `json:"version"`
	BuildID      string   `json:"build_id"`
	CommitHash   string   `json:"commit_hash"`
	BranchTag    string   `json:"branch_tag"`
	Tags         []string `json:"tags"`
}

// List handles GET /api/v2/tests?engagement_id=...
func (h *TestHandler) List(w http.ResponseWriter, r *http.Request) {
	engIDStr := r.URL.Query().Get("engagement_id")
	if engIDStr == "" {
		respondError(w, http.StatusBadRequest, "engagement_id query param required")
		return
	}
	engID, err := uuid.Parse(engIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid engagement_id")
		return
	}
	tests, err := h.repo.ListByEngagement(r.Context(), engID)
	if err != nil {
		h.log.Error().Err(err).Msg("TestHandler.List")
		respondError(w, http.StatusInternalServerError, "failed to list tests")
		return
	}
	resp := make([]*TestResponse, 0, len(tests))
	for _, t := range tests {
		resp = append(resp, toTestResponse(t))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tests": resp,
		"total": len(resp),
	})
}

// Create handles POST /api/v2/tests.
func (h *TestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ScanType == "" {
		respondError(w, http.StatusBadRequest, "scan_type is required")
		return
	}
	engID, err := uuid.Parse(req.EngagementID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid engagement_id")
		return
	}

	t := test.New(engID, req.ScanType, req.Title)
	t.Version = req.Version
	t.BuildID = req.BuildID
	t.CommitHash = req.CommitHash
	t.BranchTag = req.BranchTag
	if len(req.Tags) > 0 {
		t.Tags = req.Tags
	}

	if err := h.repo.Create(r.Context(), t); err != nil {
		h.log.Error().Err(err).Msg("TestHandler.Create")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, toTestResponse(t))
}

// Get handles GET /api/v2/tests/{id}.
func (h *TestHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test id")
		return
	}
	t, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "test not found")
		return
	}
	respondJSON(w, http.StatusOK, toTestResponse(t))
}

// Delete handles DELETE /api/v2/tests/{id}.
func (h *TestHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid test id")
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("test_id", id.String()).Msg("TestHandler.Delete")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ─────────────────────────────────────────────────────────────────

func toTestResponse(t *test.Test) *TestResponse {
	return &TestResponse{
		ID:              t.ID.String(),
		EngagementID:    t.EngagementID.String(),
		ScanType:        t.ScanType,
		Title:           t.Title,
		Version:         t.Version,
		BuildID:         t.BuildID,
		CommitHash:      t.CommitHash,
		BranchTag:       t.BranchTag,
		PercentComplete: t.PercentComplete,
		Tags:            t.Tags,
		CreatedAt:       t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
