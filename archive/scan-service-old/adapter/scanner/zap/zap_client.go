// Package zap provides an OWASP ZAP REST API client adapter.
package zap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/osv/scan-service/internal/domain/entity"
)

// Client provides low-level access to the ZAP REST API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a ZAP API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewContext creates a new ZAP scanning context. Returns contextID.
func (c *Client) NewContext(ctx context.Context, name string) (string, error) {
	resp, err := c.get(ctx, "/JSON/context/action/newContext/", url.Values{"contextname": {name}})
	if err != nil {
		return "", err
	}
	var r struct { ContextID string `json:"contextId"` }
	return r.ContextID, json.Unmarshal(resp, &r)
}

// IncludeInContext adds a URL pattern to the context scope.
func (c *Client) IncludeInContext(ctx context.Context, contextID, urlPattern string) error {
	_, err := c.get(ctx, "/JSON/context/action/includeInContext/", url.Values{
		"contextname": {contextID}, "regex": {urlPattern},
	})
	return err
}

// StartSpider starts a spider scan and returns its scanID.
func (c *Client) StartSpider(ctx context.Context, contextID, targetURL string) (string, error) {
	resp, err := c.get(ctx, "/JSON/spider/action/scan/", url.Values{
		"contextid": {contextID}, "url": {targetURL}, "recurse": {"true"},
	})
	if err != nil {
		return "", err
	}
	var r struct { Scan string `json:"scan"` }
	return r.Scan, json.Unmarshal(resp, &r)
}

// SpiderStatus returns the spider scan progress (0-100).
func (c *Client) SpiderStatus(ctx context.Context, scanID string) (int, error) {
	resp, err := c.get(ctx, "/JSON/spider/view/status/", url.Values{"scanId": {scanID}})
	if err != nil {
		return 0, err
	}
	var r struct { Status string `json:"status"` }
	if err := json.Unmarshal(resp, &r); err != nil {
		return 0, err
	}
	var progress int
	fmt.Sscanf(r.Status, "%d", &progress)
	return progress, nil
}

// StartActiveScan starts an active scan and returns its scanID.
func (c *Client) StartActiveScan(ctx context.Context, contextID, targetURL string) (string, error) {
	resp, err := c.get(ctx, "/JSON/ascan/action/scan/", url.Values{
		"contextid": {contextID}, "url": {targetURL}, "recurse": {"true"},
	})
	if err != nil {
		return "", err
	}
	var r struct { Scan string `json:"scan"` }
	return r.Scan, json.Unmarshal(resp, &r)
}

// ActiveScanStatus returns the active scan progress (0-100).
func (c *Client) ActiveScanStatus(ctx context.Context, scanID string) (int, error) {
	resp, err := c.get(ctx, "/JSON/ascan/view/status/", url.Values{"scanId": {scanID}})
	if err != nil {
		return 0, err
	}
	var r struct { Status string `json:"status"` }
	if err := json.Unmarshal(resp, &r); err != nil {
		return 0, err
	}
	var progress int
	fmt.Sscanf(r.Status, "%d", &progress)
	return progress, nil
}

// ZAPAlert represents a single security alert from ZAP.
type ZAPAlert struct {
	Alert       string `json:"alert"`
	Risk        string `json:"risk"`
	Confidence  string `json:"confidence"`
	Description string `json:"description"`
	Solution    string `json:"solution"`
	Reference   string `json:"reference"`
	Evidence    string `json:"evidence"`
	URL         string `json:"url"`
}

// GetAlerts retrieves all alerts for the given target URL.
func (c *Client) GetAlerts(ctx context.Context, targetURL string) ([]ZAPAlert, error) {
	resp, err := c.get(ctx, "/JSON/core/view/alerts/", url.Values{"baseurl": {targetURL}})
	if err != nil {
		return nil, err
	}
	var r struct { Alerts []ZAPAlert `json:"alerts"` }
	return r.Alerts, json.Unmarshal(resp, &r)
}

// RemoveContext cleans up a scanning context.
func (c *Client) RemoveContext(ctx context.Context, contextID string) error {
	_, err := c.get(ctx, "/JSON/context/action/removeContext/", url.Values{"contextname": {contextID}})
	return err
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	params.Set("apikey", c.APIKey)
	reqURL := c.BaseURL + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zap request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zap returned %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

// ZAPConfig holds ZAP scanning configuration.
type ZAPConfig struct {
	SpiderTimeout     time.Duration
	ActiveScanTimeout time.Duration
}

// ZAPScanner orchestrates a full ZAP scan cycle.
type ZAPScanner struct {
	Client *Client
}

// NewZAPScanner creates a ZAPScanner with the given client.
func NewZAPScanner(client *Client) *ZAPScanner {
	return &ZAPScanner{Client: client}
}

// Scan runs a full spider + active scan and returns web alerts.
func (z *ZAPScanner) Scan(ctx context.Context, scanID uuid.UUID, targetURL string, cfg ZAPConfig) ([]*entity.WebAlert, error) {
	if cfg.SpiderTimeout == 0 {
		cfg.SpiderTimeout = 5 * time.Minute
	}
	if cfg.ActiveScanTimeout == 0 {
		cfg.ActiveScanTimeout = 10 * time.Minute
	}

	// 1. Create context
	contextID, err := z.Client.NewContext(ctx, scanID.String())
	if err != nil {
		return nil, fmt.Errorf("create context: %w", err)
	}
	defer z.Client.RemoveContext(ctx, contextID) //nolint:errcheck

	// 2. Set scope
	if err := z.Client.IncludeInContext(ctx, contextID, targetURL+".*"); err != nil {
		return nil, err
	}

	// 3. Spider
	spiderID, err := z.Client.StartSpider(ctx, contextID, targetURL)
	if err != nil {
		return nil, err
	}
	if err := z.pollUntilDone(ctx, "spider", spiderID, cfg.SpiderTimeout, z.Client.SpiderStatus); err != nil {
		return nil, err
	}

	// 4. Active scan
	ascanID, err := z.Client.StartActiveScan(ctx, contextID, targetURL)
	if err != nil {
		return nil, err
	}
	if err := z.pollUntilDone(ctx, "active", ascanID, cfg.ActiveScanTimeout, z.Client.ActiveScanStatus); err != nil {
		return nil, err
	}

	// 5. Collect alerts
	zapAlerts, err := z.Client.GetAlerts(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	alerts := make([]*entity.WebAlert, 0, len(zapAlerts))
	for _, a := range zapAlerts {
		alerts = append(alerts, &entity.WebAlert{
			ID:          uuid.New(),
			ScanID:      scanID,
			TargetURL:   a.URL,
			AlertName:   a.Alert,
			Risk:        a.Risk,
			Confidence:  a.Confidence,
			Description: a.Description,
			Solution:    a.Solution,
			Reference:   a.Reference,
			Evidence:    a.Evidence,
		})
	}
	return alerts, nil
}

// pollUntilDone polls a status function every 5s until 100% or timeout.
func (z *ZAPScanner) pollUntilDone(ctx context.Context, scanType, scanID string, timeout time.Duration, statusFn func(context.Context, string) (int, error)) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			progress, err := statusFn(ctx, scanID)
			if err != nil {
				return fmt.Errorf("zap %s status: %w", scanType, err)
			}
			if progress >= 100 {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("zap %s timed out at %d%%", scanType, progress)
			}
		}
	}
}
