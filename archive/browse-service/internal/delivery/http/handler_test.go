// Package http_test provides unit tests for browse-service HTTP handlers.
// Uses interface mocks — no real Redis or MongoDB required.
package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	browsehttp "github.com/osv/browse-service/internal/delivery/http"
	"github.com/osv/browse-service/internal/domain/repository"
)

// ── Mock Implementations ──────────────────────────────────────────────────────

// mockCache implements repository.CacheRepository.
type mockCache struct {
	vendors  map[string][]string // key: cpeType → vendors
	products map[string][]string // key: vendor → products
	getErr   error
}

func newMockCache() *mockCache {
	return &mockCache{
		vendors:  make(map[string][]string),
		products: make(map[string][]string),
	}
}

func (m *mockCache) GetVendors(_ context.Context, cpeType string) ([]string, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.vendors[cpeType], nil
}

func (m *mockCache) GetProducts(_ context.Context, vendor string) ([]string, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.products[vendor], nil
}

func (m *mockCache) SetVendors(_ context.Context, cpeType string, v []string) error {
	m.vendors[cpeType] = v
	return nil
}

func (m *mockCache) SetProducts(_ context.Context, vendor string, p []string) error {
	m.products[vendor] = p
	return nil
}

// Verify interface compliance.
var _ repository.CacheRepository = (*mockCache)(nil)

// mockCPERepo implements repository.CPERepository.
type mockCPERepo struct {
	vendors  []string
	products []string
	search   []string
	err      error
}

func (m *mockCPERepo) ListVendors(_ context.Context, _ string) ([]string, error) {
	return m.vendors, m.err
}
func (m *mockCPERepo) ListProducts(_ context.Context, _ string) ([]string, error) {
	return m.products, m.err
}
func (m *mockCPERepo) SearchVendors(_ context.Context, _ string) ([]string, error) {
	return m.search, m.err
}

// Verify interface compliance.
var _ repository.CPERepository = (*mockCPERepo)(nil)

// ── Tests ─────────────────────────────────────────────────────────────────────

func newTestHandler(cache *mockCache, repo *mockCPERepo) http.Handler {
	h := browsehttp.NewHandler(cache, repo, zerolog.Nop())
	return browsehttp.NewRouter(h)
}

// TestListVendors_CacheHit verifies that vendors are returned from Redis L1 cache.
func TestListVendors_CacheHit(t *testing.T) {
	cache := newMockCache()
	cache.vendors["a"] = []string{"apache", "microsoft", "cisco"}
	repo := &mockCPERepo{} // should NOT be called

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/browse/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	items, _ := result["items"].([]interface{})
	if len(items) != 3 {
		t.Errorf("expected 3 vendors from cache, got %d", len(items))
	}
}

// TestListVendors_CacheMiss_MongoDB verifies L1 miss falls back to MongoDB L2.
func TestListVendors_CacheMiss_MongoDB(t *testing.T) {
	cache := newMockCache() // empty cache
	repo := &mockCPERepo{vendors: []string{"redhat", "ubuntu"}}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/browse/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	items, _ := result["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 vendors from MongoDB fallback, got %d", len(items))
	}
}

// TestListVendors_TypeParam verifies ?type=o returns OS vendors.
func TestListVendors_TypeParam(t *testing.T) {
	cache := newMockCache()
	cache.vendors["o"] = []string{"linux", "microsoft"}
	repo := &mockCPERepo{}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/browse/?type=o", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestListProducts_CacheHit verifies products are returned from L1 cache.
func TestListProducts_CacheHit(t *testing.T) {
	cache := newMockCache()
	cache.products["apache"] = []string{"httpd", "tomcat", "struts"}
	repo := &mockCPERepo{}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/browse/apache", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	items, _ := result["items"].([]interface{})
	if len(items) != 3 {
		t.Errorf("expected 3 products from cache, got %d", len(items))
	}
}

// TestListProducts_CacheMiss verifies fallback to MongoDB.
func TestListProducts_CacheMiss(t *testing.T) {
	cache := newMockCache()
	repo := &mockCPERepo{products: []string{"httpd", "tomcat"}}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/browse/apache", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestSearchVendors_Success verifies vendor search returns results.
func TestSearchVendors_Success(t *testing.T) {
	cache := newMockCache()
	repo := &mockCPERepo{search: []string{"cisco", "cisco_systems"}}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/vendors/search?q=cisco", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	items, _ := result["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 search results, got %d", len(items))
	}
}

// TestSearchVendors_MissingQ verifies missing q param returns 400.
func TestSearchVendors_MissingQ(t *testing.T) {
	cache := newMockCache()
	repo := &mockCPERepo{}

	router := newTestHandler(cache, repo)
	req := httptest.NewRequest(http.MethodGet, "/vendors/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestHealth verifies health endpoint.
func TestHealth(t *testing.T) {
	cache := newMockCache()
	repo := &mockCPERepo{}
	router := newTestHandler(cache, repo)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

// TestListVersions_ReturnsNote verifies stub handler returns redirect note.
func TestListVersions_ReturnsNote(t *testing.T) {
	cache := newMockCache()
	repo := &mockCPERepo{}
	router := newTestHandler(cache, repo)

	req := httptest.NewRequest(http.MethodGet, "/search/apache/httpd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result) //nolint:errcheck
	if result["vendor"] != "apache" {
		t.Errorf("expected vendor=apache, got %v", result["vendor"])
	}
	if result["note"] == nil {
		t.Error("expected note field in response")
	}
}
