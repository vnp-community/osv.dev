// Package taxonomy — injectable HTTP handler for CWE and CAPEC endpoints.
// Refactored from standalone main.go to support mounting in data-service main router.
//
// ADDITIVE: original internal/domain/taxonomy/main.go is NOT modified.
// This handler can be mounted into any chi.Router.
//
// Routes exposed via Handler.Mount():
//
//	GET /cwe/{id}       — CWE lookup (accepts "502", "CWE-502", "CWE502")
//	GET /cwe/{id}/capec — CAPECs linked to a CWE
//	GET /capec/{id}     — CAPEC lookup
//	GET /capec/{id}/cwe — CWE IDs linked to a CAPEC
package taxonomy

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler provides CWE and CAPEC HTTP endpoints.
// Mount using Handler.Mount(r chi.Router) into any chi router.
type Handler struct {
	cweCol   *mongo.Collection
	capecCol *mongo.Collection
}

// NewHandler creates a taxonomy Handler.
// db: MongoDB database connection (uses "cwe" and "capec" collections)
func NewHandler(db *mongo.Database) *Handler {
	return &Handler{
		cweCol:   db.Collection("cwe"),
		capecCol: db.Collection("capec"),
	}
}

// Mount registers all taxonomy routes to a chi router.
// Routes:
//
//	GET /cwe/{id}       — CWE lookup (accepts "502", "CWE-502", "CWE502")
//	GET /cwe/{id}/capec — CAPECs linked to a CWE
//	GET /capec/{id}     — CAPEC lookup
//	GET /capec/{id}/cwe — CWEs linked to a CAPEC
func (h *Handler) Mount(r chi.Router) {
	r.Get("/cwe/{id}", h.GetCWE)
	r.Get("/cwe/{id}/capec", h.GetCAPECForCWE)
	r.Get("/capec/{id}", h.GetCAPEC)
	r.Get("/capec/{id}/cwe", h.GetCWEForCAPEC)
}

// GetCWE handles: GET /cwe/{id}
// Accepts: "502", "CWE-502", "CWE502" — normalizes to numeric ID for MongoDB lookup.
func (h *Handler) GetCWE(w http.ResponseWriter, r *http.Request) {
	id := normalizeCWEID(chi.URLParam(r, "id"))
	var result bson.M
	err := h.cweCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		writeTaxonomyJSON(w, http.StatusNotFound, map[string]string{
			"error": "CWE not found",
			"id":    id,
		})
		return
	}
	if err != nil {
		writeTaxonomyJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeTaxonomyJSON(w, http.StatusOK, result)
}

// GetCAPECForCWE handles: GET /cwe/{id}/capec
// Returns CAPECs that reference this CWE via their related_weakness field.
func (h *Handler) GetCAPECForCWE(w http.ResponseWriter, r *http.Request) {
	cweID := normalizeCWEID(chi.URLParam(r, "id"))
	cursor, err := h.capecCol.Find(r.Context(), bson.M{
		"related_weakness": bson.M{
			"$elemMatch": bson.M{"$regex": "^" + cweID + "$"},
		},
	})
	if err != nil {
		writeTaxonomyJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer cursor.Close(r.Context())

	var results []bson.M
	cursor.All(r.Context(), &results) //nolint:errcheck
	writeTaxonomyJSON(w, http.StatusOK, map[string]interface{}{
		"capec": results,
		"total": len(results),
		"cwe":   cweID,
	})
}

// GetCAPEC handles: GET /capec/{id}
func (h *Handler) GetCAPEC(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var result bson.M
	err := h.capecCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		writeTaxonomyJSON(w, http.StatusNotFound, map[string]string{
			"error": "CAPEC not found",
			"id":    id,
		})
		return
	}
	if err != nil {
		writeTaxonomyJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeTaxonomyJSON(w, http.StatusOK, result)
}

// GetCWEForCAPEC handles: GET /capec/{id}/cwe
// Returns CWE IDs linked to this CAPEC via its related_weakness field.
func (h *Handler) GetCWEForCAPEC(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var capec struct {
		RelatedWeakness []string `bson:"related_weakness"`
	}
	if err := h.capecCol.FindOne(r.Context(), bson.M{"id": id}).Decode(&capec); err != nil {
		if err == mongo.ErrNoDocuments {
			writeTaxonomyJSON(w, http.StatusNotFound, map[string]string{"error": "CAPEC not found"})
			return
		}
		writeTaxonomyJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeTaxonomyJSON(w, http.StatusOK, map[string]interface{}{
		"capec":   id,
		"cwe_ids": capec.RelatedWeakness,
		"total":   len(capec.RelatedWeakness),
	})
}

// normalizeCWEID accepts "502", "CWE-502", "CWE502" → returns "502"
func normalizeCWEID(id string) string {
	id = strings.TrimPrefix(id, "CWE-")
	id = strings.TrimPrefix(id, "CWE")
	return id
}

func writeTaxonomyJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
