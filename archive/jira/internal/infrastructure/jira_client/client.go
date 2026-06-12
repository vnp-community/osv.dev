// Package jira_client provides a minimal JIRA Cloud REST API v3 client.
package jira_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a minimal JIRA Cloud REST API v3 client using Basic Auth.
type Client struct {
	baseURL    string
	username   string
	apiToken   string
	httpClient *http.Client
}

// New creates a JIRA Client with the given credentials.
func New(baseURL, username, apiToken string) *Client {
	return &Client{
		baseURL:    baseURL,
		username:   username,
		apiToken:   apiToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateIssueRequest is the body for POST /rest/api/3/issue.
type CreateIssueRequest struct {
	Fields IssueFields `json:"fields"`
}

// IssueFields defines the JIRA issue fields.
type IssueFields struct {
	Project     map[string]string  `json:"project"`
	Summary     string             `json:"summary"`
	Description interface{}        `json:"description"` // Atlassian Document Format
	IssueType   map[string]string  `json:"issuetype"`
	Priority    map[string]string  `json:"priority,omitempty"`
	Labels      []string           `json:"labels,omitempty"`
	Assignee    *map[string]string `json:"assignee,omitempty"`
}

// CreateIssueResponse holds the JIRA response after issue creation.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// IssueTransition represents a JIRA workflow transition.
type IssueTransition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateIssue calls POST /rest/api/3/issue and returns the new issue key and ID.
func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/rest/api/3/issue", bytes.NewReader(body))
	httpReq.SetBasicAuth(c.username, c.apiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jira: create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("jira: create issue: HTTP %d", resp.StatusCode)
	}

	var result CreateIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TransitionIssue changes a JIRA issue's workflow status.
func (c *Client) TransitionIssue(ctx context.Context, issueKey, transitionID string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/rest/api/3/issue/"+issueKey+"/transitions",
		bytes.NewReader(body))
	httpReq.SetBasicAuth(c.username, c.apiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("jira: transition issue %s: HTTP %d", issueKey, resp.StatusCode)
	}
	return nil
}

// GetTransitions lists available workflow transitions for a JIRA issue.
func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]IssueTransition, error) {
	httpReq, _ := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/rest/api/3/issue/"+issueKey+"/transitions", nil)
	httpReq.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Transitions []IssueTransition `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Transitions, nil
}

// UpdateIssueComment adds a comment to an issue.
func (c *Client) UpdateIssueComment(ctx context.Context, issueKey, comment string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": comment},
					},
				},
			},
		},
	})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/rest/api/3/issue/"+issueKey+"/comment",
		bytes.NewReader(body))
	httpReq.SetBasicAuth(c.username, c.apiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ADFDescription builds an Atlassian Document Format description from plaintext.
func ADFDescription(text string) interface{} {
	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": []map[string]interface{}{
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": text},
				},
			},
		},
	}
}
