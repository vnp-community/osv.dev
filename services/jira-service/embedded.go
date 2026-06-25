package jira

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	deliveryhttp "github.com/osv/jira-service/internal/delivery/http"
	"github.com/osv/jira-service/internal/infra/postgres"
)

// ─── AES-256-GCM CryptoService ───────────────────────────────────────────────

// aesCrypto implements CryptoService using AES-256-GCM authenticated encryption.
// Key must be exactly 32 bytes (256 bits), sourced from JIRA_TOKEN_ENCRYPTION_KEY env
// encoded as standard base64.
type aesCrypto struct {
	key []byte
}

// newAesCrypto decodes the base64-encoded key from env.
func newAesCrypto(keyB64 string) (*aesCrypto, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode JIRA_TOKEN_ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("JIRA_TOKEN_ENCRYPTION_KEY must be 32 bytes after decode (got %d)", len(key))
	}
	return &aesCrypto{key: key}, nil
}

// Encrypt encodes plaintext with AES-256-GCM, returning base64(nonce+ciphertext+tag).
func (c *aesCrypto) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt. Returns an error if the ciphertext is tampered.
func (c *aesCrypto) Decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("aes gcm open (authentication failed): %w", err)
	}
	return plaintext, nil
}

// ─── Plaintext fallback (dev only) ───────────────────────────────────────────

// plaintextCrypto is a no-op CryptoService for development environments without a key.
// SECURITY WARNING: Do NOT use in production. Tokens stored as-is in DB.
type plaintextCrypto struct{}

func (c *plaintextCrypto) Encrypt(plaintext []byte) (string, error) { return string(plaintext), nil }
func (c *plaintextCrypto) Decrypt(ciphertext string) ([]byte, error) {
	return []byte(ciphertext), nil
}

// ─── Real Jira API Client (stdlib net/http) ───────────────────────────────────

// jiraAPIClient implements JiraAPIClient using standard net/http.
// No external dependencies — only uses Jira Cloud REST API v3.
type jiraAPIClient struct {
	client *http.Client
}

func newJiraAPIClient() *jiraAPIClient {
	return &jiraAPIClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// TestConnection verifies JIRA credentials by calling GET /rest/api/3/myself.
// Returns nil on HTTP 200, error otherwise.
func (a *jiraAPIClient) TestConnection(ctx context.Context, jiraURL, username, apiToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jiraURL+"/rest/api/3/myself", nil)
	if err != nil {
		return fmt.Errorf("build jira request: %w", err)
	}
	req.SetBasicAuth(username, apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("jira connection test: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("jira authentication failed: invalid credentials")
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("jira connection test: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// GetVersionAndProject verifies project existence via GET /rest/api/3/project/{key}.
// Returns version="cloud" and projectFound=true/false.
func (a *jiraAPIClient) GetVersionAndProject(ctx context.Context, jiraURL, username, apiToken, projectKey string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jiraURL+"/rest/api/3/project/"+projectKey, nil)
	if err != nil {
		return "", false, fmt.Errorf("build jira project request: %w", err)
	}
	req.SetBasicAuth(username, apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("jira project check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "cloud", false, nil
	}
	if resp.StatusCode >= 300 {
		return "cloud", false, fmt.Errorf("jira project check: status %d", resp.StatusCode)
	}

	// Parse project key from response to confirm
	var proj struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		return "cloud", false, nil
	}
	return "cloud", proj.Key != "", nil
}

// ─── WireEmbedded ─────────────────────────────────────────────────────────────

// WireEmbedded initialises the jira-service routes on the provided ServeMux.
//
// Encryption: Uses AES-256-GCM when JIRA_TOKEN_ENCRYPTION_KEY is set (base64-encoded
// 32-byte key). Falls back to plaintext storage with a warning when unset (dev mode).
// Generate a production key with: openssl rand -base64 32
//
// Platform URL: reads PLATFORM_URL env (default: https://c12.openledger.vn).
func WireEmbedded(ctx context.Context, logger zerolog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) error {
	configRepo := postgres.NewConfigRepo(pool)
	mappingRepo := postgres.NewIssueMappingRepo(pool)

	// ── Crypto: AES-256-GCM when env key present, plaintext fallback for dev ──
	var crypto deliveryhttp.CryptoService
	keyB64 := os.Getenv("JIRA_TOKEN_ENCRYPTION_KEY")
	if keyB64 != "" {
		c, err := newAesCrypto(keyB64)
		if err != nil {
			logger.Warn().Err(err).Msg("jira-service: invalid JIRA_TOKEN_ENCRYPTION_KEY — falling back to plaintext (dev mode only)")
			crypto = &plaintextCrypto{}
		} else {
			crypto = c
			logger.Info().Msg("jira-service: AES-256-GCM token encryption enabled")
		}
	} else {
		logger.Warn().Msg("jira-service: JIRA_TOKEN_ENCRYPTION_KEY not set — API tokens stored as plaintext (generate with: openssl rand -base64 32)")
		crypto = &plaintextCrypto{}
	}

	// ── Jira API: real HTTP client, no external dependencies ──
	jiraAPI := newJiraAPIClient()

	// ── Platform URL for webhook construction ──
	platformURL := os.Getenv("PLATFORM_URL")
	if platformURL == "" {
		platformURL = "https://c12.openledger.vn"
	}

	configHandler := deliveryhttp.NewConfigHandler(configRepo, jiraAPI, crypto, platformURL)
	configHandler.SetIssueRepo(mappingRepo)

	router := &deliveryhttp.Router{
		Config: configHandler,
	}

	mux.Handle("/api/v2/jira-configurations", deliveryhttp.NewRouter(router))
	mux.Handle("/api/v2/jira-configurations/", deliveryhttp.NewRouter(router))
	mux.Handle("/api/v2/jira-issues", deliveryhttp.NewRouter(router))
	mux.Handle("/api/v2/jira-issues/", deliveryhttp.NewRouter(router))

	// Legacy route
	mux.Handle("/jira/", deliveryhttp.NewRouter(router))

	return nil
}
