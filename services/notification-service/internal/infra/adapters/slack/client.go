// Package slack provides a Slack Web API client for sending notifications
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const slackAPIURL = "https://slack.com/api/chat.postMessage"

// Client sends messages to Slack via Web API
type Client struct {
	botToken   string
	httpClient *http.Client
}

// New creates a new Slack client
func New(botToken string) *Client {
	return &Client{botToken: botToken, httpClient: &http.Client{}}
}

// SendMessage sends a text message to a Slack channel
func (c *Client) SendMessage(ctx context.Context, channel, text string) error {
	payload := map[string]string{"channel": channel, "text": text}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API error: %s", resp.Status)
	}
	return nil
}
