// Package credential provides a centralized credential manager for source-sync.
// TASK-03-02: Source Credential Manager backed by GCP Secret Manager.
package credential

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CredentialType identifies the kind of credential.
type CredentialType string

const (
	CredentialTypeSSHKey    CredentialType = "ssh_key"
	CredentialTypeToken     CredentialType = "token"
	CredentialTypeBasicAuth CredentialType = "basic_auth"
)

// Credential holds a raw credential value and its metadata.
type Credential struct {
	Type      CredentialType
	Value     string
	ExpiresAt time.Time // zero value = no expiry
}

// IsExpired returns true if the credential has passed its expiry time.
func (c Credential) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// SSHCredential holds an SSH private key.
type SSHCredential struct {
	PrivateKey  string // PEM-encoded
	Fingerprint string
	ExpiresAt   time.Time
}

// BasicAuth holds username+password credentials.
type BasicAuth struct {
	Username string
	Password string
}

// Manager is the interface for credential operations.
type Manager interface {
	// GetSSHKey retrieves the SSH private key for a named source.
	GetSSHKey(ctx context.Context, sourceName string) (*SSHCredential, error)

	// GetToken retrieves an API token for a named source.
	GetToken(ctx context.Context, sourceName string) (string, error)

	// GetBasicAuth retrieves username/password credentials.
	GetBasicAuth(ctx context.Context, sourceName string) (*BasicAuth, error)

	// GetExpiry returns when the credential for a source expires.
	GetExpiry(ctx context.Context, sourceName string) (time.Time, error)

	// RotateCredential triggers rotation of the credential for a source.
	RotateCredential(ctx context.Context, sourceName string) error

	// ListSources returns all source names that have credentials registered.
	ListSources(ctx context.Context) ([]string, error)
}

// SecretNameFunc converts a source name + credential type to a secret name.
// Allows customization per environment.
type SecretNameFunc func(sourceName string, credType CredentialType) string

// DefaultSecretName is the default naming convention for GCP Secret Manager.
func DefaultSecretName(sourceName string, credType CredentialType) string {
	// Format: osv-src-{sourceName}-{credType}
	// e.g. "osv-src-github-advisory-token"
	normalized := strings.ReplaceAll(strings.ToLower(sourceName), "/", "-")
	return fmt.Sprintf("osv-src-%s-%s", normalized, credType)
}

// ---- In-memory (local dev) implementation ----

// InMemoryManager is a simple in-memory credential store for local development and testing.
// DO NOT use in production.
type InMemoryManager struct {
	mu    sync.RWMutex
	store map[string]map[CredentialType]Credential // sourceName → credType → Credential
}

// NewInMemoryManager creates an empty in-memory credential manager.
func NewInMemoryManager() *InMemoryManager {
	return &InMemoryManager{
		store: make(map[string]map[CredentialType]Credential),
	}
}

// Register adds a credential for a source.
func (m *InMemoryManager) Register(sourceName string, credType CredentialType, value string, expiresAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[sourceName]; !ok {
		m.store[sourceName] = make(map[CredentialType]Credential)
	}
	m.store[sourceName][credType] = Credential{
		Type:      credType,
		Value:     value,
		ExpiresAt: expiresAt,
	}
}

// GetSSHKey retrieves the SSH key for a source.
func (m *InMemoryManager) GetSSHKey(_ context.Context, sourceName string) (*SSHCredential, error) {
	cred, err := m.get(sourceName, CredentialTypeSSHKey)
	if err != nil {
		return nil, err
	}
	return &SSHCredential{PrivateKey: cred.Value, ExpiresAt: cred.ExpiresAt}, nil
}

// GetToken retrieves an API token for a source.
func (m *InMemoryManager) GetToken(_ context.Context, sourceName string) (string, error) {
	cred, err := m.get(sourceName, CredentialTypeToken)
	if err != nil {
		return "", err
	}
	return cred.Value, nil
}

// GetBasicAuth retrieves username:password credentials for a source.
func (m *InMemoryManager) GetBasicAuth(_ context.Context, sourceName string) (*BasicAuth, error) {
	cred, err := m.get(sourceName, CredentialTypeBasicAuth)
	if err != nil {
		return nil, err
	}
	// Value format: "username:password"
	parts := strings.SplitN(cred.Value, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("credential %q: invalid basic auth format (expected user:pass)", sourceName)
	}
	return &BasicAuth{Username: parts[0], Password: parts[1]}, nil
}

// GetExpiry returns the expiry time for a source's credential.
func (m *InMemoryManager) GetExpiry(_ context.Context, sourceName string) (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds, ok := m.store[sourceName]
	if !ok {
		return time.Time{}, fmt.Errorf("no credentials for source %q", sourceName)
	}
	// Return the earliest expiry across all credential types.
	earliest := time.Time{}
	for _, cred := range creds {
		if !cred.ExpiresAt.IsZero() {
			if earliest.IsZero() || cred.ExpiresAt.Before(earliest) {
				earliest = cred.ExpiresAt
			}
		}
	}
	return earliest, nil
}

// RotateCredential is a no-op for the in-memory manager.
func (m *InMemoryManager) RotateCredential(_ context.Context, sourceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[sourceName]; !ok {
		return fmt.Errorf("no credentials for source %q", sourceName)
	}
	// In-memory rotation just clears expiry (no external call possible).
	for credType, cred := range m.store[sourceName] {
		cred.ExpiresAt = time.Time{}
		m.store[sourceName][credType] = cred
	}
	return nil
}

// ListSources returns all registered source names.
func (m *InMemoryManager) ListSources(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sources := make([]string, 0, len(m.store))
	for name := range m.store {
		sources = append(sources, name)
	}
	return sources, nil
}

func (m *InMemoryManager) get(sourceName string, credType CredentialType) (Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	creds, ok := m.store[sourceName]
	if !ok {
		return Credential{}, fmt.Errorf("no credentials for source %q", sourceName)
	}
	cred, ok := creds[credType]
	if !ok {
		return Credential{}, fmt.Errorf("no %s credential for source %q", credType, sourceName)
	}
	if cred.IsExpired() {
		return Credential{}, fmt.Errorf("credential for source %q has expired at %s", sourceName, cred.ExpiresAt.Format(time.RFC3339))
	}
	return cred, nil
}

// ---- GCP Secret Manager implementation stub ----

// GCPSecretManagerConfig holds configuration for the GCP Secret Manager backend.
type GCPSecretManagerConfig struct {
	ProjectID      string
	SecretNameFunc SecretNameFunc // defaults to DefaultSecretName
}

// GCPSecretManager retrieves credentials from GCP Secret Manager.
// In production, replace the stub methods with actual Secret Manager API calls.
type GCPSecretManager struct {
	cfg   GCPSecretManagerConfig
	cache sync.Map // sourceName+credType → Credential (TTL cache)
}

// NewGCPSecretManager creates a GCP Secret Manager-backed credential manager.
func NewGCPSecretManager(cfg GCPSecretManagerConfig) *GCPSecretManager {
	if cfg.SecretNameFunc == nil {
		cfg.SecretNameFunc = DefaultSecretName
	}
	return &GCPSecretManager{cfg: cfg}
}

// secretID builds the full GCP secret resource name.
func (g *GCPSecretManager) secretID(sourceName string, credType CredentialType) string {
	name := g.cfg.SecretNameFunc(sourceName, credType)
	return fmt.Sprintf("projects/%s/secrets/%s/versions/latest", g.cfg.ProjectID, name)
}

// GetToken retrieves an API token from GCP Secret Manager.
// TODO(production): replace stub with actual secretmanager.Client call.
func (g *GCPSecretManager) GetToken(_ context.Context, sourceName string) (string, error) {
	secretID := g.secretID(sourceName, CredentialTypeToken)
	// Production:
	//   client, _ := secretmanager.NewClient(ctx)
	//   result, _ := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: secretID})
	//   return string(result.Payload.Data), nil
	return "", fmt.Errorf("GCPSecretManager.GetToken: not configured (secret=%s, set up Secret Manager client)", secretID)
}

// GetSSHKey retrieves an SSH key from GCP Secret Manager.
func (g *GCPSecretManager) GetSSHKey(_ context.Context, sourceName string) (*SSHCredential, error) {
	secretID := g.secretID(sourceName, CredentialTypeSSHKey)
	return nil, fmt.Errorf("GCPSecretManager.GetSSHKey: not configured (secret=%s)", secretID)
}

// GetBasicAuth retrieves basic auth credentials from GCP Secret Manager.
func (g *GCPSecretManager) GetBasicAuth(_ context.Context, sourceName string) (*BasicAuth, error) {
	secretID := g.secretID(sourceName, CredentialTypeBasicAuth)
	return nil, fmt.Errorf("GCPSecretManager.GetBasicAuth: not configured (secret=%s)", secretID)
}

// GetExpiry retrieves the credential expiry (from Secret Manager metadata).
func (g *GCPSecretManager) GetExpiry(_ context.Context, _ string) (time.Time, error) {
	// GCP Secret Manager doesn't have built-in expiry; check via label or separate secret.
	return time.Time{}, nil
}

// RotateCredential triggers rotation via GCP Secret Manager replication or a rotation function.
func (g *GCPSecretManager) RotateCredential(_ context.Context, sourceName string) error {
	return fmt.Errorf("GCPSecretManager.RotateCredential for %q: configure Secret Manager rotation function", sourceName)
}

// ListSources lists all secrets with the osv-src prefix in the project.
func (g *GCPSecretManager) ListSources(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("GCPSecretManager.ListSources: not configured (project=%s)", g.cfg.ProjectID)
}
