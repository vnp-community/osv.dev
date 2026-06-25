// Package jira provides a client for Jira REST API v3.
package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for Jira REST API v3.
type Client struct {
	serverURL string
	apiToken  string
	email     string
	httpClient *http.Client
}

// NewClient creates a new Jira REST API v3 client.
// serverURL: e.g. "https://your-org.atlassian.net"
// email: Jira account email
// apiToken: Jira API token
func NewClient(serverURL, email, apiToken string) *Client {
	return &Client{
		serverURL:  serverURL,
		apiToken:   apiToken,
		email:      email,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// CreateIssueRequest is the payload to create a Jira issue.
type CreateIssueRequest struct {
	Fields struct {
		Project   struct{ Key string } `json:"project"`
		Summary   string               `json:"summary"`
		IssueType struct{ Name string } `json:"issuetype"`
		Priority  struct{ Name string } `json:"priority,omitempty"`
		Labels    []string             `json:"labels,omitempty"`
		Description *ADF              `json:"description,omitempty"`
	} `json:"fields"`
}

// ADF is Atlassian Document Format for rich text.
type ADF struct {
	Type    string        `json:"type"`
	Version int           `json:"version"`
	Content []ADFContent  `json:"content"`
}

// ADFContent is a paragraph in ADF.
type ADFContent struct {
	Type    string      `json:"type"`
	Content []ADFText   `json:"content,omitempty"`
}

// ADFText is a text node in ADF.
type ADFText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CreateIssueResponse is the Jira API response for issue creation.
type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// IssueStatus is the Jira issue status.
type IssueStatus struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Status struct {
		Name string `json:"name"`
	} `json:"fields"`
}

// CreateIssue creates a Jira issue and returns the key and URL.
func (c *Client) CreateIssue(ctx context.Context, projectKey, issueType, summary, description, priority string, labels []string) (key, url string, err error) {
	req := CreateIssueRequest{}
	req.Fields.Project.Key = projectKey
	req.Fields.Summary = summary
	req.Fields.IssueType.Name = issueType
	if priority != "" {
		req.Fields.Priority.Name = priority
	}
	req.Fields.Labels = labels
	if description != "" {
		req.Fields.Description = &ADF{
			Type:    "doc",
			Version: 1,
			Content: []ADFContent{{
				Type: "paragraph",
				Content: []ADFText{{Type: "text", Text: description}},
			}},
		}
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/rest/api/3/issue", req)
	if err != nil {
		return "", "", fmt.Errorf("create jira issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("jira create issue failed %d: %s", resp.StatusCode, string(body))
	}

	var result CreateIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode jira response: %w", err)
	}

	issueURL := fmt.Sprintf("%s/browse/%s", c.serverURL, result.Key)
	return result.Key, issueURL, nil
}

// GetIssueStatus returns the current status of a Jira issue.
func (c *Client) GetIssueStatus(ctx context.Context, issueKey string) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=status", issueKey), nil)
	if err != nil {
		return "", fmt.Errorf("get jira issue status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "Not Found", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("jira get issue failed: %d", resp.StatusCode)
	}

	var result struct {
		Fields struct {
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode jira issue: %w", err)
	}
	return result.Fields.Status.Name, nil
}

// doRequest makes an authenticated HTTP request to the Jira API.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.serverURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Basic auth: email:api_token encoded in base64
	credentials := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}
