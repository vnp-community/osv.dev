package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHubUserInfo holds the profile data returned by the GitHub user API.
type GitHubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`    // may be empty if private
	AvatarURL string `json:"avatar_url"`
}

// GitHubProvider handles GitHub OAuth2 flow.
type GitHubProvider struct {
	config     *oauth2.Config
	httpClient *http.Client
}

// NewGitHubProvider creates a GitHubProvider with the given OAuth2 credentials.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     github.Endpoint,
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthCodeURL returns the GitHub OAuth2 authorization URL.
func (p *GitHubProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

// Exchange exchanges a code for GitHub user profile.
// Makes two API calls: /user for profile + /user/emails for private email.
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*GitHubUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github token exchange: %w", err)
	}

	info, err := p.getUser(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	// GitHub email may be private — fetch from /user/emails if missing
	if info.Email == "" {
		email, err := p.getPrimaryEmail(ctx, token.AccessToken)
		if err == nil {
			info.Email = email
		}
	}

	if info.Email == "" {
		return nil, fmt.Errorf("github account has no accessible email")
	}
	return info, nil
}

func (p *GitHubProvider) getUser(ctx context.Context, accessToken string) (*GitHubUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "OpenVulnScan/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var info GitHubUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("github user parse: %w", err)
	}
	return &info, nil
}

// getPrimaryEmail fetches the primary verified email from /user/emails.
func (p *GitHubProvider) getPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "OpenVulnScan/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary verified email found")
}
