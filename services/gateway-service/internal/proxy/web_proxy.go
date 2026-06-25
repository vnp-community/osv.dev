// web_proxy.go — Frontend web interface serving for gateway-service.
// Supports two modes:
//   1. FRONTEND_URL set → Reverse proxy to Next.js / dev server
//   2. STATIC_DIR set → Serve pre-built static files
//   3. Neither set → 404 with friendly message
//
// Mount at /* (catch-all) AFTER API routes so API routes take priority.
// This is ADDITIVE — not modifying main.go directly.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// WebProxyConfig holds frontend serving configuration.
type WebProxyConfig struct {
	// FrontendURL is the URL of the frontend dev server (e.g. "http://localhost:3000").
	// When set, all non-API requests are proxied here.
	FrontendURL string

	// StaticDir is the path to the pre-built static files directory.
	// Used when FrontendURL is not set.
	StaticDir string
}

// WebProxyConfigFromEnv loads config from environment variables.
func WebProxyConfigFromEnv() WebProxyConfig {
	return WebProxyConfig{
		FrontendURL: os.Getenv("FRONTEND_URL"),
		StaticDir:   os.Getenv("STATIC_DIR"),
	}
}

// WebHandler returns an http.Handler for serving the frontend.
// Priority: FrontendURL > StaticDir > 404 page
func WebHandler(cfg WebProxyConfig) http.Handler {
	if cfg.FrontendURL != "" {
		target, err := url.Parse(cfg.FrontendURL)
		if err != nil {
			log.Error().Err(err).Str("url", cfg.FrontendURL).Msg("web proxy: invalid FRONTEND_URL")
			return notFoundHandler()
		}
		log.Info().Str("frontend_url", cfg.FrontendURL).Msg("web proxy: proxying to frontend")
		proxy := httputil.NewSingleHostReverseProxy(target)
		// Modify request to set correct Host header
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "osv-gateway/1.0")
			}
		}
		return proxy
	}

	if cfg.StaticDir != "" {
		log.Info().Str("static_dir", cfg.StaticDir).Msg("web proxy: serving static files")
		fs := http.FileServer(http.Dir(cfg.StaticDir))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Next.js uses file-based routing, handle missing files gracefully
			path := r.URL.Path
			if !strings.Contains(path, ".") {
				// Try index.html for SPA routes
				r.URL.Path = "/"
			}
			fs.ServeHTTP(w, r)
		})
	}

	return notFoundHandler()
}

func notFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"frontend not configured","hint":"set FRONTEND_URL or STATIC_DIR env var"}`)) //nolint:errcheck
	})
}
