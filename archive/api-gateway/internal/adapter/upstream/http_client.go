// Package upstream provides an HTTP reverse proxy client for upstream services.
package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// UpstreamConfig holds the address of an upstream service.
type UpstreamConfig struct {
	URL     string
	Timeout time.Duration
}

// HTTPUpstreamClient forwards requests to upstream services.
type HTTPUpstreamClient struct {
	services   map[string]UpstreamConfig
	httpClient *http.Client
}

// NewHTTPUpstreamClient creates a client for the given service map.
func NewHTTPUpstreamClient(services map[string]UpstreamConfig) *HTTPUpstreamClient {
	return &HTTPUpstreamClient{
		services: services,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Forward proxies a request to the named upstream service.
// It copies headers, body, status code and response body.
func (c *HTTPUpstreamClient) Forward(ctx context.Context, w http.ResponseWriter, r *http.Request, targetService string) error {
	cfg, ok := c.services[targetService]
	if !ok {
		http.Error(w, `{"error":"upstream_not_configured","service":"`+targetService+`"}`, http.StatusBadGateway)
		return fmt.Errorf("upstream %q not configured", targetService)
	}

	targetURL := cfg.URL + r.URL.RequestURI()

	reqCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	proxyReq, err := http.NewRequestWithContext(reqCtx, r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return fmt.Errorf("build proxy request: %w", err)
	}

	// Copy relevant request headers.
	copyHeaders := []string{
		"Content-Type", "Accept", "Authorization",
		"X-User-ID", "X-User-Role", "X-User-Permissions",
		"X-API-Key", "X-Request-ID", "X-Forwarded-For", "X-Real-IP",
	}
	for _, h := range copyHeaders {
		if v := r.Header.Get(h); v != "" {
			proxyReq.Header.Set(h, v)
		}
	}
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)
	proxyReq.Header.Set("X-Forwarded-Proto", "http")

	resp, err := c.httpClient.Do(proxyReq)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			http.Error(w, `{"error":"upstream_timeout"}`, http.StatusGatewayTimeout)
			return fmt.Errorf("upstream timeout: %w", err)
		}
		http.Error(w, `{"error":"upstream_unavailable"}`, http.StatusBadGateway)
		return fmt.Errorf("upstream error: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	// Copy response headers.
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
	return nil
}
