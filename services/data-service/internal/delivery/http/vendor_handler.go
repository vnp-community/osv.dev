// Package http — vendor_handler.go (data-service)
// Provides vendor endpoints:
//   GET /api/v2/vendors?q=apa&limit=20      — autocomplete / list with counts
//   GET /api/v2/browse?vendor=apache        — vendor summary
//   GET /api/v2/browse/{vendor}             — products for a vendor
//   GET /api/v2/browse/{vendor}/{product}   — CVEs for a vendor+product
//
// Results from /api/v2/vendors are cached in Redis for 1h.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// VendorRepository provides vendor autocomplete queries (original interface).
type VendorRepository interface {
	AutocompleteVendors(ctx context.Context, prefix string, limit int) ([]string, error)

	// Browse methods — P1-08
	GetVendors(ctx context.Context, q string, limit int) ([]VendorEntry, error)
	GetProductsByVendor(ctx context.Context, vendor string, page, pageSize int) ([]ProductEntry, int, error)
	GetCVEsByVendorProduct(ctx context.Context, vendor, product string, page, pageSize int) ([]CVEBrowseEntry, int, error)
}

// VendorEntry is a vendor with its CVE count.
type VendorEntry struct {
	Vendor   string `json:"vendor"`
	CVECount int64  `json:"cve_count"`
}

// ProductEntry is a product under a vendor with its CVE count.
type ProductEntry struct {
	Product  string `json:"product"`
	Vendor   string `json:"vendor"`
	CVECount int64  `json:"cve_count"`
}

// CVEBrowseEntry is a CVE in a vendor/product browse list.
type CVEBrowseEntry struct {
	CveID       string  `json:"cve_id"`
	SeverityV3  string  `json:"severity_v3"`
	CVSSScore   float64 `json:"cvss_v3_score"`
	IsKEV       bool    `json:"is_kev"`
	PublishedAt string  `json:"published_at"`
}

// VendorHandler handles vendor autocomplete and browse endpoints.
type VendorHandler struct {
	vendorRepo VendorRepository
	redis      *redis.Client
}

// NewVendorHandler creates a new VendorHandler.
func NewVendorHandler(repo VendorRepository, redis *redis.Client) *VendorHandler {
	return &VendorHandler{vendorRepo: repo, redis: redis}
}

// GET /api/v2/vendors?q=apa&limit=20 — autocomplete string list (original, Redis cached)
func (h *VendorHandler) AutocompleteVendors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Cache key uses osv: prefix to avoid collisions
	cacheKey := "osv:vendors:ac:" + strings.ToLower(q) + ":" + strconv.Itoa(limit)

	if h.redis != nil {
		if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write(cached) //nolint:errcheck
			return
		}
	}

	vendors, err := h.vendorRepo.AutocompleteVendors(r.Context(), q, limit)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := map[string]interface{}{"vendors": vendors}
	data, _ := json.Marshal(resp)

	// Cache for 1 hour
	if h.redis != nil {
		h.redis.Set(r.Context(), cacheKey, data, time.Hour) //nolint:errcheck
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data) //nolint:errcheck
}

// GET /api/v2/vendors?q=apache&limit=50 — list vendors with CVE counts
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	vendors, err := h.vendorRepo.GetVendors(r.Context(), q, limit)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if vendors == nil {
		vendors = []VendorEntry{}
	}

	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"vendors": vendors,
		"total":   len(vendors),
	})
}

// GET /api/v2/browse?vendor=apache — vendor summary (alias for ListVendors with single filter)
func (h *VendorHandler) Browse(w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	vendors, err := h.vendorRepo.GetVendors(r.Context(), vendor, 50)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if vendors == nil {
		vendors = []VendorEntry{}
	}
	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"vendors": vendors,
		"total":   len(vendors),
	})
}

// GET /api/v2/browse/{vendor} — list products for a specific vendor
func (h *VendorHandler) BrowseVendor(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 20)

	products, total, err := h.vendorRepo.GetProductsByVendor(r.Context(), vendor, page, pageSize)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if products == nil {
		products = []ProductEntry{}
	}

	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"vendor":    vendor,
		"products":  products,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GET /api/v2/browse/{vendor}/{product} — list CVEs for a specific vendor+product
func (h *VendorHandler) BrowseProduct(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	product := chi.URLParam(r, "product")
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 20)

	cves, total, err := h.vendorRepo.GetCVEsByVendorProduct(r.Context(), vendor, product, page, pageSize)
	if err != nil {
		writeJSONData(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cves == nil {
		cves = []CVEBrowseEntry{}
	}

	writeJSONData(w, http.StatusOK, map[string]interface{}{
		"vendor":    vendor,
		"product":   product,
		"cves":      cves,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// parseIntDefault parses a string to int, returning def on error or invalid value.
func parseIntDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}


