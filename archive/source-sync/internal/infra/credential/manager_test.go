package credential_test

import (
	"context"
	"testing"
	"time"

	"github.com/osv/source-sync/internal/infra/credential"
)

func TestInMemoryManager(t *testing.T) {
	ctx := context.Background()
	mgr := credential.NewInMemoryManager()

	// Register credentials
	mgr.Register("github-advisory", credential.CredentialTypeToken, "ghp_testtoken123", time.Time{})
	mgr.Register("gitlab-security", credential.CredentialTypeToken, "glpat_testtoken456", time.Time{})
	mgr.Register("private-git", credential.CredentialTypeSSHKey, "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----", time.Time{})
	mgr.Register("basic-source", credential.CredentialTypeBasicAuth, "user:pass123", time.Time{})

	t.Run("GetToken success", func(t *testing.T) {
		token, err := mgr.GetToken(ctx, "github-advisory")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "ghp_testtoken123" {
			t.Errorf("got %q, want %q", token, "ghp_testtoken123")
		}
	})

	t.Run("GetToken unknown source", func(t *testing.T) {
		_, err := mgr.GetToken(ctx, "unknown-source")
		if err == nil {
			t.Error("expected error for unknown source")
		}
	})

	t.Run("GetSSHKey success", func(t *testing.T) {
		cred, err := mgr.GetSSHKey(ctx, "private-git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cred.PrivateKey == "" {
			t.Error("expected non-empty private key")
		}
	})

	t.Run("GetBasicAuth success", func(t *testing.T) {
		auth, err := mgr.GetBasicAuth(ctx, "basic-source")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if auth.Username != "user" {
			t.Errorf("Username: got %q, want %q", auth.Username, "user")
		}
		if auth.Password != "pass123" {
			t.Errorf("Password: got %q, want %q", auth.Password, "pass123")
		}
	})

	t.Run("GetExpiry no expiry set", func(t *testing.T) {
		exp, err := mgr.GetExpiry(ctx, "github-advisory")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exp.IsZero() {
			t.Errorf("expected zero expiry, got %v", exp)
		}
	})

	t.Run("Expired credential", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		mgr.Register("expired-source", credential.CredentialTypeToken, "oldtoken", past)
		_, err := mgr.GetToken(ctx, "expired-source")
		if err == nil {
			t.Error("expected error for expired credential")
		}
	})

	t.Run("ListSources", func(t *testing.T) {
		sources, err := mgr.ListSources(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sources) < 4 {
			t.Errorf("expected >= 4 sources, got %d", len(sources))
		}
	})

	t.Run("RotateCredential", func(t *testing.T) {
		err := mgr.RotateCredential(ctx, "github-advisory")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("RotateCredential unknown source", func(t *testing.T) {
		err := mgr.RotateCredential(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for unknown source")
		}
	})
}

func TestDefaultSecretName(t *testing.T) {
	tests := []struct {
		sourceName string
		credType   credential.CredentialType
		want       string
	}{
		{"github-advisory", credential.CredentialTypeToken, "osv-src-github-advisory-token"},
		{"github-advisory", credential.CredentialTypeSSHKey, "osv-src-github-advisory-ssh_key"},
		{"org/repo", credential.CredentialTypeToken, "osv-src-org-repo-token"},
	}
	for _, tt := range tests {
		got := credential.DefaultSecretName(tt.sourceName, tt.credType)
		if got != tt.want {
			t.Errorf("DefaultSecretName(%q, %q): got %q, want %q", tt.sourceName, tt.credType, got, tt.want)
		}
	}
}

func TestCredentialExpiry(t *testing.T) {
	c := credential.Credential{
		Type:      credential.CredentialTypeToken,
		Value:     "test",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !c.IsExpired() {
		t.Error("expected credential to be expired")
	}

	c2 := credential.Credential{
		Type:      credential.CredentialTypeToken,
		Value:     "test",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if c2.IsExpired() {
		t.Error("expected credential to NOT be expired")
	}

	c3 := credential.Credential{Type: credential.CredentialTypeToken, Value: "test"}
	if c3.IsExpired() {
		t.Error("expected credential with no expiry to NOT be expired")
	}
}
