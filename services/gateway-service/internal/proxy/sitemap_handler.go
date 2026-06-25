// sitemap_handler.go — Generates sitemap.xml for SEO discoverability of CVEs.
// This is a NEW handler — additive to gateway-service.
// Mount at GET /sitemap.xml in main.go.
package proxy

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"time"
)

// SitemapHandler generates sitemap.xml by listing CVE IDs from data-service.
// Activated via GET /sitemap.xml in gateway-service.
type SitemapHandler struct {
	dataHTTP  string // data-service HTTP base URL (e.g. "http://data-service:8082")
	publicURL string // Public base URL (e.g. "https://osv.dev")
	httpClient *http.Client
}

// NewSitemapHandler creates a SitemapHandler.
// publicURL: e.g. "https://osv.dev" (from PUBLIC_URL env var)
// dataHTTP: e.g. "http://localhost:8082" (from DATA_SERVICE_HTTP env var)
func NewSitemapHandler() *SitemapHandler {
	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "https://osv.dev"
	}
	dataHTTP := os.Getenv("DATA_SERVICE_HTTP")
	if dataHTTP == "" {
		dataHTTP = "http://localhost:8082"
	}
	return &SitemapHandler{
		dataHTTP:   dataHTTP,
		publicURL:  publicURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── XML types ─────────────────────────────────────────────────────────────────

type sitemapURLSet struct {
	XMLName xml.Name      `xml:"urlset"`
	XMLNS   string        `xml:"xmlns,attr"`
	URLs    []sitemapURL  `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// ServeHTTP implements http.Handler.
// Returns sitemap.xml with CVE URLs from data-service.
func (h *SitemapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	sitemap := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs: []sitemapURL{
			{
				Loc:        h.publicURL + "/",
				Priority:   "1.0",
				ChangeFreq: "daily",
				LastMod:    time.Now().Format("2006-01-02"),
			},
			{
				Loc:        h.publicURL + "/vulns",
				Priority:   "0.9",
				ChangeFreq: "hourly",
			},
		},
	}

	// Fetch CVE IDs from data-service (non-fatal if unavailable)
	cveURLs, err := h.fetchCVEURLs(ctx)
	if err == nil {
		sitemap.URLs = append(sitemap.URLs, cveURLs...)
	}
	// If data-service unavailable, we still return the base sitemap

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))            //nolint:errcheck
	xml.NewEncoder(w).Encode(sitemap)      //nolint:errcheck
}

// fetchCVEURLs lists CVE IDs from data-service and converts them to sitemap URLs.
func (h *SitemapHandler) fetchCVEURLs(ctx context.Context) ([]sitemapURL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.dataHTTP+"/v1/cves?limit=5000&fields=id,modified", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("data-service list CVEs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("data-service list CVEs: status %d", resp.StatusCode)
	}

	var cveList struct {
		Items []struct {
			ID       string `json:"id"`
			Modified string `json:"modified"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cveList); err != nil {
		return nil, fmt.Errorf("decode CVE list: %w", err)
	}

	urls := make([]sitemapURL, 0, len(cveList.Items))
	for _, item := range cveList.Items {
		u := sitemapURL{
			Loc:        fmt.Sprintf("%s/vulns/%s", h.publicURL, item.ID),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		}
		if item.Modified != "" {
			// Truncate to date format
			if len(item.Modified) >= 10 {
				u.LastMod = item.Modified[:10]
			}
		}
		urls = append(urls, u)
	}
	return urls, nil
}
