package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	batchenrich "github.com/osv/ai-service/internal/usecase/batch_enrich"
	enrich "github.com/osv/ai-service/internal/usecase/enrich_cve"
	embedding "github.com/osv/ai-service/internal/usecase/generate_embedding"
	"github.com/osv/ai-service/internal/provider"
	triage "github.com/osv/ai-service/internal/usecase/triage_finding"
)

// AIHTTPHandler handles HTTP requests for AI Enrichment Service.
type AIHTTPHandler struct {
	enrichHandler *enrich.Handler
	batchUC       *batchenrich.UseCase
	embeddingUC   *embedding.UseCase
	triageUC      *triage.TriageFindingUseCase
	redis         *redis.Client  // CR-002: for triage queue + enrichment status persistence
	chain         *provider.Chain // P2-01: graceful degradation when LLM unavailable
	log           zerolog.Logger
}

// NewAIHTTPHandler creates a new HTTP handler for AI operations.
func NewAIHTTPHandler(
	enrichHandler *enrich.Handler,
	batchUC *batchenrich.UseCase,
	embeddingUC *embedding.UseCase,
	triageUC *triage.TriageFindingUseCase,
	redisClient *redis.Client,
	log zerolog.Logger,
) *AIHTTPHandler {
	return &AIHTTPHandler{
		enrichHandler: enrichHandler,
		batchUC:       batchUC,
		embeddingUC:   embeddingUC,
		triageUC:      triageUC,
		redis:         redisClient,
		log:           log,
	}
}

// WithChain sets the provider chain for availability checks (P2-01).
func (h *AIHTTPHandler) WithChain(chain *provider.Chain) *AIHTTPHandler {
	h.chain = chain
	return h
}

// isReady returns true if at least one LLM provider is available.
// When false, handlers return graceful degradation responses.
func (h *AIHTTPHandler) isReady() bool {
	if h.chain == nil {
		return false
	}
	return h.chain.HasAvailableProvider()
}

// TriageFinding handles POST /api/v1/ai/triage (legacy — finding_id in body)
func (h *AIHTTPHandler) TriageFinding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FindingID string `json:"finding_id"`
		CVE       string `json:"cve_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "queued",
			"message": "AI provider not available. Request queued for later processing.",
		})
		return
	}

	if h.triageUC == nil {
		http.Error(w, "triage use case not initialized", http.StatusNotImplemented)
		return
	}

	res, err := h.triageUC.Execute(r.Context(), triage.TriageFindingInput{
		FindingID: req.FindingID,
		CVE:       req.CVE,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("TriageFinding error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res) //nolint:errcheck
}

// TriageFindingByID handles POST /api/v1/ai/triage/{findingId} — CR-002
// Returns 202 Accepted immediately; processing runs async in goroutine.
func (h *AIHTTPHandler) TriageFindingByID(w http.ResponseWriter, r *http.Request) {
	findingID := chi.URLParam(r, "findingId")
	if findingID == "" {
		http.Error(w, `{"error":"finding_id required"}`, http.StatusBadRequest)
		return
	}

	if h.triageUC == nil {
		http.Error(w, `{"error":"triage not initialized"}`, http.StatusNotImplemented)
		return
	}

	// CR-002: push to Redis queue before firing goroutine
	queuePos := 1
	if h.redis != nil {
		queueKey := "ai:triage:queue"
		item, _ := json.Marshal(map[string]any{
			"finding_id": findingID,
			"status":     "pending",
			"queued_at":  time.Now().UTC().Format(time.RFC3339),
		})
		h.redis.RPush(r.Context(), queueKey, item) //nolint:errcheck
		pos, _ := h.redis.LLen(r.Context(), queueKey).Result()
		queuePos = int(pos)
		// TTL 24h for queue
		h.redis.Expire(r.Context(), queueKey, 24*time.Hour) //nolint:errcheck
	}

	// Fire-and-forget: return 202 immediately, process async
	go func() {
		ctx := context.Background()
		if _, err := h.triageUC.Execute(ctx, triage.TriageFindingInput{
			FindingID: findingID,
		}); err != nil {
			h.log.Error().Err(err).Str("finding_id", findingID).Msg("async TriageFindingByID error")
			// Update queue status to failed
			if h.redis != nil {
				h.redis.HSet(ctx, "ai:triage:status:"+findingID, map[string]any{
					"status": "failed",
					"error":  err.Error(),
				}) //nolint:errcheck
			}
		} else {
			// Update queue status to done
			if h.redis != nil {
				h.redis.HSet(ctx, "ai:triage:status:"+findingID, map[string]any{
					"status":       "completed",
					"completed_at": time.Now().UTC().Format(time.RFC3339),
				}) //nolint:errcheck
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"status":         "queued",
		"finding_id":     findingID,
		"queue_position": queuePos,
	})
}

// ReviewTriage handles POST /api/v1/ai/triage/{findingId}/review — CR-014
// Persists human decision into Redis with 409 conflict detection.
// Supports ?force=true to override an existing review.
func (h *AIHTTPHandler) ReviewTriage(w http.ResponseWriter, r *http.Request) {
	findingID := chi.URLParam(r, "findingId")
	force := r.URL.Query().Get("force") == "true"

	var req struct {
		Decision string  `json:"decision"` // accepted|overridden|rejected
		Note     *string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate decision enum — CR-014
	validDecisions := map[string]bool{"accepted": true, "overridden": true, "rejected": true}
	if !validDecisions[req.Decision] {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"error":   "INVALID_DECISION",
			"message": "decision must be: accepted, overridden, or rejected",
		})
		return
	}

	reviewer := r.Header.Get("X-User-Email")
	if reviewer == "" {
		reviewer = r.Header.Get("X-User-ID")
	}

	if h.redis != nil {
		statusKey := "ai:triage:status:" + findingID

		// Check if already reviewed — CR-014: 409 Conflict
		existing, _ := h.redis.HGetAll(r.Context(), statusKey).Result()
		if decision, ok := existing["human_decision"]; ok && decision != "" && !force {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"error":   "ALREADY_REVIEWED",
				"message": "Use ?force=true to override existing review",
			})
			return
		}

		// Persist decision to Redis hash
		note := ""
		if req.Note != nil {
			note = *req.Note
		}
		h.redis.HSet(r.Context(), statusKey, map[string]any{ //nolint:errcheck
			"human_decision": req.Decision,
			"human_note":     note,
			"reviewed_by":    reviewer,
			"reviewed_at":    time.Now().UTC().Format(time.RFC3339),
		})
		h.redis.Expire(r.Context(), statusKey, 30*24*time.Hour) //nolint:errcheck

		// Also update the queue list item in-place by re-writing the status hash
		h.redis.HSet(r.Context(), "ai:triage:status:"+findingID, "status", "reviewed") //nolint:errcheck
	}

	h.log.Info().
		Str("finding_id", findingID).
		Str("decision", req.Decision).
		Str("reviewer", reviewer).
		Msg("triage review submitted")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"finding_id":  findingID,
		"decision":    req.Decision,
		"reviewer":    reviewer,
		"reviewed_at": time.Now().UTC().Format(time.RFC3339),
		"success":     true,
	})
}

// GetTriageQueue handles GET /api/v1/ai/triage/queue — CR-014
// Returns paginated triage queue with ai_result nested object + stats.
// Supports ?status=pending|accepted|overridden|rejected|all, ?severity=, ?remarks=
func (h *AIHTTPHandler) GetTriageQueue(w http.ResponseWriter, r *http.Request) {
	// P2-01: graceful degradation — return empty queue if LLM not ready
	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"items":   []interface{}{},
			"total":   0,
			"stats": map[string]interface{}{
				"pending":             0,
				"accepted_today":      0,
				"avg_confidence":      0.0,
				"false_positive_rate": 0.0,
			},
			"status":  "ai_service_initializing",
			"message": "AI provider not yet available. Queue will populate once Ollama is ready.",
		})
		return
	}

	statusFilter   := r.URL.Query().Get("status")
	severityFilter := r.URL.Query().Get("severity")
	remarksFilter  := r.URL.Query().Get("remarks")
	if statusFilter == "" {
		statusFilter = "pending"
	}

	items := []interface{}{}
	var total int64
	var pendingCount, acceptedToday int
	var totalConf float64
	var fpCount, allCount int

	if h.redis != nil {
		raw, err := h.redis.LRange(r.Context(), "ai:triage:queue", 0, -1).Result()
		if err == nil {
			for _, s := range raw {
				var item map[string]interface{}
				if json.Unmarshal([]byte(s), &item) != nil {
					continue
				}
				allCount++

				// Accumulate stats
				if humanDecision, _ := item["human_decision"]; humanDecision == nil {
					pendingCount++
				}
				if conf, ok := item["confidence"].(float64); ok {
					totalConf += conf
				}
				if remarks, _ := item["remarks"].(string); remarks == "FalsePositive" {
					fpCount++
				}

				// Apply filters
				itemHD, _ := item["human_decision"].(string)
				if statusFilter != "all" && statusFilter != "" {
					switch statusFilter {
					case "pending":
						if item["human_decision"] != nil {
							continue
						}
					case "accepted", "overridden", "rejected":
						if itemHD != statusFilter {
							continue
						}
					}
				}
				if severityFilter != "" {
					if sev, _ := item["severity"].(string); sev != severityFilter {
						continue
					}
				}
				if remarksFilter != "" {
					if rem, _ := item["remarks"].(string); rem != remarksFilter {
						continue
					}
				}

				// CR-014: reshape into ai_result nested object
				reshaped := map[string]interface{}{
					"finding_id":    item["finding_id"],
					"finding_title": item["finding_title"],
					"cve_id":        item["cve_id"],
					"severity":      item["severity"],
					"ai_result": map[string]interface{}{
						"remarks":       item["remarks"],
						"confidence":    item["confidence"],
						"justification": item["justification"],
						"actions":       item["actions"],
						"generated_at":  item["ai_generated_at"],
					},
					"human_decision": item["human_decision"],
					"human_note":     item["human_note"],
					"reviewed_by":    item["reviewed_by"],
					"reviewed_at":    item["reviewed_at"],
					"queued_at":      item["queued_at"],
				}
				items = append(items, reshaped)
			}
			total = int64(len(items))
		}
	}

	// Compute stats
	avgConf := 0.0
	if allCount > 0 {
		avgConf = (totalConf / float64(allCount)) * 100
	}
	fpRate := 0.0
	if allCount > 0 {
		fpRate = float64(fpCount) / float64(allCount) * 100
	}

	h.log.Info().
		Str("status_filter", statusFilter).
		Int64("count", total).
		Msg("GetTriageQueue")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"items": items,
		"total": total,
		"stats": map[string]interface{}{
			"pending":             pendingCount,
			"accepted_today":      acceptedToday,
			"avg_confidence":      avgConf,
			"false_positive_rate": fpRate,
		},
	})
}

// GetEnrichmentStatus handles GET /api/v1/ai/enrichment — CR-002
// Returns the current enrichment pipeline status from Redis.
// P2-01: returns graceful unavailable response when LLM provider unavailable.
func (h *AIHTTPHandler) GetEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	// P2-01: graceful degradation
	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status":          "unavailable",
			"provider":        "none",
			"queue_size":      0,
			"processed_today": 0,
			"total_enriched":  0,
			"last_run_at":     nil,
			"message":         "AI service not configured. Set AI_BACKEND=ollama and AI_BASE_URL to enable.",
		})
		return
	}

	status := "idle"
	var totalEnriched int64
	var lastRunAt interface{}

	if h.redis != nil {
		// Read aggregate status from Redis hash ai:enrichment:status
		vals, err := h.redis.HGetAll(r.Context(), "ai:enrichment:status").Result()
		if err == nil && len(vals) > 0 {
			if s, ok := vals["status"]; ok {
				status = s
			}
			if n, ok := vals["total_enriched"]; ok {
				totalEnriched, _ = strconv.ParseInt(n, 10, 64)
			}
			if t, ok := vals["last_run_at"]; ok {
				lastRunAt = t
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"status":         status,
		"total_enriched": totalEnriched,
		"last_run_at":    lastRunAt,
	})
}

// TriggerEnrichment handles POST /api/v1/ai/enrichment/trigger — CR-002
// Triggers manual enrichment for a list of CVE IDs.
// P2-01: returns 202 no-op when LLM provider unavailable.
func (h *AIHTTPHandler) TriggerEnrichment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CVEIDs []string `json:"cve_ids"`
		Force  bool     `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.CVEIDs) == 0 {
		http.Error(w, `{"error":"cve_ids required"}`, http.StatusBadRequest)
		return
	}

	// P2-01: graceful degradation — skip enrichment if LLM not available
	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status":  "skipped",
			"message": "AI provider not available. Enrichment skipped. Configure OLLAMA_URL to enable.",
			"queued_count": 0,
		})
		return
	}

	// CR-002: mark status as running in Redis
	if h.redis != nil {
		h.redis.HSet(r.Context(), "ai:enrichment:status", map[string]any{
			"status":      "running",
			"last_run_at": time.Now().UTC().Format(time.RFC3339),
			"queued":      len(req.CVEIDs),
		}) //nolint:errcheck
	}

	// Fire-and-forget enrichment for each CVE
	if h.enrichHandler != nil {
		go func() {
			ctx := context.Background()
			enriched := 0
			for _, id := range req.CVEIDs {
				if _, err := h.enrichHandler.Handle(ctx, enrich.Command{VulnID: id}); err != nil {
					h.log.Error().Err(err).Str("cve_id", id).Msg("TriggerEnrichment error")
				} else {
					enriched++
					// Store per-CVE enrichment result in Redis hash
					if h.redis != nil {
						h.redis.HSet(ctx, "ai:enrichment:"+id, map[string]any{
							"status":      "completed",
							"enriched_at": time.Now().UTC().Format(time.RFC3339),
						}) //nolint:errcheck
						h.redis.Expire(ctx, "ai:enrichment:"+id, 30*24*time.Hour) //nolint:errcheck
					}
				}
			}
			// Update aggregate status
			if h.redis != nil {
				h.redis.HSet(ctx, "ai:enrichment:status", map[string]any{
					"status":         "idle",
					"last_run_at":    time.Now().UTC().Format(time.RFC3339),
					"total_enriched": strconv.Itoa(enriched),
				}) //nolint:errcheck
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"queued_count": len(req.CVEIDs),
		"status":       "queued",
	})
}

// GetEnrichmentByCVE handles GET /api/v1/ai/enrichment/{cveId} — CR-002
// Returns AI enrichment detail for a specific CVE from Redis.
func (h *AIHTTPHandler) GetEnrichmentByCVE(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "cveId")

	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cve_id":  cveID,
			"status":  "not_enriched",
			"message": "AI enrichment not available",
		})
		return
	}

	if h.redis != nil {
		// Read per-CVE enrichment from Redis hash ai:enrichment:{cveId}
		vals, err := h.redis.HGetAll(r.Context(), "ai:enrichment:"+cveID).Result()
		if err == nil && len(vals) > 0 {
			h.log.Info().Str("cve_id", cveID).Msg("GetEnrichmentByCVE: cache hit")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"cve_id":      cveID,
				"status":      vals["status"],
				"enriched_at": vals["enriched_at"],
				"summary":     vals["summary"],
				"tags":        vals["tags"],
			})
			return
		}
	}

	// Not yet enriched — return 404 with actionable message
	h.log.Info().Str("cve_id", cveID).Msg("GetEnrichmentByCVE: not found")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"error":  "NOT_ENRICHED",
		"cve_id": cveID,
		"detail": "This CVE has not been enriched yet. Use POST /enrichment/trigger to queue it.",
	})
}

// EnrichCVE handles POST /api/v1/ai/enrich (legacy — cve_id in body)
func (h *AIHTTPHandler) EnrichCVE(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VulnID string `json:"cve_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "queued",
			"message": "AI provider not available. Request queued for later processing.",
		})
		return
	}

	if h.enrichHandler == nil {
		http.Error(w, "enrich handler not initialized", http.StatusNotImplemented)
		return
	}

	res, err := h.enrichHandler.Handle(r.Context(), enrich.Command{
		VulnID: req.VulnID,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("EnrichCVE error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res) //nolint:errcheck
}

// GetInsights handles GET /api/v1/ai/insights — BUG-011
// Returns AI-generated insights. Gracefully returns empty list when LLM unavailable.
func (h *AIHTTPHandler) GetInsights(w http.ResponseWriter, r *http.Request) {
	if !h.isReady() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"items":        []interface{}{},
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"status":       "ai_unavailable",
			"message":      "AI provider not configured. Set AI_BACKEND=ollama and AI_BASE_URL to enable.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"items":        []interface{}{},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"status":       "ok",
	})
}

// NewRouter creates the HTTP router for ai-service.
func NewRouter(h *AIHTTPHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/api/v1/ai", func(r chi.Router) {
		// Legacy endpoints (body-param based, kept for backward compat)
		r.Post("/triage", h.TriageFinding)
		r.Post("/enrich", h.EnrichCVE)

		// CR-002: New triage endpoints (path-param based)
		// IMPORTANT: /triage/queue MUST be before /triage/{findingId} in chi to avoid shadowing
		r.Get("/triage/queue", h.GetTriageQueue)
		r.Post("/triage/{findingId}", h.TriageFindingByID)
		r.Post("/triage/{findingId}/review", h.ReviewTriage)

		// CR-002: Enrichment management endpoints
		r.Get("/enrichment", h.GetEnrichmentStatus)
		r.Post("/enrichment/trigger", h.TriggerEnrichment)
		r.Get("/enrichment/{cveId}", h.GetEnrichmentByCVE)

		// BUG-011: Insights endpoint
		r.Get("/insights", h.GetInsights)
	})

	return r
}
