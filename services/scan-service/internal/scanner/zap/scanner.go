package zap

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "time"

    "github.com/rs/zerolog"
)

const (
	// pollInterval is kept as a local fallback; prefer ScannerConfig.PollInterval.
	pollInterval = 5 * time.Second
)

// WebAlert represents a ZAP vulnerability finding
type WebAlert struct {
    AlertName   string
    Risk        string // High|Medium|Low|Informational
    Confidence  string // High|Medium|Low|False Positive
    Description string
    Solution    string
    Reference   string
    Evidence    string
    TargetURL   string
    CWEId       int
    WASCID      int
}

// ScannerConfig holds OWASP ZAP API configuration.
type ScannerConfig struct {
	BaseURL           string
	APIKey            string
	Timeout           time.Duration
	// SpiderTimeout controls how long to wait for the spider scan to complete.
	// Default: 5m. Override via ZAP_SPIDER_TIMEOUT env var.
	SpiderTimeout time.Duration
	// ActiveScanTimeout controls how long to wait for the active scan to complete.
	// Default: 10m. Override via ZAP_ACTIVE_SCAN_TIMEOUT env var.
	ActiveScanTimeout time.Duration
	// PollInterval controls how often to poll ZAP for scan status.
	// Default: 5s. Override via ZAP_POLL_INTERVAL env var.
	PollInterval time.Duration
}

// Scanner wraps the OWASP ZAP REST API client.
type Scanner struct {
    config ScannerConfig
    client *http.Client
    logger zerolog.Logger
}

// New creates a ZAP Scanner.
// [FIX BUG-012] defaultZAPBase const removed — caller must provide BaseURL via env var ZAP_BASE_URL.
// Logs a warning if BaseURL is empty (will fail at runtime when scanning).
func New(cfg ScannerConfig, logger zerolog.Logger) *Scanner {
	if cfg.BaseURL == "" {
		logger.Warn().Msg("ZAP BaseURL is empty — set ZAP_BASE_URL env var; scans will fail")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	// Apply defaults for configurable timeouts
	if cfg.SpiderTimeout <= 0 {
		cfg.SpiderTimeout = 5 * time.Minute // [FIX] was: const defaultSpiderTime = 300
	}
	if cfg.ActiveScanTimeout <= 0 {
		cfg.ActiveScanTimeout = 10 * time.Minute // [FIX] was: const defaultActiveScan = 600
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = pollInterval
	}
	return &Scanner{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
	}
}

// ActiveScan runs a full ZAP active scan: Spider → Wait → Active Scan → Wait → Get Alerts
func (s *Scanner) ActiveScan(ctx context.Context, scanID, targetURL string) ([]*WebAlert, error) {
    s.logger.Info().
        Str("scan_id", scanID).
        Str("target", targetURL).
        Msg("starting ZAP active scan")

    // 1. Start spider
    spiderID, err := s.startSpider(ctx, targetURL)
    if err != nil {
        return nil, fmt.Errorf("start spider: %w", err)
    }

    // 2. Wait for spider to finish
    if err := s.waitForSpider(ctx, spiderID); err != nil {
        return nil, fmt.Errorf("spider timeout: %w", err)
    }

    // 3. Start active scan
    ascanID, err := s.startActiveScan(ctx, targetURL)
    if err != nil {
        return nil, fmt.Errorf("start active scan: %w", err)
    }

    // 4. Wait for active scan to finish
    if err := s.waitForActiveScan(ctx, ascanID); err != nil {
        return nil, fmt.Errorf("active scan timeout: %w", err)
    }

    // 5. Collect alerts
    return s.getAlerts(ctx, targetURL)
}

// startSpider initiates a ZAP spider scan and returns the scan ID.
func (s *Scanner) startSpider(ctx context.Context, targetURL string) (string, error) {
    params := url.Values{
        "zapapiformat": {"JSON"},
        "apikey":       {s.config.APIKey},
        "url":          {targetURL},
        "maxChildren":  {"0"},
    }
    resp, err := s.get(ctx, "/JSON/spider/action/scan/", params)
    if err != nil {
        return "", err
    }

    var result struct {
        Scan string `json:"scan"`
    }
    if err := json.Unmarshal(resp, &result); err != nil {
        return "", fmt.Errorf("parse spider response: %w", err)
    }
    return result.Scan, nil
}

// waitForSpider polls until the spider scan is 100% complete.
func (s *Scanner) waitForSpider(ctx context.Context, scanID string) error {
	// [FIX BUG-012] Use s.config.SpiderTimeout instead of const defaultSpiderTime
	deadline := time.Now().Add(s.config.SpiderTimeout)
	interval := s.config.PollInterval
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		params := url.Values{
			"zapapiformat": {"JSON"},
			"apikey":       {s.config.APIKey},
			"scanId":       {scanID},
		}
		resp, err := s.get(ctx, "/JSON/spider/view/status/", params)
		if err != nil {
			continue
		}

		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(resp, &status); err == nil && status.Status == "100" {
			return nil
		}
	}
	return fmt.Errorf("spider timeout after %s", s.config.SpiderTimeout)
}

// startActiveScan initiates a ZAP active vulnerability scan.
func (s *Scanner) startActiveScan(ctx context.Context, targetURL string) (string, error) {
    params := url.Values{
        "zapapiformat": {"JSON"},
        "apikey":       {s.config.APIKey},
        "url":          {targetURL},
        "recurse":      {"true"},
        "scanPolicyName": {""},
    }
    resp, err := s.get(ctx, "/JSON/ascan/action/scan/", params)
    if err != nil {
        return "", err
    }

    var result struct {
        Scan string `json:"scan"`
    }
    if err := json.Unmarshal(resp, &result); err != nil {
        return "", fmt.Errorf("parse active scan response: %w", err)
    }
    return result.Scan, nil
}

// waitForActiveScan polls until the active scan reaches 100%.
func (s *Scanner) waitForActiveScan(ctx context.Context, scanID string) error {
	// [FIX BUG-012] Use s.config.ActiveScanTimeout instead of const defaultActiveScan
	deadline := time.Now().Add(s.config.ActiveScanTimeout)
	interval := s.config.PollInterval
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		params := url.Values{
			"zapapiformat": {"JSON"},
			"apikey":       {s.config.APIKey},
			"scanId":       {scanID},
		}
		resp, err := s.get(ctx, "/JSON/ascan/view/status/", params)
		if err != nil {
			continue
		}

		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(resp, &status); err == nil && status.Status == "100" {
			return nil
		}
	}
	return fmt.Errorf("active scan timeout after %s", s.config.ActiveScanTimeout)
}

// getAlerts retrieves all alerts (findings) from ZAP for a given URL.
func (s *Scanner) getAlerts(ctx context.Context, targetURL string) ([]*WebAlert, error) {
    params := url.Values{
        "zapapiformat": {"JSON"},
        "apikey":       {s.config.APIKey},
        "baseurl":      {targetURL},
        "start":        {"0"},
        "count":        {"10000"},
    }
    resp, err := s.get(ctx, "/JSON/alert/view/alerts/", params)
    if err != nil {
        return nil, err
    }

    var result struct {
        Alerts []struct {
            Alert       string `json:"alert"`
            Risk        string `json:"risk"`
            Confidence  string `json:"confidence"`
            Description string `json:"description"`
            Solution    string `json:"solution"`
            Reference   string `json:"reference"`
            Evidence    string `json:"evidence"`
            URL         string `json:"url"`
            CWEId       string `json:"cweid"`
            WASCId      string `json:"wascid"`
        } `json:"alerts"`
    }

    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, fmt.Errorf("parse alerts: %w", err)
    }

    alerts := make([]*WebAlert, 0, len(result.Alerts))
    for _, a := range result.Alerts {
        cweID, _ := strconv.Atoi(a.CWEId)
        wascID, _ := strconv.Atoi(a.WASCId)
        alerts = append(alerts, &WebAlert{
            AlertName:   a.Alert,
            Risk:        a.Risk,
            Confidence:  a.Confidence,
            Description: a.Description,
            Solution:    a.Solution,
            Reference:   a.Reference,
            Evidence:    a.Evidence,
            TargetURL:   a.URL,
            CWEId:       cweID,
            WASCID:      wascID,
        })
    }

    s.logger.Info().
        Str("target", targetURL).
        Int("alert_count", len(alerts)).
        Msg("ZAP scan completed")

    return alerts, nil
}

// get makes a GET request to the ZAP API.
func (s *Scanner) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
    reqURL := s.config.BaseURL + path + "?" + params.Encode()
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    if err != nil {
        return nil, err
    }

    resp, err := s.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("zap request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("zap returned %d: %s", resp.StatusCode, string(body))
    }

    return body, nil
}
