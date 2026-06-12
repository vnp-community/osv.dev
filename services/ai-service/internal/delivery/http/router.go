package http

import "net/http"

// NewRouter creates the HTTP router for ai-service
// Registers all REST endpoints alongside existing gRPC
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// POST /enrich/{cve_id}   — trigger enrichment
	// GET  /enrich/{cve_id}   — get cached enrichment
	// GET  /epss/{cve_id}     — EPSS score
	// POST /triage/finding    — AI triage
	// POST /admin/batch-enrich — bulk enrichment
	// GET  /embedding/{cve_id} — get stored embedding

	mux.HandleFunc("POST /enrich/", enrichCVEHandler)
	mux.HandleFunc("GET /enrich/", getEnrichmentHandler)
	mux.HandleFunc("GET /epss/", getEPSSHandler)
	mux.HandleFunc("POST /triage/finding", triageFindingHandler)
	mux.HandleFunc("POST /admin/batch-enrich", batchEnrichHandler)
	mux.HandleFunc("GET /embedding/", getEmbeddingHandler)

	return mux
}

func enrichCVEHandler(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(http.StatusAccepted) }
func getEnrichmentHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func getEPSSHandler(w http.ResponseWriter, r *http.Request)       { w.WriteHeader(http.StatusOK) }
func triageFindingHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func batchEnrichHandler(w http.ResponseWriter, r *http.Request)   { w.WriteHeader(http.StatusAccepted) }
func getEmbeddingHandler(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(http.StatusOK) }
