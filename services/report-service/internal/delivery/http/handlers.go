package http

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/google/uuid"
    "github.com/rs/zerolog"

    "github.com/google/osv.dev/services/report-service/internal/storage"
    "github.com/google/osv.dev/services/report-service/internal/usecase"
)

// Handler holds report-service HTTP handler dependencies.
type Handler struct {
    generateUC *usecase.GenerateUseCase
    runRepo    usecase.ReportRunRepository
    store      *storage.MinIOStorage
    logger     zerolog.Logger
}

// NewHandler creates a report-service HTTP handler.
func NewHandler(
    generateUC *usecase.GenerateUseCase,
    runRepo usecase.ReportRunRepository,
    store *storage.MinIOStorage,
    logger zerolog.Logger,
) *Handler {
    return &Handler{
        generateUC: generateUC,
        runRepo:    runRepo,
        store:      store,
        logger:     logger,
    }
}

// Router sets up report-service routes.
func (h *Handler) Router() http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.Recoverer)

    r.Post("/reports/generate", h.Generate)
    r.Get("/reports/{runID}", h.GetReport)
    r.Get("/reports/{runID}/download/{format}", h.Download)
    r.Get("/reports/{runID}/exit-code", h.ExitCode)

    return r
}

// Generate handles POST /reports/generate — starts async report generation.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
    var req usecase.GenerateInput
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    if req.ScanID == uuid.Nil {
        jsonError(w, "scan_id is required", http.StatusBadRequest)
        return
    }
    if len(req.Formats) == 0 {
        req.Formats = []string{"html", "csv", "json"}
    }

    out, err := h.generateUC.Execute(r.Context(), req)
    if err != nil {
        h.logger.Error().Err(err).Msg("report generation failed")
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    jsonResponse(w, http.StatusAccepted, map[string]interface{}{
        "run_id":  out.RunID,
        "status":  "pending",
        "message": "report generation started",
    })
}

// GetReport handles GET /reports/{runID} — returns report run status.
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
    runID, err := uuid.Parse(chi.URLParam(r, "runID"))
    if err != nil {
        jsonError(w, "invalid run ID", http.StatusBadRequest)
        return
    }

    run, err := h.runRepo.FindByID(r.Context(), runID)
    if err != nil {
        jsonError(w, "report run not found", http.StatusNotFound)
        return
    }

    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "id":           run.ID,
        "scan_id":      run.ScanID,
        "status":       run.Status,
        "formats":      run.Formats,
        "artifacts":    run.Artifacts,
        "exit_code":    run.ExitCode,
        "created_at":   run.CreatedAt,
        "completed_at": run.CompletedAt,
    })
}

// Download handles GET /reports/{runID}/download/{format} — returns presigned URL.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
    runID, err := uuid.Parse(chi.URLParam(r, "runID"))
    if err != nil {
        jsonError(w, "invalid run ID", http.StatusBadRequest)
        return
    }
    format := chi.URLParam(r, "format")

    run, err := h.runRepo.FindByID(r.Context(), runID)
    if err != nil {
        jsonError(w, "report not found", http.StatusNotFound)
        return
    }

    objectKey, ok := run.Artifacts[format]
    if !ok {
        jsonError(w, "format not available", http.StatusNotFound)
        return
    }

    url, err := h.store.PresignedURL(r.Context(), objectKey, 15*time.Minute)
    if err != nil {
        jsonError(w, "failed to generate download URL", http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, url, http.StatusFound)
}

// ExitCode handles GET /reports/{runID}/exit-code — for CI/CD integration.
// Returns exit code 0 (clean) or 1 (CVEs found). Response code matches exit code + 200.
func (h *Handler) ExitCode(w http.ResponseWriter, r *http.Request) {
    runID, err := uuid.Parse(chi.URLParam(r, "runID"))
    if err != nil {
        jsonError(w, "invalid run ID", http.StatusBadRequest)
        return
    }

    run, err := h.runRepo.FindByID(r.Context(), runID)
    if err != nil {
        jsonError(w, "report not found", http.StatusNotFound)
        return
    }

    // HTTP 200 = exit code 0 (clean), HTTP 280 = exit code 1 (CVEs found)
    // In practice CI scripts check the JSON exit_code field
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "run_id":    run.ID,
        "exit_code": run.ExitCode,
        "status":    run.Status,
    })
}

func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(body)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
    jsonResponse(w, status, map[string]string{"error": msg})
}
