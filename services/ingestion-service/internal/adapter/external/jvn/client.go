// Package jvn provides a client for the JVN (Japan Vulnerability Notes) CVE feed.
package jvn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/osv/ingestion-service/internal/domain/entity"
)

const defaultFeedURL = "https://jvndb.jvn.jp/myjvn?method=getVulnOverviewList&feed=hnd&lang=en&startItem=1&maxCountItem=100"

// Client fetches JVN vulnerability data.
type Client struct {
	feedURL    string
	httpClient *http.Client
}

// NewClient creates a JVN client.
func NewClient() *Client {
	return &Client{
		feedURL:    defaultFeedURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type jvnItem struct {
	ID        string `json:"jvnID"`
	Title     string `json:"title"`
	Published string `json:"published"` // "2021-12-09T17:15:00Z"
}

type jvnResponse struct {
	Items []jvnItem `json:"items"`
}

// FetchLatest fetches the latest JVN vulnerability list.
func (c *Client) FetchLatest(ctx context.Context) ([]*entity.CVE, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jvn: request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jvn: status %d", resp.StatusCode)
	}

	var jr jvnResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		// JVN may return HTML/XML; treat gracefully
		return nil, fmt.Errorf("jvn: decode: %w", err)
	}

	cves := make([]*entity.CVE, 0, len(jr.Items))
	for _, item := range jr.Items {
		if item.ID == "" {
			continue
		}
		cve := &entity.CVE{
			ID:          item.ID,
			Description: item.Title,
			Source:      entity.SourceJVN,
			Link:        "https://jvndb.jvn.jp/en/contents/" + item.ID + ".html",
		}
		if t, err := time.Parse(time.RFC3339, item.Published); err == nil {
			cve.Published = &t
		}
		cve.InferSeverity("")
		cves = append(cves, cve)
	}
	return cves, nil
}
