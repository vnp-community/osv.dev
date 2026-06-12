// Package teams provides a Microsoft Teams webhook client
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client sends messages to Microsoft Teams via incoming webhook
type Client struct{ httpClient *http.Client }

// New creates a new Teams client
func New() *Client { return &Client{httpClient: &http.Client{}} }

// Send sends a message to a Teams channel via webhook URL
func (c *Client) Send(ctx context.Context, webhookURL, text string) error {
	payload := map[string]string{"text": text}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("teams webhook error: %s", resp.Status)
	}
	return nil
}
