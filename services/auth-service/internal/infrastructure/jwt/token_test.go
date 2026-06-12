// Package jwt_test provides unit tests for JWT token generation and validation.
package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jwtpkg "github.com/osv/auth-service/internal/infrastructure/jwt"
	"github.com/osv/auth-service/internal/domain/entity"
)

// newTestService creates a JWT Service backed by a randomly generated 2048-bit key.
// The key is written to a temp file because NewService reads from disk.
func newTestService(t *testing.T, ttl time.Duration) *jwtpkg.Service {
	t.Helper()
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	// Generate key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Write PEM to temp file
	tmpFile := t.TempDir() + "/key.pem"
	f, err := os.Create(tmpFile)
	require.NoError(t, err)
	defer f.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	err = pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	require.NoError(t, err)
	f.Close()

	svc, err := jwtpkg.NewService(jwtpkg.Config{
		PrivateKeyPath:  tmpFile,
		Issuer:          "test-issuer",
		Audience:        []string{"test-audience"},
		AccessTokenTTL:  ttl,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	})
	require.NoError(t, err)
	return svc
}

// newTestUser creates an entity.User with a deterministic UUID.
func newTestUser(role string) *entity.User {
	return &entity.User{
		ID:   uuid.New(),
		Role: role,
	}
}

// TestGenerateAndValidateAccessToken verifies issue+verify round-trip.
func TestGenerateAndValidateAccessToken(t *testing.T) {
	svc := newTestService(t, 15*time.Minute)
	user := newTestUser("analyst")

	tokenStr, jti, err := svc.GenerateAccessToken(user)
	require.NoError(t, err, "generate access token")
	assert.NotEmpty(t, tokenStr)
	assert.NotEmpty(t, jti, "JTI should not be empty")

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err, "validate token")
	assert.Equal(t, user.ID.String(), claims.UserID)
	assert.Equal(t, "analyst", claims.Role)
}

// TestExpiredToken verifies that expired tokens are rejected.
func TestExpiredToken(t *testing.T) {
	svc := newTestService(t, 1*time.Millisecond)
	user := newTestUser("readonly")

	tokenStr, _, err := svc.GenerateAccessToken(user)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateToken(tokenStr)
	assert.Error(t, err, "expired token should be rejected")
}

// TestInvalidSignature verifies that tokens from a different key are rejected.
func TestInvalidSignature(t *testing.T) {
	svc1 := newTestService(t, 15*time.Minute)
	svc2 := newTestService(t, 15*time.Minute) // different generated key

	user := newTestUser("admin")
	tokenStr, _, err := svc1.GenerateAccessToken(user)
	require.NoError(t, err)

	_, err = svc2.ValidateToken(tokenStr)
	assert.Error(t, err, "token from different key should be rejected")
}

// TestTokenClaims verifies fields in claims.
func TestTokenClaims(t *testing.T) {
	svc := newTestService(t, 15*time.Minute)
	user := newTestUser("admin")

	tokenStr, _, err := svc.GenerateAccessToken(user)
	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, user.ID.String(), claims.UserID)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.True(t, claims.ExpiryTime().After(time.Now()))
	assert.NotEmpty(t, claims.Permissions)
}

// TestGenerateRefreshToken verifies refresh token generation.
func TestGenerateRefreshToken(t *testing.T) {
	svc := newTestService(t, 15*time.Minute)

	t1, err := svc.GenerateRefreshToken()
	require.NoError(t, err)
	t2, err := svc.GenerateRefreshToken()
	require.NoError(t, err)

	assert.NotEmpty(t, t1)
	assert.NotEmpty(t, t2)
	assert.NotEqual(t, t1, t2)
	assert.GreaterOrEqual(t, len(t1), 64)
}

// TestMalformedToken verifies garbage tokens are rejected.
func TestMalformedToken(t *testing.T) {
	svc := newTestService(t, 15*time.Minute)

	_, err := svc.ValidateToken("not.a.valid.jwt")
	assert.Error(t, err)

	_, err = svc.ValidateToken("")
	assert.Error(t, err)
}

// TestPublicKeyJWKS verifies the JWKS endpoint returns a valid key set.
func TestPublicKeyJWKS(t *testing.T) {
	svc := newTestService(t, 15*time.Minute)
	jwksBytes, err := svc.PublicKeyJWKS()
	require.NoError(t, err)
	assert.NotEmpty(t, jwksBytes)
	// Should be valid JSON containing "keys"
	assert.Contains(t, string(jwksBytes), `"keys"`)
	assert.Contains(t, string(jwksBytes), `"RS256"`)
}
