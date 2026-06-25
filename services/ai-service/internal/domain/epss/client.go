package epss

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"

    "github.com/rs/zerolog"
)

const (
    epssAPIBase   = "https://api.first.org/data/v1/epss"
    cacheExpiry   = 24 * time.Hour
    batchSize     = 100 // FIRST.org API limit per request
)

// EPSSScore holds the EPSS data for a CVE.
type EPSSScore struct {
    CVEID     string
    Score     float64 // 0.0-1.0 probability of exploitation in next 30 days
    Percentile float64 // relative to all CVEs
    Date      time.Time
}

// Client fetches EPSS scores from FIRST.org API with in-memory caching.
type Client struct {
    httpClient *http.Client
    cache      sync.Map // cveID → *EPSSScore
    cacheTime  sync.Map // cveID → time.Time (expiry)
    logger     zerolog.Logger
}

// New creates an EPSS Client.
func New(logger zerolog.Logger) *Client {
    return &Client{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        logger:     logger,
    }
}

// GetScore fetches the EPSS score for a single CVE (with cache).
func (c *Client) GetScore(ctx context.Context, cveID string) (*EPSSScore, error) {
    // Check cache
    if score, ok := c.getFromCache(cveID); ok {
        return score, nil
    }

    scores, err := c.fetchBatch(ctx, []string{cveID})
    if err != nil {
        return nil, err
    }

    if score, ok := scores[cveID]; ok {
        return score, nil
    }

    return nil, nil // CVE not in EPSS database
}

// GetBatch fetches EPSS scores for multiple CVEs.
func (c *Client) GetBatch(ctx context.Context, cveIDs []string) (map[string]*EPSSScore, error) {
    result := make(map[string]*EPSSScore)
    missing := make([]string, 0)

    // Check cache first
    for _, id := range cveIDs {
        if score, ok := c.getFromCache(id); ok {
            result[id] = score
        } else {
            missing = append(missing, id)
        }
    }

    if len(missing) == 0 {
        return result, nil
    }

    // Fetch missing in batches
    for i := 0; i < len(missing); i += batchSize {
        end := i + batchSize
        if end > len(missing) {
            end = len(missing)
        }
        batch := missing[i:end]

        fetched, err := c.fetchBatch(ctx, batch)
        if err != nil {
            c.logger.Warn().Err(err).Msg("EPSS batch fetch failed")
            continue
        }

        for k, v := range fetched {
            result[k] = v
        }
    }

    return result, nil
}

// fetchBatch calls the FIRST.org API for a batch of CVE IDs.
func (c *Client) fetchBatch(ctx context.Context, cveIDs []string) (map[string]*EPSSScore, error) {
    query := ""
    for i, id := range cveIDs {
        if i > 0 {
            query += ","
        }
        query += id
    }

    url := fmt.Sprintf("%s?cve=%s", epssAPIBase, query)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("EPSS API request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read EPSS response: %w", err)
    }

    var apiResp struct {
        Data []struct {
            CVE        string `json:"cve"`
            EPSS       string `json:"epss"`
            Percentile string `json:"percentile"`
            Date       string `json:"date"`
        } `json:"data"`
    }

    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, fmt.Errorf("parse EPSS response: %w", err)
    }

    result := make(map[string]*EPSSScore)
    for _, d := range apiResp.Data {
        var score, pctile float64
        fmt.Sscanf(d.EPSS, "%f", &score)
        fmt.Sscanf(d.Percentile, "%f", &pctile)

        s := &EPSSScore{
            CVEID:      d.CVE,
            Score:      score,
            Percentile: pctile,
            Date:       time.Now().UTC(),
        }
        result[d.CVE] = s

        // Cache the result
        c.cache.Store(d.CVE, s)
        c.cacheTime.Store(d.CVE, time.Now().UTC().Add(cacheExpiry))
    }

    return result, nil
}

// getFromCache returns cached score if not expired.
func (c *Client) getFromCache(cveID string) (*EPSSScore, bool) {
    score, ok := c.cache.Load(cveID)
    if !ok {
        return nil, false
    }

    expiry, _ := c.cacheTime.Load(cveID)
    if expTime, ok := expiry.(time.Time); ok && time.Now().UTC().After(expTime) {
        c.cache.Delete(cveID)
        c.cacheTime.Delete(cveID)
        return nil, false
    }

    return score.(*EPSSScore), true
}

// RefreshAll refreshes the EPSS cache for all known CVEs (called at midnight).
func (c *Client) RefreshAll(ctx context.Context) error {
    var keys []string
    c.cache.Range(func(key, _ interface{}) bool {
        keys = append(keys, key.(string))
        return true
    })

    if len(keys) == 0 {
        return nil
    }

    c.logger.Info().Int("count", len(keys)).Msg("refreshing EPSS cache")
    _, err := c.GetBatch(ctx, keys)
    return err
}
