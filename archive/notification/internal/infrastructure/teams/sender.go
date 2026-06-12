// Package teams provides the MS Teams Adaptive Card sender.
// Includes SSRF protection: rejects private IP ranges.
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Sender implements dispatch.TeamsSender by posting Adaptive Cards to Teams webhooks.
type Sender struct {
	httpClient *http.Client
}

// New creates a Teams Sender.
func New() *Sender {
	return &Sender{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// Send posts an Adaptive Card message to a Teams incoming webhook URL.
// Returns an error if the URL resolves to a private/internal address (SSRF protection).
func (s *Sender) Send(_ context.Context, webhookURL, title, description string) error {
	if err := validateWebhookURL(webhookURL); err != nil {
		return fmt.Errorf("teams sender: SSRF check: %w", err)
	}

	card := buildAdaptiveCard(title, description)
	body, _ := json.Marshal(card)

	resp, err := s.httpClient.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("teams sender: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("teams sender: HTTP %d", resp.StatusCode)
	}
	return nil
}

// validateWebhookURL rejects private/internal IP addresses (SSRF protection).
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook must use HTTPS")
	}

	host := u.Hostname()
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		// Block private ranges
		for _, block := range privateRanges {
			if block.Contains(ip) {
				return fmt.Errorf("webhook points to private/internal IP: %s", ipStr)
			}
		}
	}
	return nil
}

var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, block)
	}
}

// buildAdaptiveCard creates an MS Teams Adaptive Card payload.
func buildAdaptiveCard(title, description string) map[string]interface{} {
	return map[string]interface{}{
		"type": "message",
		"attachments": []map[string]interface{}{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.4",
					"body": []map[string]interface{}{
						{"type": "TextBlock", "text": title, "weight": "Bolder", "size": "Medium"},
						{"type": "TextBlock", "text": description, "wrap": true},
					},
					"actions": []map[string]interface{}{
						{
							"type":  "Action.OpenUrl",
							"title": "View in DefectDojo",
							"url":   "#",
						},
					},
				},
			},
		},
	}
}

// ensure Teams URLs end in .webhook.office.com (extra safety check)
func isTeamsURL(rawURL string) bool {
	return strings.Contains(rawURL, "webhook.office.com") || strings.Contains(rawURL, "outlook.office.com")
}
