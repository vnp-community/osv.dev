package epss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const epssAPIURL = "https://api.first.org/data/v1/epss"

// Client fetches EPSS scores from FIRST.org API
type Client struct {
	httpClient *http.Client
}

// New creates a new EPSS API client
func New() *Client {
	return &Client{httpClient: &http.Client{}}
}

type epssResponse struct {
	Data []struct {
		CVE        string  `json:"cve"`
		EPSS       string  `json:"epss"`
		Percentile string  `json:"percentile"`
	} `json:"data"`
}

// GetScore fetches the EPSS score and percentile for a CVE ID
func (c *Client) GetScore(ctx context.Context, cveID string) (score, percentile float64, err error) {
	url := fmt.Sprintf("%s?cve=%s", epssAPIURL, cveID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result epssResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}
	if len(result.Data) == 0 {
		return 0, 0, nil
	}

	fmt.Sscanf(result.Data[0].EPSS, "%f", &score)
	fmt.Sscanf(result.Data[0].Percentile, "%f", &percentile)
	return score, percentile, nil
}
