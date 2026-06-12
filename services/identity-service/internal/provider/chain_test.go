package provider_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osv/identity-service/internal/provider"
)

// ── Mock Provider ────────────────────────────────────────────────────────────

type mockProvider struct {
	name   string
	result provider.AuthResult
	err    error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Authenticate(_ context.Context, _, _ string) (provider.AuthResult, error) {
	return m.result, m.err
}

func newMock(name string, result provider.AuthResult) *mockProvider {
	return &mockProvider{name: name, result: result}
}

// ── Chain Tests ───────────────────────────────────────────────────────────────

func TestChain_Required_Success(t *testing.T) {
	chain, err := provider.New("local:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthOK),
	})
	require.NoError(t, err)
	assert.True(t, chain.Authenticate(context.Background(), "user", "pass"))
}

func TestChain_Required_Fail(t *testing.T) {
	chain, err := provider.New("local:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthWrongCreds),
	})
	require.NoError(t, err)
	assert.False(t, chain.Authenticate(context.Background(), "user", "wrong"))
}

func TestChain_Sufficient_FirstSucceeds(t *testing.T) {
	// If sufficient provider succeeds, chain returns true without trying further
	chain, err := provider.New("local:sufficient,ldap:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthOK),
		"ldap":  newMock("ldap", provider.AuthWrongCreds), // should not be evaluated
	})
	require.NoError(t, err)
	assert.True(t, chain.Authenticate(context.Background(), "user", "pass"))
}

func TestChain_Sufficient_FallsThrough(t *testing.T) {
	// sufficient fails → falls through to required → required succeeds
	chain, err := provider.New("local:sufficient,ldap:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthWrongCreds),
		"ldap":  newMock("ldap", provider.AuthOK),
	})
	require.NoError(t, err)
	assert.True(t, chain.Authenticate(context.Background(), "user", "pass"))
}

func TestChain_Sufficient_BothFail(t *testing.T) {
	chain, err := provider.New("local:sufficient,ldap:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthWrongCreds),
		"ldap":  newMock("ldap", provider.AuthWrongCreds),
	})
	require.NoError(t, err)
	assert.False(t, chain.Authenticate(context.Background(), "user", "pass"))
}

func TestChain_Unavailable_Skipped(t *testing.T) {
	// Unavailable provider is skipped (doesn't count as failure)
	chain, err := provider.New("ldap:required,local:required", map[string]provider.Provider{
		"ldap":  newMock("ldap", provider.AuthUnavailable),
		"local": newMock("local", provider.AuthOK),
	})
	require.NoError(t, err)
	assert.True(t, chain.Authenticate(context.Background(), "user", "pass"))
}

func TestChain_InvalidSpec_MissingColon(t *testing.T) {
	_, err := provider.New("local", map[string]provider.Provider{
		"local": newMock("local", provider.AuthOK),
	})
	assert.Error(t, err)
}

func TestChain_InvalidSpec_UnknownProvider(t *testing.T) {
	_, err := provider.New("oauth:required", map[string]provider.Provider{
		"local": newMock("local", provider.AuthOK),
	})
	assert.Error(t, err)
}

func TestChain_InvalidSpec_UnknownFlag(t *testing.T) {
	_, err := provider.New("local:optional", map[string]provider.Provider{
		"local": newMock("local", provider.AuthOK),
	})
	assert.Error(t, err)
}

func TestChain_EmptySpec(t *testing.T) {
	_, err := provider.New("", map[string]provider.Provider{})
	assert.Error(t, err)
}

// ── HashPassword Tests ────────────────────────────────────────────────────────

func TestHashPassword_ProducesHash(t *testing.T) {
	hash, err := provider.HashPassword("mysecret")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mysecret", hash)
	assert.Contains(t, hash, "$2a$") // bcrypt prefix
}

func TestHashPassword_EmptyReturnsError(t *testing.T) {
	_, err := provider.HashPassword("")
	assert.Error(t, err)
}
