// info_handler.go — Database statistics endpoint for data-service.
// GET /info — returns collection sizes and last update timestamps.
// Mirrors Python: web/restapi/dbinfo.py in cve-search
package http

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// InfoHandler provides database statistics endpoints.
type InfoHandler struct {
	db *mongo.Database
}

// NewInfoHandler creates an InfoHandler.
func NewInfoHandler(db *mongo.Database) *InfoHandler {
	return &InfoHandler{db: db}
}

// CollectionInfo holds statistics for a single MongoDB collection.
type CollectionInfo struct {
	Collection string    `json:"collection"`
	Size       int64     `json:"size"`
	LastUpdate time.Time `json:"last_update,omitempty"`
}

// GetDBInfo handles: GET /info
//
// Returns statistics for all CVE-related collections.
// Uses EstimatedDocumentCount (O(1)) for performance.
// Mirrors Python: web/restapi/dbinfo.py
//
// @Summary Get database statistics
// @Success 200 {object} map[string]interface{}
// @Router /info [get]
func (h *InfoHandler) GetDBInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Collections to report
	collectionNames := []string{"cves", "cpe", "cwe", "capec", "via4", "ranking"}

	stats := make(map[string]*CollectionInfo, len(collectionNames))

	for _, name := range collectionNames {
		col := h.db.Collection(name)

		// EstimatedDocumentCount is O(1) — uses collection metadata
		count, err := col.EstimatedDocumentCount(ctx)
		if err != nil {
			count = -1 // indicate error
		}

		info := &CollectionInfo{
			Collection: name,
			Size:       count,
		}

		// Try to get last update time from "info" collection
		info.LastUpdate = h.getLastUpdate(ctx, name)

		stats[name] = info
	}

	// Summary shortcut
	totalCVEs := int64(0)
	if cveInfo, ok := stats["cves"]; ok {
		totalCVEs = cveInfo.Size
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"collections": stats,
		"total_cves":  totalCVEs,
		"fetched_at":  time.Now().UTC(),
	})
}

// getLastUpdate retrieves the last update time for a collection from the "info" meta-collection.
// Falls back to zero time if not found.
func (h *InfoHandler) getLastUpdate(ctx context.Context, colName string) time.Time {
	var doc struct {
		LastModified time.Time `bson:"lastModified"`
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	h.db.Collection("info").FindOne(ctxWithTimeout, bson.M{"db": colName}).Decode(&doc) //nolint:errcheck
	return doc.LastModified
}
